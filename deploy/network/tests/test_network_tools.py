import copy
import json
import os
from pathlib import Path
import sys
import tempfile
import unittest
from unittest.mock import patch

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import mongo_archive
import preflight


EXAMPLE = Path(__file__).resolve().parents[1] / "profiles.example.json"


class ProfilesTest(unittest.TestCase):
    def setUp(self):
        self.document = json.loads(EXAMPLE.read_text())

    def test_template_is_isolated_and_v3_disabled(self):
        profiles = preflight.validate_profiles(self.document)
        self.assertFalse(profiles["v3"]["enabled"])
        with self.assertRaisesRegex(preflight.InvalidProfile, "disabled"):
            preflight.validate_activation(profiles, "v3")

    def test_shared_storage_credentials_and_origin_are_rejected(self):
        for field in ("database", "volume", "mongo_uri_env", "mongo_user_file_env",
                      "mongo_password_file_env", "origin"):
            with self.subTest(field=field):
                document = copy.deepcopy(self.document)
                document["networks"][1][field] = document["networks"][0][field]
                with self.assertRaises(preflight.InvalidProfile):
                    preflight.validate_profiles(document)

    def test_all_service_ports_must_be_unique_and_loopback(self):
        self.document["networks"][1]["ports"]["mongodb"] = 18120
        with self.assertRaisesRegex(preflight.InvalidProfile, "ports overlap"):
            preflight.validate_profiles(self.document)
        self.document = json.loads(EXAMPLE.read_text())
        self.document["networks"][0]["bind"] = "0.0.0.0"
        with self.assertRaisesRegex(preflight.InvalidProfile, "loopback"):
            preflight.validate_profiles(self.document)

    def test_equivalent_origins_are_treated_as_shared(self):
        self.document["networks"][1]["origin"] = "https://ZONDSCAN.COM:443/"
        with self.assertRaisesRegex(preflight.InvalidProfile, "share origin"):
            preflight.validate_profiles(self.document)

    def test_reserved_and_legacy_database_aliases_are_rejected(self):
        for database in ("admin", "CONFIG", "local", "_database", "QRLData-Z"):
            with self.subTest(database=database):
                document = copy.deepcopy(self.document)
                document["networks"][1]["database"] = database
                with self.assertRaises(preflight.InvalidProfile):
                    preflight.validate_profiles(document)

    def active_profiles(self):
        profiles = preflight.validate_profiles(self.document)
        profiles["v3"].update(enabled=True, chain_id="90001", genesis_hash="0x" + "ab" * 32,
                              beacon_genesis_validators_root="0x" + "cd" * 32,
                              beacon_genesis_time="1800000000",
                              rpc_url="http://127.0.0.1:19001", beacon_url="http://127.0.0.1:19002")
        return profiles

    def test_activation_requires_verified_pins_and_readiness(self):
        profiles = preflight.validate_profiles(self.document)
        profiles["v3"]["enabled"] = True
        with self.assertRaisesRegex(preflight.InvalidProfile, "chain ID"):
            preflight.validate_activation(profiles, "v3")
        with self.assertRaisesRegex(preflight.InvalidProfile, "readiness manifest"):
            preflight.validate_activation(self.active_profiles(), "v3")

    def test_out_of_range_chain_and_same_genesis_are_rejected(self):
        profiles = self.active_profiles()
        profiles["v3"]["chain_id"] = str(2 ** 256)
        with self.assertRaisesRegex(preflight.InvalidProfile, "chain ID"):
            preflight.validate_activation(profiles, "v3")
        profiles = self.active_profiles()
        profiles["v2"]["genesis_hash"] = "0x" + "AB" * 32
        with self.assertRaisesRegex(preflight.InvalidProfile, "share genesis"):
            preflight.validate_activation(profiles, "v3")

    def test_activation_requires_a_valid_beacon_pin_pair(self):
        for field, values in (
            ("beacon_genesis_validators_root", (None, "", "0x" + "00" * 32, "0xabc", 123)),
            ("beacon_genesis_time", (None, "", "0", "01", "-1", "1.5", 1, str(2 ** 64))),
        ):
            for value in values:
                with self.subTest(field=field, value=value):
                    profiles = self.active_profiles()
                    profiles["v3"][field] = value
                    with self.assertRaisesRegex(preflight.InvalidProfile, "beacon genesis"):
                        preflight.validate_activation(profiles, "v3")

    def test_same_beacon_genesis_pair_is_rejected(self):
        profiles = self.active_profiles()
        profiles["v2"]["beacon_genesis_validators_root"] = "0x" + "CD" * 32
        profiles["v2"]["beacon_genesis_time"] = "1800000000"
        with self.assertRaisesRegex(preflight.InvalidProfile, "share the beacon genesis identity"):
            preflight.validate_activation(profiles, "v3")

    def test_legacy_or_modified_artifacts_cannot_activate_v3(self):
        profiles = self.active_profiles()
        with tempfile.TemporaryDirectory() as directory:
            artifact = Path(directory) / "artifact"
            artifact.write_bytes(b"test release artifact")
            manifest = {
                "networkId": "v3", "addressBytes": 64, "chainId": "90001", "identityVerified": True,
                "genesisHash": profiles["v3"]["genesis_hash"], "transactional_ingestion": True,
                "beaconGenesisValidatorsRoot": profiles["v3"]["beacon_genesis_validators_root"],
                "beaconGenesisTime": profiles["v3"]["beacon_genesis_time"],
                "artifacts": {name: {"path": str(artifact), "sha256": preflight.digest(artifact),
                    "address_bytes": 64, "network_isolation": True}
                    for name in ("frontend", "backend", "syncer")},
            }
            preflight.validate_activation(profiles, "v3", manifest)
            for field, values in (
                ("beaconGenesisValidatorsRoot", (None, "0x" + "ef" * 32)),
                ("beaconGenesisTime", (None, "1800000001", 1800000000)),
            ):
                original = manifest[field]
                for value in values:
                    with self.subTest(field=field, value=value):
                        manifest[field] = value
                        with self.assertRaisesRegex(preflight.InvalidProfile, "beacon genesis pins"):
                            preflight.validate_activation(profiles, "v3", manifest)
                manifest[field] = original
            manifest["beaconGenesisValidatorsRoot"] = "0x" + "CD" * 32
            preflight.validate_activation(profiles, "v3", manifest)
            manifest["identityVerified"] = False
            with self.assertRaisesRegex(preflight.InvalidProfile, "verified network identity"):
                preflight.validate_activation(profiles, "v3", manifest)
            manifest["identityVerified"] = True
            manifest["artifacts"]["syncer"]["address_bytes"] = 20
            with self.assertRaisesRegex(preflight.InvalidProfile, "legacy"):
                preflight.validate_activation(profiles, "v3", manifest)
            manifest["artifacts"]["syncer"]["address_bytes"] = 64
            artifact.write_bytes(b"different executable")
            with self.assertRaisesRegex(preflight.InvalidProfile, "checksum"):
                preflight.validate_activation(profiles, "v3", manifest)

    def test_shared_or_public_credentials_are_rejected(self):
        profiles = preflight.validate_profiles(self.document)
        with tempfile.TemporaryDirectory() as directory:
            environment = {}
            for network, profile in profiles.items():
                for kind, value in (("user", network + "-user"), ("password", network + "-secret")):
                    path = Path(directory) / (network + kind)
                    path.write_text(value)
                    path.chmod(0o600)
                    environment[profile[f"mongo_{kind}_file_env"]] = str(path)
                environment[profile["mongo_uri_env"]] = (
                    f"mongodb://{network}-user:{network}-secret@127.0.0.1:"
                    f"{profile['ports']['mongodb']}/{profile['database']}")
            preflight.validate_credentials(profiles, environment)
            password_path = Path(environment[profiles["v3"]["mongo_password_file_env"]])
            password_path.chmod(0o644)
            with self.assertRaisesRegex(preflight.InvalidProfile, "owner-only"):
                preflight.validate_credentials(profiles, environment)


class ArchiveTest(unittest.TestCase):
    inventory = [{"name": "faucetClaims", "type": "collection", "count": 2,
                  "indexes": [{"name": "_id_", "key": {"_id": 1}}]}]

    def test_same_server_alias_and_live_database_restore_rejected(self):
        manifest = {"version": 1, "database": "qrldata-z",
                    "source_identity": mongo_archive.mongo_identity("mongodb://localhost:27017/")}
        for uri, database in (("mongodb://127.0.0.1:27017/", "restore_probe_test"),
                              ("mongodb://127.0.0.1:27019/", "qrldata-z"),
                              ("mongodb://127.0.0.1:27019/", "qrldata-v3")):
            with self.assertRaises(preflight.InvalidProfile):
                mongo_archive.validate_probe(manifest, uri, database)

    def test_config_keeps_credentials_out_of_arguments_and_has_private_mode(self):
        with mongo_archive.tool_config("mongodb://user:secret@127.0.0.1:27017/") as path:
            self.assertEqual(path.stat().st_mode & 0o777, 0o600)
            self.assertIn("user:secret", path.read_text())
        self.assertFalse(path.exists())

    def test_archive_and_probe_preserve_all_collections_without_drop(self):
        calls = []
        def invoke(arguments, environment=None):
            calls.append(arguments)
            if arguments[0] == "mongodump":
                destination = next(value.split("=", 1)[1] for value in arguments if value.startswith("--archive="))
                Path(destination).write_bytes(b"stand-in archive")
            return ""
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "backup"
            with patch.object(mongo_archive, "invoke", side_effect=invoke), patch.object(
                    mongo_archive, "inventory", return_value=self.inventory):
                result = mongo_archive.backup("mongodb://127.0.0.1:27017/", "qrldata-z",
                                              source, "writers-quiesced")
            self.assertEqual(result["sha256"], preflight.digest(source / "database.archive.gz"))
            with patch.object(mongo_archive, "invoke", side_effect=invoke), patch.object(
                    mongo_archive, "inventory", side_effect=[[], self.inventory]):
                restored = mongo_archive.restore_probe("mongodb://127.0.0.1:27019/",
                                                        "restore_probe_test", source)
            self.assertTrue(restored["counts_match_source"])
            self.assertTrue(any("--nsTo=restore_probe_test.*" in call for call in calls))
            self.assertTrue(any("--bypassDocumentValidation" in call for call in calls))
            self.assertFalse(any("--drop" in call for call in calls))
            self.assertFalse(any(any(value.startswith("--collection") for value in call) for call in calls))

    def test_changed_quiesced_source_leaves_only_partial_archive(self):
        def invoke(arguments, environment=None):
            path = next(value.split("=", 1)[1] for value in arguments if value.startswith("--archive="))
            Path(path).write_bytes(b"archive")
        with tempfile.TemporaryDirectory() as directory:
            source = Path(directory) / "backup"
            with patch.object(mongo_archive, "invoke", side_effect=invoke), patch.object(
                    mongo_archive, "inventory", side_effect=[self.inventory, []]):
                with self.assertRaisesRegex(preflight.InvalidProfile, "changed"):
                    mongo_archive.backup("mongodb://127.0.0.1:27017/", "qrldata-z", source,
                                         "writers-quiesced")
            self.assertFalse((source / "manifest.json").exists())
            self.assertTrue((source / "database.archive.gz.partial").exists())

    def test_hot_dump_explicitly_records_lack_of_consistency(self):
        def invoke(arguments, environment=None):
            path = next(value.split("=", 1)[1] for value in arguments if value.startswith("--archive="))
            Path(path).write_bytes(b"archive")
        with tempfile.TemporaryDirectory() as directory:
            with patch.object(mongo_archive, "invoke", side_effect=invoke), patch.object(
                    mongo_archive, "inventory", side_effect=[self.inventory, []]):
                result = mongo_archive.backup("mongodb://127.0.0.1:27017/", "qrldata-z",
                                              Path(directory) / "backup", "hot-standalone")
            self.assertIn("No point-in-time guarantee", result["consistency_note"])


if __name__ == "__main__":
    unittest.main()
