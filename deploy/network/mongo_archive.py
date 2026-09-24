#!/usr/bin/env python3
"""Archive one entire explorer database and restore only into a separate probe server."""

import argparse
from contextlib import contextmanager
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys
import tempfile
from urllib.parse import urlsplit

from preflight import InvalidProfile, digest, require


def now():
    return datetime.now(timezone.utc).isoformat()


def mongo_identity(uri):
    parsed = urlsplit(uri)
    require(parsed.scheme == "mongodb" and parsed.hostname in ("localhost", "127.0.0.1"),
            "Archive tools require one loopback MongoDB endpoint")
    require(parsed.path in ("", "/"), "Archive URI must omit a database; supply --database explicitly")
    require(not parsed.fragment, "MongoDB URI cannot include a fragment")
    return hashlib.sha256(f"127.0.0.1:{parsed.port or 27017}".encode()).hexdigest()


def database_name(value):
    require(re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9_-]{0,62}", value) and
            value.lower() not in ("admin", "local", "config"), "Unsafe database name")
    return value


def invoke(arguments, environment=None):
    result = subprocess.run(arguments, env=environment, capture_output=True, text=True, check=False)
    # MongoDB errors may contain the connection URI. Keep them out of console output.
    require(result.returncode == 0, f"{Path(arguments[0]).name} failed (exit {result.returncode})")
    return result.stdout


@contextmanager
def tool_config(uri):
    # A JSON string is a valid YAML scalar. The file avoids secrets in process arguments.
    with tempfile.TemporaryDirectory(prefix="zondscan-mongo-") as directory:
        path = Path(directory) / "connection.yaml"
        descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
        with os.fdopen(descriptor, "w") as handle:
            handle.write("uri: " + json.dumps(uri) + "\n")
        yield path


def inventory(uri, database):
    environment = dict(os.environ, ZONDSCAN_ARCHIVE_URI=uri, ZONDSCAN_ARCHIVE_DB=database)
    script = """
const database = new Mongo(process.env.ZONDSCAN_ARCHIVE_URI).getDB(process.env.ZONDSCAN_ARCHIVE_DB);
const collections = database.getCollectionInfos().map(info => {
  if (info.type === 'view') return {name: info.name, type: info.type, options: info.options || {}, count: null, indexes: []};
  const stats = database.runCommand({collStats: info.name});
  if (!stats.ok) throw new Error('Collection statistics unavailable');
  return {name: info.name, type: info.type, options: info.options || {}, count: Number(stats.count),
    indexes: database.getCollection(info.name).getIndexes().map(index => {
      const {ns, ...definition} = index;
      return definition;
    }).sort((a, b) => a.name.localeCompare(b.name))};
}).sort((a, b) => a.name.localeCompare(b.name));
print(JSON.stringify(collections));
"""
    return json.loads(invoke(["mongosh", "--quiet", "--nodb", "--eval", script], environment))


def write_json(path, value):
    descriptor = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(descriptor, "w") as handle:
        json.dump(value, handle, indent=2, sort_keys=True)
        handle.write("\n")


def backup(uri, database, destination, consistency):
    database_name(database)
    identity = mongo_identity(uri)
    require(consistency in ("hot-standalone", "writers-quiesced"), "Invalid consistency mode")
    destination.mkdir(mode=0o700, parents=True, exist_ok=False)
    started = now()
    before = inventory(uri, database)
    require(before, "Source database is empty or does not exist")
    partial = destination / "database.archive.gz.partial"
    with tool_config(uri) as config:
        invoke(["mongodump", f"--config={config}", f"--db={database}", "--gzip",
                f"--archive={partial}"])
    require(partial.is_file() and partial.stat().st_size > 0, "Dump did not produce an archive")
    partial.chmod(0o600)
    after = inventory(uri, database)
    if consistency == "writers-quiesced":
        require(before == after, "Database changed during the quiesced dump; archive remains partial")
    archive = destination / "database.archive.gz"
    partial.rename(archive)
    metadata = {
        "version": 1, "database": database, "source_identity": identity,
        "started_at": started, "completed_at": now(), "consistency": consistency,
        "consistency_note": (
            "Operator attested all writers were stopped for the entire dump; inventories match."
            if consistency == "writers-quiesced" else
            "Hot standalone logical dump; collections may reflect different times. No point-in-time guarantee."
        ),
        "archive": archive.name, "bytes": archive.stat().st_size, "sha256": digest(archive),
        "inventory_before": before, "inventory_after": after,
    }
    write_json(destination / "manifest.json", metadata)
    (destination / "SHA256SUMS").write_text(f"{metadata['sha256']}  {archive.name}\n")
    (destination / "SHA256SUMS").chmod(0o600)
    return metadata


def validate_probe(manifest, uri, database):
    database_name(database)
    database_name(manifest["database"])
    require(manifest.get("version") == 1, "Unsupported archive manifest")
    require(database.startswith("restore_probe_") and database != manifest["database"],
            "Restore destination must be a new restore_probe_ database")
    require(mongo_identity(uri) != manifest["source_identity"],
            "Restore probe must use a separate MongoDB server port")


def restore_probe(uri, database, source):
    manifest = json.loads((source / "manifest.json").read_text())
    validate_probe(manifest, uri, database)
    require(manifest.get("archive") == "database.archive.gz", "Unexpected archive filename")
    archive = source / manifest["archive"]
    require(digest(archive) == manifest["sha256"] and archive.stat().st_size == manifest["bytes"],
            "Archive checksum or size mismatch")
    require(not inventory(uri, database), "Restore destination already contains collections")
    with tool_config(uri) as config:
        invoke(["mongorestore", f"--config={config}", "--gzip", f"--archive={archive}",
                f"--nsInclude={manifest['database']}.*", f"--nsFrom={manifest['database']}.*",
                f"--nsTo={database}.*", "--bypassDocumentValidation", "--stopOnError"])
    restored = inventory(uri, database)
    baseline = manifest["inventory_after"]
    comparable = lambda rows: [{k: v for k, v in row.items() if k != "count"} for row in rows]
    require(comparable(restored) == comparable(baseline), "Restored collection or index inventory differs")
    counts_match = restored == baseline
    if manifest["consistency"] == "writers-quiesced":
        require(counts_match, "Restored document counts differ from the quiesced source")
    result = {"completed_at": now(), "database": database, "archive_sha256": manifest["sha256"],
              "collections_and_indexes_match": True, "counts_match_source": counts_match,
              "source_consistency": manifest["consistency"], "inventory": restored}
    write_json(source / f"{database}.json", result)
    return result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    subparsers = parser.add_subparsers(dest="command", required=True)
    dump = subparsers.add_parser("backup")
    dump.add_argument("--uri-env", default="MONGO_BACKUP_URI")
    dump.add_argument("--database", required=True)
    dump.add_argument("--output", type=Path, required=True)
    dump.add_argument("--consistency", choices=("hot-standalone", "writers-quiesced"), required=True)
    restore = subparsers.add_parser("restore-probe")
    restore.add_argument("--uri-env", default="MONGO_RESTORE_URI")
    restore.add_argument("--database", required=True)
    restore.add_argument("--input", type=Path, required=True)
    args = parser.parse_args()
    try:
        uri = os.environ.get(args.uri_env, "")
        require(bool(uri), f"Missing environment variable {args.uri_env}")
        if args.command == "backup":
            result = backup(uri, args.database, args.output, args.consistency)
            print(f"Archived entire database; SHA-256 {result['sha256']}; {result['consistency']}")
        else:
            result = restore_probe(uri, args.database, args.input)
            print("Restore probe passed; source counts match: " + str(result["counts_match_source"]))
    except (OSError, ValueError, KeyError, TypeError) as error:
        print(f"Archive operation refused: {error}", file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
