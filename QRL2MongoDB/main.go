package main

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/db"
	"QRL2MongoDB/metadata"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/synchroniser"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// validateEnv fails fast if the node endpoint env vars are missing. Without
// this, an empty NODE_URLS/NODE_URL only logs a warning in
// rpc.newEndpointSelectorFromEnv and the first RPC call returns "no node
// endpoints configured", so Sync exits silently with no obvious cause.
// MONGOURI is validated inside configs.ConnectDB, called explicitly below.
func validateEnv() {
	if os.Getenv("NODE_URLS") == "" && os.Getenv("NODE_URL") == "" {
		log.Fatal("Required environment variable NODE_URLS (or legacy NODE_URL) is not set")
	}
}

func main() {
	// Load .env before anything reads os.Getenv. The old import-time
	// ConnectDB loaded it as a side effect; validateEnv below fatals on a
	// missing NODE_URLS if this doesn't run first (2026-07-11 prod incident).
	configs.LoadEnv()

	// Ensure logger resources are properly released
	defer configs.Logger.Sync()

	// Route the zap global logger (zap.L()) to our file logger. The rpc
	// package logs failover events through zap.L(); without this call those
	// go to zap's default no-op logger and are silently discarded.
	zap.ReplaceGlobals(configs.Logger)

	configs.Logger.Info("Initializing QRL to MongoDB synchronizer...")

	// Fail fast before any sync work if required env vars are missing.
	validateEnv()

	configs.Logger.Info("Connecting to MongoDB and RPC node...")

	// Connect explicitly: nothing connects at import time anymore, and a
	// missing/unreachable MONGOURI must be fatal for the syncer.
	if err := configs.ConnectDB(); err != nil {
		configs.Logger.Fatal("Failed to connect to MongoDB", zap.Error(err))
	}
	// Enforce one active chain writer across processes. The in-process mutation
	// mutex serializes local workers with rollback; this renewable Mongo lease
	// prevents a second syncer instance from bypassing that boundary.
	const syncerLeaseTTL = 2 * time.Minute
	leaseOwner, err := configs.NewSyncerLeaseOwner()
	if err != nil {
		configs.Logger.Fatal("Failed to create syncer lease identity", zap.Error(err))
	}
	leaseCtx, leaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
	syncerLease, err := configs.AcquireSyncerLease(leaseCtx, leaseOwner, syncerLeaseTTL)
	leaseCancel()
	if err != nil {
		configs.Logger.Fatal("Failed to acquire exclusive syncer lease", zap.Error(err))
	}
	synchroniser.ConfigureChainMutationLease(syncerLease.ExpiresAt)
	leaseStopCh := make(chan struct{})
	leaseDoneCh := make(chan struct{})
	go func() {
		defer close(leaseDoneCh)
		ticker := time.NewTicker(syncerLeaseTTL / 3)
		defer ticker.Stop()
		lease := syncerLease
		for {
			select {
			case <-leaseStopCh:
				return
			case <-ticker.C:
				renewCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				renewed, renewErr := configs.RenewSyncerLease(renewCtx, lease, syncerLeaseTTL)
				cancel()
				if renewErr != nil {
					configs.Logger.Fatal("Lost exclusive syncer lease; stopping before further chain writes",
						zap.Error(renewErr))
				}
				lease = renewed
				synchroniser.ConfigureChainMutationLease(renewed.ExpiresAt)
			}
		}
	}()
	failStartupAfterLease := func(message string, startupErr error) {
		configs.Logger.Error(message, zap.Error(startupErr))
		close(leaseStopCh)
		<-leaseDoneCh
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
		releaseErr := synchroniser.WithChainMutationLock(func() error {
			return configs.ReleaseSyncerLease(releaseCtx, syncerLease)
		})
		releaseCancel()
		if releaseErr != nil {
			configs.Logger.Error("Failed to release syncer lease after startup error",
				zap.Error(releaseErr))
		}
		disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
		if disconnectErr := configs.DB.Disconnect(disconnectCtx); disconnectErr != nil {
			configs.Logger.Error("Failed to disconnect MongoDB after startup error",
				zap.Error(disconnectErr))
		}
		disconnectCancel()
		_ = configs.Logger.Sync()
		os.Exit(1)
	}

	// Collection, index, and seed mutations run only after the exclusive lease
	// is held and renewing. This keeps startup bootstrap from racing rollback or
	// maintenance writers in another process.
	if err := configs.BootstrapDB(); err != nil {
		failStartupAfterLease("Failed to bootstrap MongoDB", err)
		return
	}
	if err := db.ValidateBlockIngestionMigration(); err != nil {
		failStartupAfterLease(
			"Block ingestion migration gate failed; run the documented block companion reindex before starting this version",
			err,
		)
		return
	}
	// Token collection indexes and legacy numeric-order backfills are an
	// integrity prerequisite for every token-adjacent writer. Complete this
	// leased preflight before balance, metadata, receipt, or block workers can
	// run their immediate startup pass.
	if err := synchroniser.InitializeTokenCollections(); err != nil {
		failStartupAfterLease("Token collection readiness gate failed", err)
		return
	}

	// stopCh is closed when a termination signal is received. Sync() and other
	// long-running loops should watch this channel so they can finish their current
	// unit of work and exit cleanly.
	stopCh := make(chan struct{})

	// doneCh is closed by the main sync goroutine once it has finished.
	doneCh := make(chan struct{})
	syncErrCh := make(chan error, 1)

	// Create a buffered channel to avoid signal notification drops.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	configs.Logger.Info("Starting blockchain synchronization process...")
	// Log only the host: MONGOURI may embed credentials.
	if u, err := url.Parse(os.Getenv("MONGOURI")); err == nil && u.Host != "" {
		configs.Logger.Info("MongoDB host: " + u.Host)
	}
	configs.Logger.Info("Node URLs: " + strings.Join(rpc.Endpoints().AllURLs(), ", "))

	// Start health check server for Kubernetes probes. The handler probes the
	// configured node endpoints via the same selector the syncer uses, so
	// `/health` reflects whether the syncer can actually make RPC calls right
	// now (not just that the process is alive).
	healthMux := http.NewServeMux()
	healthMux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		// Probe synchronously under the request context with a 5s cap.
		// ProbeChainHeadCtx honours ctx for the in-flight HTTP request, so
		// a client disconnect cancels the probe instead of leaking a
		// detached goroutine that outlives the handler.
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		height, probeErr := rpc.ProbeChainHeadCtx(ctx)

		w.Header().Set("Content-Type", "application/json")
		payload := map[string]interface{}{
			"endpoints":   rpc.Endpoints().AllURLs(),
			"currentUrl":  rpc.Endpoints().CurrentURL(),
			"primaryUrl":  rpc.Endpoints().PrimaryURL(),
			"probeHeight": height,
		}
		if probeErr != nil {
			payload["status"] = "degraded"
			payload["error"] = probeErr.Error()
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			payload["status"] = "ok"
			w.WriteHeader(http.StatusOK)
		}
		body, _ := json.Marshal(payload)
		w.Write(body)
	})
	healthPort := os.Getenv("HEALTH_PORT")
	if healthPort == "" {
		healthPort = "8083"
	}
	healthServer := &http.Server{
		Addr:              ":" + healthPort,
		Handler:           healthMux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	go func() {
		configs.Logger.Info("Starting health check server on port " + healthPort)
		if err := healthServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			configs.Logger.Error("Health server failed", zap.Error(err))
		}
	}()

	// Start pending transaction sync (this is not started in sync.go).
	// stopCh is threaded in so the mempool/cleanup/verify tickers stop
	// accepting new work when a shutdown signal arrives.
	configs.Logger.Info("Starting pending transaction sync service...")
	synchroniser.StartPendingTransactionSync(stopCh)
	configs.Logger.Info("Starting stale balance reconciliation service...")
	synchroniser.StartBalanceReconciliationJob(stopCh)

	// Phase 3a: start the off-chain NFT collection metadata fetcher.
	// Background goroutine that polls contractCode for unfetched
	// metadataURI rows and resolves them through the configured IPFS
	// gateway. Self-disables via METADATA_FETCHER_ENABLED=false.
	metadataCtx, cancelMetadata := context.WithCancel(context.Background())
	metadataSvc := metadata.NewService()
	metadataSvc.Start(metadataCtx)

	// Run the main sync in a goroutine so the signal handler above can observe doneCh.
	go func() {
		defer close(doneCh)
		// Sync will now handle starting wallet count and contract reprocessing
		// services after initial sync is complete. stopCh is threaded in so
		// every background ticker goroutine Sync starts can observe shutdown.
		syncErrCh <- synchroniser.Sync(stopCh)
	}()

	// Block until either sync finishes naturally or a shutdown signal arrives.
	var syncErr error
	syncFinished := false
	select {
	case <-doneCh:
		syncErr = <-syncErrCh
		syncFinished = true
		if syncErr != nil {
			configs.Logger.Error("Synchronization stopped on a fail-closed startup or runtime error",
				zap.Error(syncErr))
		} else {
			configs.Logger.Info("Sync completed, exiting normally")
		}
	case sig := <-sigCh:
		configs.Logger.Info("Received shutdown signal, initiating graceful shutdown...",
			zap.String("signal", sig.String()))
	}

	// Seal every auxiliary writer registry before waiting. This closes the
	// race where initial sync finishes while shutdown is starting and tries to
	// launch contract or wallet workers after the wait has begun.
	var stopOnce sync.Once
	stopOnce.Do(func() { close(stopCh) })
	synchroniser.BeginBackgroundShutdown()
	cancelMetadata()
	metadataSvc.Stop()

	healthCtx, healthCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := healthServer.Shutdown(healthCtx); err != nil {
		configs.Logger.Warn("Health server shutdown failed", zap.Error(err))
	}
	healthCancel()

	// The lease keeps renewing while every writer drains. A timeout exits
	// without deleting the lease, leaving the TTL as a takeover safety delay.
	drainCtx, drainCancel := context.WithTimeout(context.Background(), 30*time.Second)
	drained := true
	select {
	case <-doneCh:
	case <-drainCtx.Done():
		drained = false
	}
	if drained {
		if err := synchroniser.WaitForBackgroundWorkers(drainCtx); err != nil {
			drained = false
		}
	}
	if drained {
		if err := metadataSvc.Wait(drainCtx); err != nil {
			drained = false
		}
	}
	if drained && !syncFinished {
		syncErr = <-syncErrCh
		syncFinished = true
	}
	drainCancel()

	if !drained {
		configs.Logger.Error("Graceful shutdown timed out; retaining syncer lease until TTL expiry")
		close(leaseStopCh)
		<-leaseDoneCh
		_ = configs.Logger.Sync()
		os.Exit(1)
	}

	close(leaseStopCh)
	<-leaseDoneCh
	releaseErr := synchroniser.WithChainMutationLock(func() error {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer releaseCancel()
		return configs.ReleaseSyncerLease(releaseCtx, syncerLease)
	})
	if releaseErr != nil {
		configs.Logger.Error("Failed to release syncer lease", zap.Error(releaseErr))
	}

	disconnectCtx, disconnectCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := configs.DB.Disconnect(disconnectCtx); err != nil {
		configs.Logger.Error("Error disconnecting from MongoDB", zap.Error(err))
	} else {
		configs.Logger.Info("MongoDB disconnected cleanly")
	}
	disconnectCancel()
	configs.Logger.Info("Synchronizer stopped")
	if syncErr != nil {
		_ = configs.Logger.Sync()
		os.Exit(1)
	}
}
