package main

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/db"
	L "Zond2mongoDB/logger"
	"Zond2mongoDB/synchroniser"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

func main() {
	logger := L.FileLogger(configs.Filename)
	zap.ReplaceGlobals(logger)
	defer logger.Sync()

	logger.Info("Initializing QRL to MongoDB synchronizer...")
	logger.Info("Connecting to MongoDB and RPC node...")

	// Create a buffered channel to avoid signal notification drops
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		logger.Info("Gracefully shutting down synchronizer...")
		logger.Info("Stopped syncing")
		os.Exit(1)
	}()

	logger.Info("Starting blockchain synchronization process...")
	logger.Info("MongoDB URL: " + os.Getenv("MONGOURI"))
	logger.Info("Node URL: " + os.Getenv("NODE_URL"))

	// Start wallet count sync
	logger.Info("Starting wallet count sync service...")
	db.StartWalletCountSync()

	// Start pending transaction sync
	logger.Info("Starting pending transaction sync service...")
	synchroniser.StartPendingTransactionSync()

	// Start blockchain sync
	synchroniser.Sync()
}
