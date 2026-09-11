import unittest

from scripts import maintenance_lease


class FakeAdmin:
    def __init__(self, hello):
        self.hello = hello
        self.calls = []

    def command(self, command, **kwargs):
        self.calls.append((command, kwargs))
        return self.hello


class FakeCollection:
    def __init__(self, responses=None, find_error=None):
        self.responses = list(responses or [])
        self.find_error = find_error
        self.find_calls = []
        self.update_calls = []

    def find_one_and_update(self, filter_document, update, **kwargs):
        self.find_calls.append((filter_document, update, kwargs))
        if self.find_error is not None:
            raise self.find_error
        if not self.responses:
            return None
        return self.responses.pop(0)

    def update_one(self, filter_document, update, **kwargs):
        self.update_calls.append((filter_document, update, kwargs))
        return type("UpdateResult", (), {"matched_count": 1})()


class FakeDatabase:
    def __init__(self, collection):
        self.collection = collection

    def __getitem__(self, name):
        if name != maintenance_lease.LEASE_COLLECTION:
            raise KeyError(name)
        return self.collection


class FakeClient:
    def __init__(self, collection, hello=None):
        self.admin = FakeAdmin(hello or {"setName": "rs0"})
        self.database = FakeDatabase(collection)

    def __getitem__(self, name):
        if name != maintenance_lease.LEASE_DATABASE:
            raise KeyError(name)
        return self.database


class ConfigurationTests(unittest.TestCase):
    def test_requires_explicit_mongo_uri(self):
        with self.assertRaises(maintenance_lease.MaintenanceLeaseError):
            maintenance_lease.require_mongo_uri({})
        self.assertEqual(
            maintenance_lease.require_mongo_uri(
                {"MONGOURI": " mongodb://mongo/?replicaSet=rs0 "}
            ),
            "mongodb://mongo/?replicaSet=rs0",
        )

    def test_rejects_standalone_mongo_topology(self):
        client = FakeClient(FakeCollection(), hello={"isWritablePrimary": True})
        with self.assertRaises(maintenance_lease.MaintenanceLeaseError):
            maintenance_lease.validate_transaction_topology(client)

    def test_accepts_replica_set_and_mongos_topologies(self):
        for hello in ({"setName": "rs0"}, {"msg": "isdbgrid"}):
            with self.subTest(hello=hello):
                maintenance_lease.validate_transaction_topology(
                    FakeClient(FakeCollection(), hello=hello)
                )


class LeaseTests(unittest.TestCase):
    def test_acquire_guard_and_release_pin_owner_generation(self):
        lease_document = {
            "_id": maintenance_lease.LEASE_ID,
            "owner": "maintenance-test",
            "generation": 9,
        }
        collection = FakeCollection(
            responses=[lease_document, lease_document, lease_document]
        )
        client = FakeClient(collection)
        keeper = maintenance_lease.MaintenanceLease(
            client,
            "test",
            owner="maintenance-test",
            ttl_seconds=120,
            start_renewer=False,
        )

        with keeper:
            calls = []
            result = keeper.guard_write(lambda: calls.append("write") or "result")
            self.assertEqual(result, "result")
            self.assertEqual(calls, ["write"])

        self.assertEqual(len(collection.find_calls), 3)
        self.assertEqual(
            collection.update_calls[0],
            (
                {"_id": maintenance_lease.LEASE_ID},
                maintenance_lease._seed_update(),
                {"upsert": True},
            ),
        )
        acquire_filter, acquire_update, acquire_options = collection.find_calls[0]
        self.assertEqual(
            acquire_filter,
            maintenance_lease._acquire_filter("maintenance-test"),
        )
        self.assertEqual(
            acquire_update,
            maintenance_lease._acquire_update("maintenance-test", 120),
        )
        self.assertEqual(
            acquire_options["maxTimeMS"],
            maintenance_lease.MONGO_OPERATION_TIMEOUT_MS,
        )
        self.assertNotIn("upsert", acquire_options)
        for renew_filter, renew_update, renew_options in collection.find_calls[1:]:
            self.assertEqual(
                renew_filter,
                maintenance_lease._renew_filter("maintenance-test", 9),
            )
            self.assertEqual(
                renew_update,
                maintenance_lease._renew_update(120),
            )
            self.assertEqual(
                renew_options["maxTimeMS"],
                maintenance_lease.MONGO_OPERATION_TIMEOUT_MS,
            )
        self.assertEqual(
            collection.update_calls[1],
            (
                {
                    "_id": maintenance_lease.LEASE_ID,
                    "owner": "maintenance-test",
                    "generation": 9,
                },
                maintenance_lease._release_update(9),
                {},
            ),
        )
        self.assertEqual(
            collection.update_calls[1][0],
            {
                "_id": maintenance_lease.LEASE_ID,
                "owner": "maintenance-test",
                "generation": 9,
            },
        )

    def test_duplicate_key_on_acquire_reports_held_lease(self):
        collection = FakeCollection(
            find_error=maintenance_lease.DuplicateKeyError(
                "duplicate chain-indexer lease"
            )
        )
        keeper = maintenance_lease.MaintenanceLease(
            FakeClient(collection),
            "test",
            owner="maintenance-test",
            start_renewer=False,
        )
        with self.assertRaises(maintenance_lease.MaintenanceLeaseHeldError):
            keeper.acquire()

    def test_missing_conditional_renewal_fails_closed(self):
        lease_document = {
            "_id": maintenance_lease.LEASE_ID,
            "owner": "maintenance-test",
            "generation": 9,
        }
        collection = FakeCollection(responses=[lease_document, None])
        keeper = maintenance_lease.MaintenanceLease(
            FakeClient(collection),
            "test",
            owner="maintenance-test",
            start_renewer=False,
        )
        keeper.acquire()
        with self.assertRaises(maintenance_lease.MaintenanceLeaseLostError):
            keeper.ensure_held()
        with self.assertRaises(maintenance_lease.MaintenanceLeaseLostError):
            keeper.ensure_held()
        keeper.close(raise_renewal_error=False)

    def test_server_time_pipeline_matches_go_lease_shape(self):
        self.assertEqual(
            maintenance_lease._acquire_filter("ignored-owner"),
            {
                "_id": maintenance_lease.LEASE_ID,
                "$or": [
                    {"owner": {"$exists": False}},
                    {
                        "$and": [
                            {"releasedAt": {"$exists": True}},
                            {
                                "$expr": {
                                    "$and": [
                                        {"$lte": ["$expiresAt", "$$NOW"]},
                                        {
                                            "$eq": [
                                                "$releasedGeneration",
                                                "$generation",
                                            ]
                                        },
                                    ]
                                }
                            },
                        ],
                    },
                ],
            },
        )
        self.assertEqual(
            maintenance_lease._seed_update(),
            [
                {
                    "$set": {
                        "generation": {"$ifNull": ["$generation", 0]},
                        "expiresAt": {
                            "$ifNull": [
                                "$expiresAt",
                                {
                                    "$dateSubtract": {
                                        "startDate": "$$NOW",
                                        "unit": "millisecond",
                                        "amount": 1,
                                    }
                                },
                            ]
                        },
                    }
                }
            ],
        )
        self.assertEqual(
            maintenance_lease._acquire_update("owner", 120)[-1],
            {"$unset": ["releasedAt", "releasedGeneration"]},
        )
        self.assertEqual(
            maintenance_lease._release_update(9)[0]["$set"]["releasedGeneration"],
            9,
        )
        self.assertEqual(
            maintenance_lease._renew_update(120),
            [
                {
                    "$set": {
                        "expiresAt": {
                            "$dateAdd": {
                                "startDate": "$$NOW",
                                "unit": "millisecond",
                                "amount": 120_000,
                            }
                        }
                    }
                }
            ],
        )


if __name__ == "__main__":
    unittest.main()
