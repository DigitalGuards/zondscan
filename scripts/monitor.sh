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

# Debounced check: requires $threshold consecutive failing ticks before
# alerting, so a single transient blip (a deploy restart, a GC pause) doesn't
# page anyone. Passes reset the streak and resolve immediately.
FAIL_COUNT_DIR="/tmp/monitor-fail-counts-$(id -u)"
mkdir -p "$FAIL_COUNT_DIR"

check_debounced() {
    local key="$1"
    local threshold="$2"
    local passed="$3"    # "pass" or "fail"
    local alert_msg="$4"
    local resolve_msg="$5"
    local fail_file="$FAIL_COUNT_DIR/$key"

    if [ "$passed" = "pass" ]; then
        rm -f "$fail_file"
        resolve "$key" "$resolve_msg"
    else
        local count=$(( $(cat "$fail_file" 2>/dev/null || echo 0) + 1 ))
        echo "$count" > "$fail_file"
        if [ "$count" -ge "$threshold" ]; then
            alert "$key" "$alert_msg"
        fi
    fi
}

# --- Reboot detection ---
# btime in /proc/stat is the boot epoch. Persist outside /tmp so reboots
# (which wipe /tmp) are detectable on the next monitor tick.
BOOT_FILE="$HOME/.monitor-last-boot"
CUR_BTIME=$(awk '/^btime / {print $2}' /proc/stat)
if [ -n "$CUR_BTIME" ]; then
    PREV_BTIME=$(cat "$BOOT_FILE" 2>/dev/null || echo "")
    if [ -z "$PREV_BTIME" ]; then
        echo "$CUR_BTIME" > "$BOOT_FILE"
    elif [ "$CUR_BTIME" != "$PREV_BTIME" ]; then
        BOOT_TS=$(date -d "@$CUR_BTIME" '+%Y-%m-%d %H:%M:%S %Z')
        UPTIME=$(uptime -p)
        curl -s -H "Content-Type: application/json" \
            -d "{\"content\":\"🔄 **Reboot detected** ($(hostname))\\nBoot time: $BOOT_TS\\n$UPTIME\"}" \
            "$WEBHOOK" >/dev/null
        echo "$CUR_BTIME" > "$BOOT_FILE"
    fi
fi

# --- PM2 Process Checks ---
for proc in synchroniser handler frontend; do
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
        check_debounced "pm2-$proc" 2 "pass" "" "$proc is back online"
    else
        check_debounced "pm2-$proc" 2 "fail" "**$proc** is $status" ""
    fi
done

# --- gqrl RPC responsive ---
BLOCK_HEX=$(curl -sf --max-time 5 -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"qrl_blockNumber","params":[],"id":1}' \
    http://127.0.0.1:8545 | python3 -c "import sys,json; print(json.load(sys.stdin)['result'])" 2>/dev/null)

if [ -z "$BLOCK_HEX" ]; then
    check_debounced "rpc-down" 2 "fail" "**gqrl RPC** not responding on :8545" ""
else
    check_debounced "rpc-down" 2 "pass" "" "gqrl RPC is responding again"
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

    # --- One-shot sync completion: notify + restart zondscan ---
    # Persist outside /tmp: a reboot wipes /tmp, and the node is already synced
    # post-reboot, so a /tmp-based flag would re-trigger this every time.
    # Deliberately does NOT touch MongoDB. A monitoring script should never
    # have destructive DB access; if a clean re-index is ever needed, that's
    # a manual, explicit operation, not something a cron job can misfire into.
    SYNC_DONE_FLAG="$HOME/.monitor-sync-complete"
    if [ ! -f "$SYNC_DONE_FLAG" ]; then
        # Node is synced (qrl_syncing returns false) and has enough blocks
        if [ "$SYNC_RESULT" = "False" ] && [ "$NODE_BLOCK" -gt 100 ]; then
            touch "$SYNC_DONE_FLAG"
            pm2 restart synchroniser handler frontend >/dev/null 2>&1
            pm2 save >/dev/null 2>&1
            curl -s -H "Content-Type: application/json" \
                -d "{\"content\":\"🎉 **Sync Complete** ($(hostname))\\nNode fully synced on chain ID 1337 (block $NODE_BLOCK).\\nZondscan restarted: syncer, handler, frontend.\"}" \
                "$WEBHOOK" >/dev/null
        fi
    fi
fi

# --- Remote beacon API responsive ---
# qrysm's /eth/v1/* paths return 404; /qrl/v1alpha1/beacon/chainhead is the canonical liveness probe.
# Beacon is flaky under sync load, so it gets a longer debounce than the other checks.
BEACON_FAIL_THRESHOLD=10
HTTP_CODE=$(curl -sf --max-time 10 -o /dev/null -w "%{http_code}" http://127.0.0.1:3500/qrl/v1alpha1/beacon/chainhead)
if [ "$HTTP_CODE" = "200" ]; then
    check_debounced "beacon-down" "$BEACON_FAIL_THRESHOLD" "pass" "" "Beacon API is responding again"
else
    check_debounced "beacon-down" "$BEACON_FAIL_THRESHOLD" "fail" "**Beacon API** (127.0.0.1:3500) not responding (HTTP $HTTP_CODE, ${BEACON_FAIL_THRESHOLD}min consecutive)" ""
fi

# --- Frontend responding ---
HTTP_CODE=$(curl -sf --max-time 5 -o /dev/null -w "%{http_code}" http://127.0.0.1:3000/)
if [ "$HTTP_CODE" = "200" ]; then
    check_debounced "frontend-http" 2 "pass" "" "Frontend is responding again"
else
    check_debounced "frontend-http" 2 "fail" "**Frontend** not responding (HTTP $HTTP_CODE)" ""
fi

# --- Backend API responding ---
HTTP_CODE=$(curl -sf --max-time 5 -o /dev/null -w "%{http_code}" http://127.0.0.1:8082/health)
if [ "$HTTP_CODE" = "200" ]; then
    check_debounced "backend-http" 2 "pass" "" "Backend API is responding again"
else
    check_debounced "backend-http" 2 "fail" "**Backend API** not responding on :8082 (HTTP $HTTP_CODE)" ""
fi

# --- MongoDB ---
if mongosh --quiet --eval 'db.runCommand({ping:1}).ok' qrldata-z 2>/dev/null | grep -q 1; then
    check_debounced "mongodb" 2 "pass" "" "MongoDB is responding again"
else
    check_debounced "mongodb" 2 "fail" "**MongoDB** not responding" ""
fi
