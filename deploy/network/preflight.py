#!/usr/bin/env python3
"""Offline network isolation and release-artifact checks. Never starts services."""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import sys
from urllib.parse import unquote, urlsplit


class InvalidProfile(ValueError):
    pass


def require(condition, message):
    if not condition:
        raise InvalidProfile(message)


def digest(path):
    value = hashlib.sha256()
    with Path(path).open("rb") as handle:
        for chunk in iter(lambda: handle.read(1024 * 1024), b""):
            value.update(chunk)
    return value.hexdigest()


def normalized_origin(value):
    origin = urlsplit(value)
    require(origin.scheme == "https" and origin.hostname and
            not origin.username and not origin.query and not origin.fragment and
            origin.path in ("", "/"), "Origin must be an HTTPS origin")
    return (origin.scheme.lower(), origin.hostname.lower(), origin.port or 443)


def validate_profiles(document):
    require(isinstance(document, dict), "Profiles must be a JSON object")
    require(document.get("version") == 1, "Unsupported profile version")
    profiles = document.get("networks", [])
    require(isinstance(profiles, list) and all(isinstance(p, dict) for p in profiles),
            "Networks must be a list of profile objects")
    require(len(profiles) == 2, "Exactly one v2 and one v3 profile are required")
    require({p.get("network") for p in profiles} == {"v2", "v3"},
            "Expected v2 and v3 profiles")
    unique = {name: set() for name in (
        "database", "volume", "origin", "mongo_uri_env", "mongo_user_file_env",
        "mongo_password_file_env")}
    all_ports = set()
    for profile in profiles:
        name = profile["network"]
        require(type(profile.get("enabled")) is bool, f"{name}: enabled must be boolean")
        require(profile.get("address_bytes") == (20 if name == "v2" else 64),
                f"{name}: incorrect address width")
        require(profile.get("bind") == "127.0.0.1", f"{name}: bind must be loopback")
        for field, seen in unique.items():
            value = profile.get(field)
            require(isinstance(value, str) and value, f"{name}: missing {field}")
            normalized = (normalized_origin(value) if field == "origin" else
                          value.lower() if field == "database" else value)
            require(normalized not in seen, f"Networks share {field}")
            seen.add(normalized)
        require(re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_-]{0,62}", profile["database"]) and
                profile["database"].lower() not in ("admin", "local", "config"),
                f"{name}: unsafe database name")
        require(re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_.-]{0,127}", profile["volume"]),
                f"{name}: unsafe volume name")
        for key in ("mongo_uri_env", "mongo_user_file_env", "mongo_password_file_env"):
            require(re.fullmatch(r"[A-Z][A-Z0-9_]*", profile[key]), f"{name}: invalid {key}")
        ports = profile.get("ports", {})
        require(set(ports) == {"frontend", "backend", "syncer_health", "mongodb"},
                f"{name}: all four service ports are required")
        for port in ports.values():
            require(type(port) is int and 1024 <= port <= 65535,
                    f"{name}: invalid service port")
            require(port not in all_ports, "Service ports overlap")
            all_ports.add(port)
        if name == "v3":
            require(profile["database"].lower() != "qrldata-z", "v3 cannot use the legacy database")
            require(profile.get("replica_set") == "zondscan-v3", "v3 requires its dedicated replica set")
    return {p["network"]: p for p in profiles}


def private_secret(path):
    source = Path(path)
    require(source.is_file() and not source.is_symlink(), "Credential file is missing or a symlink")
    require(source.stat().st_mode & 0o077 == 0, "Credential files must be owner-only")
    value = source.read_text().strip()
    require(bool(value), "Credential file is empty")
    return value


def validate_credentials(profiles, environment):
    users, passwords, uris = [], [], []
    for profile in profiles.values():
        values = {}
        for kind in ("mongo_uri_env", "mongo_user_file_env", "mongo_password_file_env"):
            value = environment.get(profile[kind], "")
            require(bool(value), f"Missing environment variable {profile[kind]}")
            values[kind] = value
        user = private_secret(values["mongo_user_file_env"])
        password = private_secret(values["mongo_password_file_env"])
        uri = urlsplit(values["mongo_uri_env"])
        require(uri.scheme == "mongodb" and uri.hostname in ("127.0.0.1", "localhost"),
                "MongoDB URI must use one loopback host")
        require(uri.port == profile["ports"]["mongodb"], "MongoDB URI port differs from profile")
        require(unquote(uri.path.lstrip("/")) == profile["database"],
                "MongoDB URI database differs from profile")
        require(unquote(uri.username or "") == user and unquote(uri.password or "") == password,
                "MongoDB URI and credential files differ")
        users.append(user)
        passwords.append(password)
        uris.append(values["mongo_uri_env"])
    require(len(set(users)) == 2 and len(set(passwords)) == 2 and len(set(uris)) == 2,
            "Networks must use separate credentials and connection strings")


def validate_activation(profiles, network, readiness=None, environment=None):
    profile = profiles[network]
    require(profile["enabled"], f"{network} is disabled")
    require(isinstance(profile.get("chain_id"), str) and re.fullmatch(r"[1-9][0-9]{0,77}", profile["chain_id"])
            and int(profile["chain_id"]) < 2 ** 256,
            "Activation requires a verified positive chain ID")
    require(isinstance(profile.get("genesis_hash"), str) and
            re.fullmatch(r"0x[0-9a-fA-F]{64}", profile["genesis_hash"]) and
            int(profile["genesis_hash"], 16) > 0, "Activation requires a verified genesis hash")
    beacon_root = profile.get("beacon_genesis_validators_root")
    require(isinstance(beacon_root, str) and re.fullmatch(r"0x[0-9a-fA-F]{64}", beacon_root)
            and int(beacon_root, 16) > 0,
            "Activation requires a verified beacon genesis validators root")
    beacon_time = profile.get("beacon_genesis_time")
    require(isinstance(beacon_time, str) and re.fullmatch(r"[1-9][0-9]{0,19}", beacon_time)
            and int(beacon_time) < 2 ** 64,
            "Activation requires a verified positive beacon genesis time")
    for field in ("rpc_url", "beacon_url"):
        url = urlsplit(profile.get(field) or "")
        require(url.scheme in ("http", "https") and url.hostname and not url.username
                and not url.fragment, f"Activation requires an explicit {field}")
        if field == "beacon_url":
            require(not url.query, "Beacon URL must be a base URL without a query")
    other = profiles["v2" if network == "v3" else "v3"]
    require(profile["genesis_hash"].lower() != (other.get("genesis_hash") or "").lower(),
            "Networks share genesis_hash")
    require((beacon_root.lower(), beacon_time) != (
                (other.get("beacon_genesis_validators_root") or "").lower(),
                other.get("beacon_genesis_time")),
            "Networks share the beacon genesis identity")
    for field in ("rpc_url", "beacon_url"):
        require(profile[field] != other.get(field), f"Networks share {field}")
    if network == "v3":
        require(isinstance(readiness, dict), "v3 requires a reviewed artifact readiness manifest")
        require(readiness.get("networkId") == network and readiness.get("addressBytes") == 64,
                "Readiness manifest has the wrong network or address capability")
        require(readiness.get("identityVerified") is True,
                "Readiness manifest must attest verified network identity")
        require(readiness.get("chainId") == profile["chain_id"] and
                readiness.get("genesisHash", "").lower() == profile["genesis_hash"].lower(),
                "Readiness manifest does not match the chain pins")
        readiness_beacon_root = readiness.get("beaconGenesisValidatorsRoot")
        require(isinstance(readiness_beacon_root, str) and
                readiness_beacon_root.lower() == beacon_root.lower() and
                readiness.get("beaconGenesisTime") == beacon_time,
                "Readiness manifest does not match the beacon genesis pins")
        require(readiness.get("transactional_ingestion") is True,
                "v3 requires validated transactional ingestion")
        artifacts = readiness.get("artifacts", {})
        require(isinstance(artifacts, dict) and set(artifacts) == {"frontend", "backend", "syncer"},
                "Readiness requires all three release artifacts")
        for name, artifact in artifacts.items():
            require(isinstance(artifact, dict), f"{name}: artifact must be an object")
            require(artifact.get("address_bytes") == 64 and
                    artifact.get("network_isolation") is True,
                    f"{name}: legacy or unverified network capability")
            expected = artifact.get("sha256", "")
            require(re.fullmatch(r"[0-9a-f]{64}", expected) and
                    digest(artifact.get("path", "")) == expected,
                    f"{name}: artifact checksum mismatch")
    if environment is not None:
        validate_credentials(profiles, environment)
    return profile


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("profiles", type=Path)
    parser.add_argument("--activate", choices=("v2", "v3"))
    parser.add_argument("--readiness", type=Path)
    parser.add_argument("--check-credentials", action="store_true")
    args = parser.parse_args()
    try:
        profiles = validate_profiles(json.loads(args.profiles.read_text()))
        if args.activate:
            readiness = json.loads(args.readiness.read_text()) if args.readiness else None
            validate_activation(profiles, args.activate, readiness, os.environ)
        if args.check_credentials:
            validate_credentials(profiles, os.environ)
    except (OSError, ValueError, KeyError, TypeError) as error:
        print(f"Preflight refused: {error}", file=sys.stderr)
        return 1
    print("Offline isolation checks passed" + (f" for {args.activate}" if args.activate else ""))
    return 0


if __name__ == "__main__":
    sys.exit(main())
