package synchroniser

import (
	"sync"
	"sync/atomic"
	"time"

	"QRL2MongoDB/configs"
	"QRL2MongoDB/db"

	"go.uber.org/zap"
)

// chainMutationMu serializes a complete indexed-chain mutation with reorg
// rollback. Block insertion, companion rows, contract trust records, pending
// state, and the sync high-water mark form one logical unit even though legacy
// storage writes span several Mongo operations.
var chainMutationMu sync.Mutex
var chainMutationLeaseExpiry atomic.Int64

func ConfigureChainMutationLease(expiresAt time.Time) {
	chainMutationLeaseExpiry.Store(expiresAt.UnixNano())
}

func lockChainMutation() {
	chainMutationMu.Lock()
	expiresAt := chainMutationLeaseExpiry.Load()
	if expiresAt > 0 && time.Now().UTC().UnixNano() >= expiresAt {
		chainMutationMu.Unlock()
		configs.Logger.Fatal("Syncer lease expired before chain mutation; refusing stale writer",
			zap.Time("expired_at", time.Unix(0, expiresAt).UTC()))
	}
}

// WithChainMutationLock runs fn after every in-process canonical writer has
// drained. Main uses this boundary to release the cross-process lease only
// after no local writer can still be inside a mutation.
func WithChainMutationLock(fn func() error) error {
	lockChainMutation()
	defer chainMutationMu.Unlock()
	return fn()
}

func reconcileSyncStateToIndexedHead() string {
	lockChainMutation()
	defer chainMutationMu.Unlock()

	indexedHead := db.GetLatestBlockNumberFromDB()
	forceUpdateSyncState(indexedHead)
	return indexedHead
}
