package synchroniser

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/db"
	"context"
	"time"

	"go.uber.org/zap"
)

const balanceReconciliationInterval = 5 * time.Second

// StartBalanceReconciliationJob refreshes snapshots invalidated by a reorg.
// The shared mutation lock prevents rollback and reconciliation from crossing,
// and the background registry keeps the writer inside the shutdown lease.
func StartBalanceReconciliationJob(stopCh <-chan struct{}) {
	runPeriodicTask(func() {
		lockChainMutation()
		defer chainMutationMu.Unlock()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		incomplete, err := tokenBlockQueueHasIncompleteWork(ctx)
		cancel()
		if err != nil {
			configs.Logger.Error("Failed to check token block queue before balance reconciliation", zap.Error(err))
			return
		}
		if incomplete {
			configs.Logger.Debug("Deferring balance reconciliation until token block queue is complete")
			return
		}
		if err := db.ReconcileStaleBalances(100); err != nil {
			configs.Logger.Error("Failed to reconcile stale balances", zap.Error(err))
		}
	}, balanceReconciliationInterval, "balance reconciliation", stopCh)
}
