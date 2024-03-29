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

	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		logger.Info("Stopped syncing")
		os.Exit(1)
	}()

	logger.Info("Started syncing")
	synchroniser.Sync()
}
