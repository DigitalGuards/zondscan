package handler

import (
	"backendAPI/collector"
	"backendAPI/marketdata"
	"context"
	"log"
	"os"
	"strconv"
	"time"
)

// startMarketCollector launches the background venue tape collector.
//
// Env controls:
//
//	MARKET_COLLECT_ENABLED   "false" disables collection entirely
//	MARKET_COLLECT_INTERVAL  poll period, Go duration (default 1m)
//
// Running two backends against one database is safe: rows are upserted by a
// venue-namespaced id, so a duplicated collector costs redundant requests
// and stores nothing twice.
func startMarketCollector() {
	if value := os.Getenv("MARKET_COLLECT_ENABLED"); value != "" {
		enabled, err := strconv.ParseBool(value)
		if err != nil {
			log.Printf("Market collector: ignoring unparseable MARKET_COLLECT_ENABLED %q: %v", value, err)
		} else if !enabled {
			log.Println("Market collector disabled by MARKET_COLLECT_ENABLED")
			return
		}
	}

	registry, err := marketdata.DefaultRegistry()
	if err != nil {
		log.Printf("Market collector disabled: %v", err)
		return
	}

	interval := collector.DefaultInterval
	if value := os.Getenv("MARKET_COLLECT_INTERVAL"); value != "" {
		parsed, parseErr := time.ParseDuration(value)
		if parseErr != nil || parsed <= 0 {
			log.Printf("Market collector: ignoring invalid MARKET_COLLECT_INTERVAL %q", value)
		} else {
			interval = parsed
		}
	}

	// The process runs until the platform kills it, and pm2 restarts it on
	// exit, so the collector's lifetime is the process lifetime.
	go collector.NewMarketTradeCollector(registry, interval, collector.DefaultTimeout).
		Run(context.Background())
}
