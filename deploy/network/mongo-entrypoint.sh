#!/usr/bin/env bash
set -euo pipefail

if [[ "${EXPLORER_NETWORK:-}" != v3 || "${MONGO_DB_NAME:-}" != qrldata-v3 ]]; then
  echo "Refusing MongoDB preparation with an unexpected network or database" >&2
  exit 1
fi

for secret in admin_user admin_password app_user app_password replica_key; do
  if [[ ! -s "/run/secrets/$secret" ]]; then
    echo "A required private MongoDB credential file is empty" >&2
    exit 1
  fi
  install -o mongodb -g mongodb -m 600 "/run/secrets/$secret" "/run/mongodb/$secret"
done

exec /usr/local/bin/docker-entrypoint.sh mongod \
  --replSet zondscan-v3 --keyFile /run/mongodb/replica_key --bind_ip_all
