import unittest
from unittest.mock import MagicMock

from network_profile import (
    IDENTITY_COLLECTION,
    NetworkIdentityError,
    guard_source,
    open_database,
    parse_profile,
    validate_sources,
)

GENESIS = "0x" + "1" * 64


class NetworkProfileTests(unittest.TestCase):
    def env(self, **values):
        return {"MONGOURI": "mongodb://127.0.0.1:27017", **values}

    def test_legacy_defaults(self):
        profile = parse_profile(self.env())
        self.assertEqual(profile["databaseName"], "qrldata-z")
        self.assertEqual(profile["identity"], {
            "networkId": "v2", "chainId": "", "genesisHash": "",
            "addressBytes": 20, "identityVerified": False,
        })

    def test_future_network_is_rejected_before_database_access(self):
        with self.assertRaisesRegex(ValueError, "20-byte"):
            parse_profile(self.env(EXPLORER_NETWORK="v3", MONGO_DB_NAME="qrldata-v3", EXPECTED_CHAIN_ID="1337", EXPECTED_GENESIS_HASH=GENESIS))

    def test_configuration_rejects_aliasing_and_incomplete_pins(self):
        for values in [
            {"EXPLORER_NETWORK": "v4"},
            {"EXPLORER_NETWORK": "v3"},
            {"EXPLORER_NETWORK": "v3", "MONGO_DB_NAME": "QRLData-Z"},
            {"MONGO_DB_NAME": "admin"},
            {"MONGO_DB_NAME": "local"},
            {"MONGO_DB_NAME": "v3/other"},
            {"MONGOURI": "mongodb://user:secret@localhost/wrong"},
            {"MONGOURI": "mongodb://localhost/qrldata%2dv3"},
            {"EXPECTED_CHAIN_ID": "1337"},
            {"EXPECTED_GENESIS_HASH": GENESIS},
            {"EXPECTED_CHAIN_ID": "0", "EXPECTED_GENESIS_HASH": GENESIS},
            {"EXPECTED_CHAIN_ID": "1337", "EXPECTED_GENESIS_HASH": "0x" + "0" * 64},
        ]:
            with self.subTest(values=values):
                with self.assertRaises(ValueError) as error:
                    parse_profile(self.env(**values))
                self.assertNotIn("secret", str(error.exception))

    def test_database_requires_exact_marker_and_never_writes(self):
        profile = parse_profile(self.env(MONGO_DB_NAME="v2_archive"))
        client = MagicMock()
        database = client.__getitem__.return_value
        marker = database.__getitem__.return_value
        marker.find_one.return_value = profile["identity"]
        self.assertIs(open_database(client, profile), database)
        client.__getitem__.assert_called_with("v2_archive")
        database.__getitem__.assert_called_with(IDENTITY_COLLECTION)
        for invalid in [None, {**profile["identity"], "networkId": "v3"}, {**profile["identity"], "identityVerified": 0}, {**profile["identity"], "addressBytes": 64}]:
            marker.find_one.return_value = invalid
            with self.assertRaises(ValueError):
                open_database(client, profile)
        marker.insert_one.assert_not_called()
        marker.update_one.assert_not_called()
        database.drop_collection.assert_not_called()

    def test_all_pinned_sources_checked_and_genesis_distinguishes_resets(self):
        environ = self.env(EXPECTED_CHAIN_ID="0x539", EXPECTED_GENESIS_HASH=GENESIS, NODE_URL="http://primary", NODE_URLS="http://primary,http://secondary", MEMPOOL_NODE_URL="http://mempool", TRACE_NODE_URL="ws://trace")
        profile = parse_profile(environ)
        calls = []

        def rpc(source, method, params):
            calls.append((source, method))
            return "0x539" if method == "qrl_chainId" else {"hash": GENESIS}

        validate_sources(profile, environ, rpc)
        self.assertEqual(len(calls), 8)
        self.assertEqual({call[0] for call in calls}, {"http://primary", "http://secondary", "http://mempool", "ws://trace"})

        def reset_rpc(source, method, params):
            return "0x539" if method == "qrl_chainId" else {"hash": "0x" + "2" * 64}

        with self.assertRaises(NetworkIdentityError):
            guard_source(profile, "http://primary", reset_rpc)

    def test_malformed_genesis_stops_maintenance(self):
        profile = parse_profile(self.env(EXPECTED_CHAIN_ID="1337", EXPECTED_GENESIS_HASH=GENESIS))
        for malformed in [None, [], {"hash": 1}, {"hash": None}, {}]:
            def rpc(source, method, params):
                return "0x539" if method == "qrl_chainId" else malformed
            with self.subTest(genesis=malformed):
                with self.assertRaises(NetworkIdentityError):
                    guard_source(profile, "http://primary", rpc)

    def test_unpinned_v2_preserves_debug_only_transport(self):
        rpc = MagicMock()
        validate_sources(parse_profile(self.env()), {"TRACE_NODE_URL": "ws://debug-only"}, rpc)
        rpc.assert_not_called()


if __name__ == "__main__":
    unittest.main()
