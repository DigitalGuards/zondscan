package main

import (
	"QRLtoMongoDB-PoS/configs"
	L "QRLtoMongoDB-PoS/logger"
	"QRLtoMongoDB-PoS/synchroniser"
	"os"
	"os/signal"
	"syscall"

	"go.uber.org/zap"
)

func main() {
	var logger *zap.Logger = L.FileLogger(configs.Filename)

	logger.Info("Initializing QRL to MongoDB synchronizer...")
	logger.Info("Connecting to MongoDB and RPC node...")

	c := make(chan os.Signal)
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
	synchroniser.Sync()
}
