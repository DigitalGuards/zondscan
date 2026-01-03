#!/bin/bash
# Check sync status of node, syncer, and MongoDB

echo "=== QRL Explorer Sync Status ==="
echo "Time: $(date)"
echo ""

# Node sync status
echo "--- Zond Node ---"
NODE_STATUS=$(curl -s -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"zond_syncing","params":[],"id":1}' \
  http://localhost:8545)

if echo "$NODE_STATUS" | grep -q '"result":false'; then
  echo "Status: Fully synced"
  CURRENT=$(curl -s -X POST -H "Content-Type: application/json" \
    --data '{"jsonrpc":"2.0","method":"zond_blockNumber","params":[],"id":1}' \
    http://localhost:8545 | python3 -c "import sys,json; print(int(json.load(sys.stdin)['result'], 16))")
  echo "Block height: $CURRENT"
else
  CURRENT=$(echo "$NODE_STATUS" | python3 -c "import sys,json; print(int(json.load(sys.stdin)['result']['currentBlock'], 16))")
  HIGHEST=$(echo "$NODE_STATUS" | python3 -c "import sys,json; print(int(json.load(sys.stdin)['result']['highestBlock'], 16))")
  PCT=$(python3 -c "print(f'{($CURRENT / $HIGHEST * 100):.1f}')")
  echo "Status: Syncing"
  echo "Current: $CURRENT / $HIGHEST ($PCT%)"
  echo "Remaining: $((HIGHEST - CURRENT)) blocks"
fi

echo ""
echo "--- MongoDB (Syncer) ---"
MONGO_BLOCKS=$(mongosh --quiet --eval 'db.blocks.countDocuments()' qrldata-z 2>/dev/null)
MONGO_LATEST=$(mongosh --quiet --eval 'db.sync_state.findOne({_id: "last_synced_block"})?.block_number || "0x0"' qrldata-z 2>/dev/null)
MONGO_LATEST_DEC=$(python3 -c "print(int('$MONGO_LATEST', 16))")
MONGO_TXS=$(mongosh --quiet --eval 'db.transactionByAddress.countDocuments()' qrldata-z 2>/dev/null)
MONGO_PENDING=$(mongosh --quiet --eval 'db.pending_transactions.countDocuments()' qrldata-z 2>/dev/null)

echo "Blocks in DB: $MONGO_BLOCKS"
echo "Latest block: $MONGO_LATEST_DEC ($MONGO_LATEST)"
echo "Transactions indexed: $MONGO_TXS"
echo "Pending transactions: $MONGO_PENDING"

echo ""
echo "--- Comparison ---"
if [ "$CURRENT" = "$MONGO_LATEST_DEC" ]; then
  echo "✓ Syncer is caught up with node"
else
  DIFF=$((CURRENT - MONGO_LATEST_DEC))
  echo "⚠ Syncer is $DIFF blocks behind node"
fi
