"""Network and database guards for the legacy maintenance tools."""

import json
import re
from urllib.parse import unquote, urlsplit

IDENTITY_COLLECTION = "explorerNetwork"
NATIVE_ADDRESS_BYTES = 20


class NetworkIdentityError(ValueError):
    """Stop maintenance immediately when the selected chain cannot be verified."""



def parse_profile(environ):
    network = environ.get("EXPLORER_NETWORK", "v2").strip() or "v2"
    database = environ.get("MONGO_DB_NAME", "").strip()
    if network not in ("v2", "v3"):
        raise ValueError("EXPLORER_NETWORK must be v2 or v3")
    if network == "v3" and (not database or database.lower() == "qrldata-z"):
        raise ValueError("v3 requires an explicit separate MONGO_DB_NAME")
    database = database or "qrldata-z"
    if not re.fullmatch(r"[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}", database):
        raise ValueError("invalid MONGO_DB_NAME")
    if database.lower() in ("admin", "local", "config"):
        raise ValueError("MONGO_DB_NAME must be an explorer database")
    chain_id = environ.get("EXPECTED_CHAIN_ID", "").strip()
    genesis = environ.get("EXPECTED_GENESIS_HASH", "").strip()
    if bool(chain_id) != bool(genesis):
        raise ValueError("EXPECTED_CHAIN_ID and EXPECTED_GENESIS_HASH must be configured together")
    if chain_id:
        chain_id = normalize_chain_id(chain_id)
        if not re.fullmatch(r"0[xX][0-9a-fA-F]{64}", genesis) or int(genesis, 16) == 0:
            raise ValueError("invalid EXPECTED_GENESIS_HASH")
        genesis = genesis.lower()
    if network == "v3":
        raise ValueError("these maintenance tools support 20-byte v2 addresses; use reviewed v3 maintenance tools")
    uri = environ.get("MONGOURI", "").strip()
    try:
        parsed = urlsplit(uri)
    except ValueError:
        raise ValueError("invalid MONGOURI") from None
    if parsed.scheme not in ("mongodb", "mongodb+srv") or not parsed.netloc:
        raise ValueError("MONGOURI is required and must be a MongoDB URI")
    suffix = unquote(parsed.path.removeprefix("/"))
    if suffix and suffix != database:
        raise ValueError("MONGOURI database suffix must match MONGO_DB_NAME")
    return {
        "databaseName": database,
        "identity": {
            "networkId": network,
            "chainId": chain_id,
            "genesisHash": genesis,
            "addressBytes": NATIVE_ADDRESS_BYTES,
            "identityVerified": bool(chain_id),
        },
    }


def normalize_chain_id(value):
    if not isinstance(value, str) or not re.fullmatch(r"(?:0[xX][0-9a-fA-F]+|[0-9]+)", value):
        raise ValueError("invalid chain ID")
    if len(value) > 80:
        raise ValueError("invalid chain ID")
    number = int(value, 16 if value.lower().startswith("0x") else 10)
    if number <= 0 or number.bit_length() > 256:
        raise ValueError("invalid chain ID")
    return str(number)


def rpc_identity_call(source, method, params):
    """Bounded identity probes never include transport errors or secrets."""
    payload = {"jsonrpc": "2.0", "id": 1, "method": method, "params": params}
    try:
        scheme = urlsplit(source).scheme
        if scheme in ("http", "https"):
            import requests

            with requests.post(source, json=payload, timeout=8, allow_redirects=False, stream=True) as response:
                if response.status_code != 200:
                    raise ValueError("RPC identity request rejected")
                body = bytearray()
                for chunk in response.iter_content(chunk_size=8192):
                    body.extend(chunk)
                    if len(body) > 2 * 1024 * 1024:
                        raise ValueError("RPC identity response too large")
        elif scheme in ("ws", "wss"):
            from websocket import create_connection

            connection = create_connection(source, timeout=8)
            try:
                connection.send(json.dumps(payload))
                body = connection.recv()
                if len(body) > 2 * 1024 * 1024:
                    raise ValueError("RPC identity response too large")
            finally:
                connection.close()
        else:
            raise ValueError("unsupported RPC transport")
        result = json.loads(body)
        if result.get("jsonrpc") != "2.0" or result.get("id") != 1 or result.get("error") or result.get("result") is None:
            raise ValueError("RPC identity response invalid")
        return result["result"]
    except Exception:
        raise NetworkIdentityError("RPC identity request failed") from None


def guard_source(profile, source, rpc_call=rpc_identity_call):
    identity = profile["identity"]
    if not identity["chainId"]:
        return
    try:
        chain_id = normalize_chain_id(rpc_call(source, "qrl_chainId", []))
    except ValueError:
        raise NetworkIdentityError("RPC chain ID invalid") from None
    genesis = rpc_call(source, "qrl_getBlockByNumber", ["0x0", False])
    hash_value = genesis.get("hash") if isinstance(genesis, dict) else None
    if chain_id != identity["chainId"] or not isinstance(hash_value, str) or hash_value.lower() != identity["genesisHash"]:
        raise NetworkIdentityError("RPC network identity mismatch")


def validate_sources(profile, environ, rpc_call=rpc_identity_call):
    if not profile["identity"]["chainId"]:
        return
    sources = []
    for name in ("NODE_URL", "NODE_URLS", "MEMPOOL_NODE_URL", "TRACE_NODE_URL"):
        for source in environ.get(name, "").split(","):
            source = source.strip()
            if source and source not in sources:
                sources.append(source)
    if not sources or len(sources) > 16:
        raise ValueError("pinned network requires 1-16 RPC sources")
    for source in sources:
        guard_source(profile, source, rpc_call)


def open_database(client, profile):
    """Maintenance requires a marker installed by the guarded API/syncer.

    These tools never adopt, relabel, or initialize databases themselves.
    """
    database = client[profile["databaseName"]]
    stored = database[IDENTITY_COLLECTION].find_one({"_id": "identity"})
    if not stored or any(type(stored.get(key)) is not type(value) or stored.get(key) != value for key, value in profile["identity"].items()):
        raise ValueError("database network identity missing or mismatched; initialize with the matching guarded API/syncer first")
    return database
