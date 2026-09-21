"""Exclusive MongoDB lease for one-off chain-index maintenance writers."""

import os
import signal
import socket
import threading
import uuid
from contextlib import contextmanager

try:
    from pymongo import ReturnDocument
    from pymongo.errors import DuplicateKeyError
except ModuleNotFoundError:
    # Pure unit tests use fake collections and do not need the driver. Script
    # entrypoints still import MongoClient before connecting and fail clearly
    # when their declared pymongo dependency is absent.
    class ReturnDocument:
        AFTER = True

    class DuplicateKeyError(Exception):
        pass


LEASE_ID = "chain-indexer"
LEASE_COLLECTION = "syncer_leases"
LEASE_DATABASE = "qrldata-z"
DEFAULT_LEASE_TTL_SECONDS = 120
MONGO_OPERATION_TIMEOUT_MS = 5_000


class MaintenanceLeaseError(RuntimeError):
    """Base class for fail-closed maintenance lease errors."""


class MaintenanceLeaseHeldError(MaintenanceLeaseError):
    """Raised when another chain writer owns the shared lease."""


class MaintenanceLeaseLostError(MaintenanceLeaseError):
    """Raised when this process can no longer prove lease ownership."""


class MaintenanceInterrupted(KeyboardInterrupt):
    """Raised by the graceful SIGINT and SIGTERM handlers."""


def require_mongo_uri(environ=None):
    """Return the explicitly configured MONGOURI or fail before connecting."""
    source = os.environ if environ is None else environ
    mongo_uri = source.get("MONGOURI", "").strip()
    if not mongo_uri:
        raise MaintenanceLeaseError(
            "required environment variable MONGOURI is not set; "
            "maintenance writers require an explicit replica-set MongoDB URI"
        )
    return mongo_uri


def validate_transaction_topology(client):
    """Require the transaction-capable topology used by reorg-safe writes."""
    hello = client.admin.command(
        {"hello": 1},
        maxTimeMS=MONGO_OPERATION_TIMEOUT_MS,
    )
    if not str(hello.get("setName", "")).strip() and hello.get("msg") != "isdbgrid":
        raise MaintenanceLeaseError(
            "MongoDB must be a replica set or mongos before running maintenance writers"
        )


def _lease_expiry_expression(ttl_seconds):
    return {
        "$dateAdd": {
            "startDate": "$$NOW",
            "unit": "millisecond",
            "amount": int(ttl_seconds * 1_000),
        }
    }


def _seed_update():
    return [
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
    ]


def _acquire_filter(_owner):
    return {
        "_id": LEASE_ID,
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
                ]
            },
        ],
    }


def _acquire_update(owner, ttl_seconds):
    return [
        {
            "$set": {
                "owner": owner,
                "generation": {
                    "$add": [{"$ifNull": ["$generation", 0]}, 1]
                },
                "expiresAt": _lease_expiry_expression(ttl_seconds),
            }
        },
        {"$unset": ["releasedAt", "releasedGeneration"]},
    ]


def _renew_filter(owner, generation):
    return {
        "_id": LEASE_ID,
        "owner": owner,
        "generation": generation,
        "$expr": {"$gt": ["$expiresAt", "$$NOW"]},
    }


def _renew_update(ttl_seconds):
    return [
        {
            "$set": {
                "expiresAt": _lease_expiry_expression(ttl_seconds),
            }
        }
    ]


def _release_update(generation):
    return [
        {
            "$set": {
                "expiresAt": "$$NOW",
                "releasedAt": "$$NOW",
                "releasedGeneration": generation,
            }
        }
    ]


class MaintenanceLease:
    """Own and continuously renew the synchronizer's chain-writer lease.

    Every script mutation goes through guard_write(). It proves owner and
    generation against Mongo server time immediately before and after the
    operation. The background renewer keeps the lease alive during RPC and
    index work. Any renewal error permanently fences this process.
    """

    def __init__(
        self,
        client,
        tool_name,
        *,
        ttl_seconds=DEFAULT_LEASE_TTL_SECONDS,
        renewal_interval_seconds=None,
        owner=None,
        start_renewer=True,
    ):
        if not tool_name or ttl_seconds < 1:
            raise ValueError("tool_name and a positive lease TTL are required")
        self._client = client
        self._collection = client[LEASE_DATABASE][LEASE_COLLECTION]
        self._ttl_seconds = ttl_seconds
        self._renewal_interval_seconds = (
            renewal_interval_seconds
            if renewal_interval_seconds is not None
            else ttl_seconds / 3
        )
        if self._renewal_interval_seconds <= 0:
            raise ValueError("renewal interval must be positive")
        self._owner = owner or (
            f"maintenance-{tool_name}-{socket.gethostname()}-{os.getpid()}-{uuid.uuid4().hex}"
        )
        self._generation = None
        self._lease_document = None
        self._failure = None
        self._failure_lock = threading.Lock()
        self._renew_lock = threading.Lock()
        self._stop_event = threading.Event()
        self._renew_thread = None
        self._start_renewer = start_renewer

    @property
    def owner(self):
        return self._owner

    @property
    def generation(self):
        return self._generation

    def __enter__(self):
        self.acquire()
        return self

    def __exit__(self, exc_type, exc_value, traceback):
        self.close(raise_renewal_error=exc_type is None)
        return False

    def acquire(self):
        validate_transaction_topology(self._client)
        self._collection.update_one(
            {"_id": LEASE_ID},
            _seed_update(),
            upsert=True,
        )
        try:
            lease = self._collection.find_one_and_update(
                _acquire_filter(self._owner),
                _acquire_update(self._owner, self._ttl_seconds),
                return_document=ReturnDocument.AFTER,
                maxTimeMS=MONGO_OPERATION_TIMEOUT_MS,
            )
        except DuplicateKeyError as error:
            raise MaintenanceLeaseHeldError(
                "chain-indexer lease is active or lacks a clean release acknowledgement"
            ) from error

        if not lease or lease.get("owner") != self._owner:
            raise MaintenanceLeaseHeldError(
                "chain-indexer lease is active or lacks a clean release acknowledgement"
            )
        generation = lease.get("generation")
        if not isinstance(generation, int) or generation <= 0:
            raise MaintenanceLeaseLostError(
                "MongoDB returned a malformed chain-indexer lease generation"
            )

        self._generation = generation
        self._lease_document = lease
        if self._start_renewer:
            self._renew_thread = threading.Thread(
                target=self._renew_loop,
                name="chain-indexer-lease-renewer",
                daemon=True,
            )
            self._renew_thread.start()
        return lease

    def ensure_held(self):
        """Synchronously renew the lease and fail closed on any uncertainty."""
        self._raise_recorded_failure()
        try:
            lease = self._renew_once()
        except MaintenanceLeaseLostError as error:
            self._record_failure(error)
            raise
        except Exception as error:
            lost = MaintenanceLeaseLostError(
                f"could not renew the chain-indexer lease: {error}"
            )
            self._record_failure(lost)
            raise lost from error
        self._raise_recorded_failure()
        return lease

    def guard_write(self, operation):
        """Run one synchronous Mongo mutation between server-time lease checks."""
        self.ensure_held()
        result = operation()
        self.ensure_held()
        return result

    def close(self, *, raise_renewal_error=True):
        """Stop renewal, wait for it to drain, then release this generation."""
        self._stop_event.set()
        if self._renew_thread is not None:
            self._renew_thread.join(timeout=MONGO_OPERATION_TIMEOUT_MS / 1_000 + 1)
            if self._renew_thread.is_alive():
                join_error = MaintenanceLeaseLostError(
                    "lease renewal did not drain; retaining the lease until TTL expiry"
                )
                self._record_failure(join_error)
                if raise_renewal_error:
                    raise join_error
                return

        release_error = None
        if self._generation is not None:
            try:
                result = self._collection.update_one(
                    {
                        "_id": LEASE_ID,
                        "owner": self._owner,
                        "generation": self._generation,
                    },
                    _release_update(self._generation),
                )
                if result.matched_count != 1:
                    raise MaintenanceLeaseLostError(
                        "chain-indexer lease changed before release"
                    )
            except Exception as error:
                release_error = MaintenanceLeaseError(
                    f"failed to release the chain-indexer lease: {error}"
                )
            finally:
                self._generation = None
                self._lease_document = None

        if release_error is not None and raise_renewal_error:
            raise release_error
        if raise_renewal_error:
            self._raise_recorded_failure()

    def _renew_once(self):
        if self._generation is None:
            raise MaintenanceLeaseLostError("chain-indexer lease is not active")
        with self._renew_lock:
            lease = self._collection.find_one_and_update(
                _renew_filter(self._owner, self._generation),
                _renew_update(self._ttl_seconds),
                return_document=ReturnDocument.AFTER,
                maxTimeMS=MONGO_OPERATION_TIMEOUT_MS,
            )
        if lease is None:
            raise MaintenanceLeaseLostError(
                "chain-indexer lease was lost, replaced, or expired"
            )
        self._lease_document = lease
        return lease

    def _renew_loop(self):
        while not self._stop_event.wait(self._renewal_interval_seconds):
            try:
                self._renew_once()
            except MaintenanceLeaseLostError as error:
                self._record_failure(error)
                return
            except Exception as error:
                self._record_failure(
                    MaintenanceLeaseLostError(
                        f"could not renew the chain-indexer lease: {error}"
                    )
                )
                return

    def _record_failure(self, error):
        with self._failure_lock:
            if self._failure is None:
                self._failure = error

    def _raise_recorded_failure(self):
        with self._failure_lock:
            failure = self._failure
        if failure is not None:
            raise failure


@contextmanager
def maintenance_shutdown_signals():
    """Turn SIGINT and SIGTERM into stack unwinding so lease release can drain."""
    previous = {}

    def handle_signal(signum, _frame):
        raise MaintenanceInterrupted(f"received signal {signum}")

    for signum in (signal.SIGINT, signal.SIGTERM):
        previous[signum] = signal.getsignal(signum)
        signal.signal(signum, handle_signal)
    try:
        yield
    finally:
        for signum, handler in previous.items():
            signal.signal(signum, handler)
