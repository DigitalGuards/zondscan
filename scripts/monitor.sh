#!/usr/bin/env bash
set -uo pipefail

WEBHOOK="${DISCORD_WEBHOOK_URL:?Set DISCORD_WEBHOOK_URL environment variable}"
STATE_FILE="/tmp/monitor-alert-state"

# Track which alerts have fired to avoid spam
touch "$STATE_FILE"

alert() {
    local key="$1"
    local msg="$2"

    # Only alert once per issue (until resolved)
    if grep -qxF "$key" "$STATE_FILE" 2>/dev/null; then
        return
    fi

    echo "$key" >> "$STATE_FILE"
    curl -s -H "Content-Type: application/json" \
        -d "{\"content\":\"🚨 **Monitor Alert** ($(hostname))\\n$msg\"}" \
        "$WEBHOOK" >/dev/null
}

resolve() {
    local key="$1"
    local msg="$2"

    if grep -qxF "$key" "$STATE_FILE" 2>/dev/null; then
        sed -i "/^${key}$/d" "$STATE_FILE"
        curl -s -H "Content-Type: application/json" \
            -d "{\"content\":\"✅ **Resolved** ($(hostname))\\n$msg\"}" \
            "$WEBHOOK" >/dev/null
    fi
}

# --- PM2 Process Checks ---
for proc in qrl-gqrl qrl-beacon syncer handler frontend; do
    status=$(pm2 jlist 2>/dev/null | python3 -c "
import sys,json
procs = json.load(sys.stdin)
for p in procs:
    if p['name'] == '$proc':
        print(p['pm2_env']['status'])
        break
else:
    print('missing')
" 2>/dev/null || echo "error")

    if [ "$status" = "online" ]; then
        resolve "pm2-$proc" "$proc is back online"
    else
        alert "pm2-$proc" "**$proc** is $status"
    fi
done

# --- gqrl RPC responsive ---
BLOCK_HEX=$(curl -sf --max-time 5 -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"qrl_blockNumber","params":[],"id":1}' \
    http://127.0.0.1:8545 | python3 -c "import sys,json; print(json.load(sys.stdin)['result'])" 2>/dev/null)

if [ -z "$BLOCK_HEX" ]; then
    alert "rpc-down" "**gqrl RPC** not responding on :8545"
else
    resolve "rpc-down" "gqrl RPC is responding again"
    NODE_BLOCK=$((16#${BLOCK_HEX#0x}))

    # --- Sync check: is node behind network? ---
    SYNC_RESULT=$(curl -sf --max-time 5 -X POST -H "Content-Type: application/json" \
        --data '{"jsonrpc":"2.0","method":"qrl_syncing","params":[],"id":1}' \
        http://127.0.0.1:8545 | python3 -c "import sys,json; print(json.load(sys.stdin)['result'])" 2>/dev/null)

    if [ "$SYNC_RESULT" != "False" ] && [ -n "$SYNC_RESULT" ]; then
        CURRENT=$(echo "$SYNC_RESULT" | python3 -c "import sys,ast; d=ast.literal_eval(sys.stdin.read()); print(int(d['currentBlock'],16))" 2>/dev/null)
        HIGHEST=$(echo "$SYNC_RESULT" | python3 -c "import sys,ast; d=ast.literal_eval(sys.stdin.read()); print(int(d['highestBlock'],16))" 2>/dev/null)
        if [ -n "$CURRENT" ] && [ -n "$HIGHEST" ]; then
            BEHIND=$((HIGHEST - CURRENT))
            if [ "$BEHIND" -gt 50 ]; then
                alert "node-behind" "**Node** is $BEHIND blocks behind network ($CURRENT / $HIGHEST)"
            else
                resolve "node-behind" "Node caught up (block $CURRENT)"
            fi
        fi
    else
        resolve "node-behind" "Node is fully synced (block $NODE_BLOCK)"
    fi

    # --- Syncer behind node ---
    SYNCER_BLOCK=$(mongosh --quiet --eval 'db.blocks.find().sort({number:-1}).limit(1).toArray()[0]?.number || 0' qrldata-z 2>/dev/null)
    if [ -n "$SYNCER_BLOCK" ] && [ "$SYNCER_BLOCK" != "0" ]; then
        SYNCER_BEHIND=$((NODE_BLOCK - SYNCER_BLOCK))
        if [ "$SYNCER_BEHIND" -gt 50 ]; then
            alert "syncer-behind" "**Syncer** is $SYNCER_BEHIND blocks behind node (syncer: $SYNCER_BLOCK, node: $NODE_BLOCK)"
        else
            resolve "syncer-behind" "Syncer caught up (block $SYNCER_BLOCK)"
        fi
    fi
fi

# --- Frontend responding ---
HTTP_CODE=$(curl -sf --max-time 5 -o /dev/null -w "%{http_code}" http://127.0.0.1:3000/)
if [ "$HTTP_CODE" = "200" ]; then
    resolve "frontend-http" "Frontend is responding again"
else
    alert "frontend-http" "**Frontend** not responding (HTTP $HTTP_CODE)"
fi

# --- Backend API responding ---
HTTP_CODE=$(curl -sf --max-time 5 -o /dev/null -w "%{http_code}" http://127.0.0.1:8082/health)
if [ "$HTTP_CODE" = "200" ]; then
    resolve "backend-http" "Backend API is responding again"
else
    alert "backend-http" "**Backend API** not responding on :8082 (HTTP $HTTP_CODE)"
fi

# --- MongoDB ---
if mongosh --quiet --eval 'db.runCommand({ping:1}).ok' qrldata-z 2>/dev/null | grep -q 1; then
    resolve "mongodb" "MongoDB is responding again"
else
    alert "mongodb" "**MongoDB** not responding"
fi
