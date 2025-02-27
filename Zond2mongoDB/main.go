package main

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/synchroniser"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// Ensure logger resources are properly released
	defer configs.Logger.Sync()

	configs.Logger.Info("Initializing QRL to MongoDB synchronizer...")
	configs.Logger.Info("Connecting to MongoDB and RPC node...")

	// Create a buffered channel to avoid signal notification drops
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		configs.Logger.Info("Gracefully shutting down synchronizer...")
		configs.Logger.Info("Stopped syncing")
		os.Exit(1)
	}()

	configs.Logger.Info("Starting blockchain synchronization process...")
	configs.Logger.Info("MongoDB URL: " + os.Getenv("MONGOURI"))
	configs.Logger.Info("Node URL: " + os.Getenv("NODE_URL"))

	// Start pending transaction sync (this is not started in sync.go)
	configs.Logger.Info("Starting pending transaction sync service...")
	synchroniser.StartPendingTransactionSync()
	// Sync will now handle starting wallet count and contract reprocessing
	// services after initial sync is complete
	synchroniser.Sync()
}
