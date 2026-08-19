package models

import "time"

// MarketTrade is one persisted public execution from a venue's trade tape.
//
// The document _id is "<venueId>:<tradeId>", which is what makes collection
// idempotent: the collector re-reads an overlapping window every poll and
// upserts, so a trade seen five times is stored once.
//
// Bucket is resolved at write time from the venue's size thresholds. A later
// threshold change therefore is not retroactive, which is deliberate: a
// historical chart must not silently redraw itself because a constant moved.
type MarketTrade struct {
	ID            string    `bson:"_id" json:"id"`
	Venue         string    `bson:"venue" json:"venue"`
	Symbol        string    `bson:"symbol" json:"symbol"`
	Price         float64   `bson:"price" json:"price"`
	Quantity      float64   `bson:"quantity" json:"quantity"`
	QuoteQuantity float64   `bson:"quoteQuantity" json:"quoteQuantity"`
	Time          int64     `bson:"time" json:"time"`
	At            time.Time `bson:"at" json:"at"`
	Side          string    `bson:"side" json:"side"`
	Bucket        string    `bson:"bucket" json:"bucket"`
}

// MarketFlowBucket is the buy/sell split for one size band over a window.
// Quantities are in the base asset (QRL); quote values are in the venue's
// quote asset (USDT for QRLUSDT).
type MarketFlowBucket struct {
	Bucket         string  `json:"bucket"`
	BuyQuantity    float64 `json:"buyQuantity"`
	SellQuantity   float64 `json:"sellQuantity"`
	NetQuantity    float64 `json:"netQuantity"`
	BuyQuote       float64 `json:"buyQuote"`
	SellQuote      float64 `json:"sellQuote"`
	NetQuote       float64 `json:"netQuote"`
	BuyTradeCount  int64   `json:"buyTradeCount"`
	SellTradeCount int64   `json:"sellTradeCount"`
}

// MarketFlowPoint is one step of a net-inflow time series. Time is the
// inclusive start of the step, as Unix milliseconds.
type MarketFlowPoint struct {
	Time         int64   `json:"time"`
	BuyQuantity  float64 `json:"buyQuantity"`
	SellQuantity float64 `json:"sellQuantity"`
	NetQuantity  float64 `json:"netQuantity"`
}

// MarketFlowCoverage describes how much history backs a response. The API
// cannot backfill: venues expose no historical trade tape, so a series can
// only accumulate forward from the first collection. Clients use this to
// distinguish "flat because flows balanced" from "flat because we were not
// collecting yet".
type MarketFlowCoverage struct {
	FirstTradeAt *int64 `json:"firstTradeAt"`
	LastTradeAt  *int64 `json:"lastTradeAt"`
	TradeCount   int64  `json:"tradeCount"`
	// Complete is true when collection started at or before the window's
	// start, so the window is fully covered by stored data.
	Complete bool `json:"complete"`
}
