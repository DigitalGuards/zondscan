package main

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/synchroniser"
	"context"
	"net/http"
	"os"
	"os/signal"
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
	configs.Logger.Info("Node URL: " + os.Getenv("NODE_URL"))

	// Start health check server for Kubernetes probes
	go func() {
		http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
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

	// Start pending transaction sync (this is not started in sync.go)
	configs.Logger.Info("Starting pending transaction sync service...")
	synchroniser.StartPendingTransactionSync()

	// Run the main sync in a goroutine so the signal handler above can observe doneCh.
	go func() {
		defer close(doneCh)
		// Sync will now handle starting wallet count and contract reprocessing
		// services after initial sync is complete
		synchroniser.Sync()
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
