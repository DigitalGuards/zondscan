package main

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/synchroniser"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
)

func main() {
	// Ensure logger resources are properly released
	defer configs.Logger.Sync()

	configs.Logger.Info("Initializing QRL to MongoDB synchronizer...")
	configs.Logger.Info("Connecting to MongoDB and RPC node...")

	// stopCh is closed when a termination signal is received. Sync() and other
	// long-running loops should watch this channel so they can finish their current
	// unit of work and exit cleanly.
	stopCh := make(chan struct{})

	// doneCh is closed by the main sync goroutine once it has finished.
	doneCh := make(chan struct{})

	// Create a buffered channel to avoid signal notification drops.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-sigCh
		configs.Logger.Info("Received shutdown signal, initiating graceful shutdown...",
			zap.String("signal", sig.String()))

		// Signal all workers to stop accepting new work.
		close(stopCh)

		// Wait up to 30 seconds for in-flight processing to complete.
		select {
		case <-doneCh:
			configs.Logger.Info("All sync work completed, shutting down cleanly")
		case <-time.After(30 * time.Second):
			configs.Logger.Warn("Graceful shutdown timed out after 30s, forcing exit")
		}

		// Disconnect MongoDB cleanly.
		disconnectCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := configs.DB.Disconnect(disconnectCtx); err != nil {
			configs.Logger.Error("Error disconnecting from MongoDB", zap.Error(err))
		} else {
			configs.Logger.Info("MongoDB disconnected cleanly")
		}

		configs.Logger.Info("Synchronizer stopped")
		os.Exit(0)
	}()

	configs.Logger.Info("Starting blockchain synchronization process...")
	configs.Logger.Info("MongoDB URL: " + os.Getenv("MONGOURI"))
	configs.Logger.Info("Node URLs: " + strings.Join(rpc.Endpoints().AllURLs(), ", "))

	// Start health check server for Kubernetes probes. The handler probes the
	// configured node endpoints via the same selector the syncer uses, so
	// `/health` reflects whether the syncer can actually make RPC calls right
	// now (not just that the process is alive).
	go func() {
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			done := make(chan struct {
				height uint64
				err    error
			}, 1)
			go func() {
				h, e := rpc.ProbeChainHead()
				done <- struct {
					height uint64
					err    error
				}{h, e}
			}()
			var height uint64
			var probeErr error
			select {
			case <-ctx.Done():
				probeErr = ctx.Err()
			case res := <-done:
				height = res.height
				probeErr = res.err
			}

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
		configs.Logger.Info("Starting health check server on port " + healthPort)
		if err := http.ListenAndServe(":"+healthPort, nil); err != nil {
			configs.Logger.Error("Health server failed", zap.Error(err))
		}
	}()

	// Start pending transaction sync (this is not started in sync.go).
	// stopCh is threaded in so the mempool/cleanup/verify tickers stop
	// accepting new work when a shutdown signal arrives.
	configs.Logger.Info("Starting pending transaction sync service...")
	synchroniser.StartPendingTransactionSync(stopCh)

	// Phase 3a: start the off-chain NFT collection metadata fetcher.
	// Background goroutine that polls contractCode for unfetched
	// metadataURI rows and resolves them through the configured IPFS
	// gateway. Self-disables via METADATA_FETCHER_ENABLED=false.
	metadataCtx, cancelMetadata := context.WithCancel(context.Background())
	metadataSvc := synchroniser.NewMetadataService()
	metadataSvc.Start(metadataCtx)
	// Ensure the goroutine stops cleanly on shutdown.
	go func() {
		<-stopCh
		cancelMetadata()
		metadataSvc.Stop()
	}()

	// Run the main sync in a goroutine so the signal handler above can observe doneCh.
	go func() {
		defer close(doneCh)
		// Sync will now handle starting wallet count and contract reprocessing
		// services after initial sync is complete. stopCh is threaded in so
		// every background ticker goroutine Sync starts can observe shutdown.
		synchroniser.Sync(stopCh)
	}()

	// Block until either sync finishes naturally or a shutdown signal arrives.
	select {
	case <-doneCh:
		configs.Logger.Info("Sync completed, exiting normally")
	case <-stopCh:
		// Signal was received; the goroutine above will handle exit after doneCh closes.
		<-doneCh
	}
}
