package db

import (
	"backendAPI/configs"
	"context"
	"math/big"
	"sort"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Reusable big.Int constants for hot paths (median calcs, ceil-divisions).
// big.Int math is allocation-heavy; sharing read-only operands shaves
// thousands of allocations per request on a busy mempool.
var (
	bigOne = big.NewInt(1)
	bigTwo = big.NewInt(2)
	// 1 Shor = 10^9 Planck (the QRL-native pair of units, equivalent in
	// magnitude to Ethereum's Gwei/wei).
	bigShor = big.NewInt(1_000_000_000)
)

// hexToBig parses a "0x"-prefixed hex string into a *big.Int. Empty / malformed
// input yields 0 — gas math should never panic on a missing field.
func hexToBig(s string) *big.Int {
	b := new(big.Int)
	if s == "" {
		return b
	}
	s = strings.TrimPrefix(s, "0x")
	if s == "" {
		return b
	}
	b.SetString(s, 16)
	return b
}

// BlockGasSample is the minimal projection we need from each block for the gas
// page and ETA math. Block documents are stored with lowercase BSON keys
// (gasused, gaslimit, basefeepergas, gasprice on tx) — match that exactly.
type BlockGasSample struct {
	BlockNumberInt int64 `bson:"blockNumberInt"`
	Result         struct {
		Number        string `bson:"number"`
		Timestamp     string `bson:"timestamp"`
		GasUsed       string `bson:"gasused"`
		GasLimit      string `bson:"gaslimit"`
		BaseFeePerGas string `bson:"basefeepergas"`
		Transactions  []struct {
			Hash     string `bson:"hash"`
			GasPrice string `bson:"gasprice"`
		} `bson:"transactions"`
	} `bson:"result"`
}

// GetRecentBlockSamples returns the last n blocks (newest first) projected to
// just the gas/timing fields. Used by both the ETA endpoint and the /gas
// summary endpoint.
func GetRecentBlockSamples(n int) ([]BlockGasSample, error) {
	if n <= 0 {
		n = 30
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := options.Find().
		SetProjection(primitive.D{
			{Key: "blockNumberInt", Value: 1},
			{Key: "result.number", Value: 1},
			{Key: "result.timestamp", Value: 1},
			{Key: "result.gasused", Value: 1},
			{Key: "result.gaslimit", Value: 1},
			{Key: "result.basefeepergas", Value: 1},
			{Key: "result.transactions.hash", Value: 1},
			{Key: "result.transactions.gasprice", Value: 1},
		}).
		SetSort(primitive.D{{Key: "blockNumberInt", Value: -1}}).
		SetLimit(int64(n))

	cur, err := configs.BlocksCollection.Find(ctx, primitive.D{}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var out []BlockGasSample
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// BlockStats aggregates a set of recent block samples for the /gas summary
// and the ETA computation. All "avg" values are simple arithmetic means over
// the provided window.
type BlockStats struct {
	AvgBlockTimeSec    float64
	AvgGasUsed         *big.Int
	AvgGasLimit        *big.Int
	LastNumberHex      string
	LastGasUsedHex     string
	LastGasLimitHex    string
	// MedianTxGasPrice is the median of every `gasprice` across every tx in
	// the sample window. Zero when no block in the window contained txs.
	MedianTxGasPrice *big.Int
	TxCount          int
}

func ComputeBlockStats(samples []BlockGasSample) BlockStats {
	stats := BlockStats{
		AvgBlockTimeSec:  60,
		AvgGasUsed:       big.NewInt(0),
		AvgGasLimit:      big.NewInt(0),
		MedianTxGasPrice: big.NewInt(0),
	}
	if len(samples) == 0 {
		return stats
	}
	stats.LastNumberHex = samples[0].Result.Number
	stats.LastGasUsedHex = samples[0].Result.GasUsed
	stats.LastGasLimitHex = samples[0].Result.GasLimit

	sumUsed := new(big.Int)
	sumLimit := new(big.Int)
	// Conservative pre-allocation: ~10 txs/block × N blocks. Avoids the
	// 3-4 grow-and-copy passes the slice would otherwise do for a typical
	// 30-block window on a busy chain.
	txPrices := make([]*big.Int, 0, len(samples)*10)
	for _, s := range samples {
		sumUsed.Add(sumUsed, hexToBig(s.Result.GasUsed))
		sumLimit.Add(sumLimit, hexToBig(s.Result.GasLimit))
		for _, t := range s.Result.Transactions {
			if t.GasPrice == "" {
				continue
			}
			txPrices = append(txPrices, hexToBig(t.GasPrice))
		}
	}
	stats.TxCount = len(txPrices)
	if len(txPrices) > 0 {
		sort.Slice(txPrices, func(i, j int) bool { return txPrices[i].Cmp(txPrices[j]) < 0 })
		mid := len(txPrices) / 2
		if len(txPrices)%2 == 1 {
			stats.MedianTxGasPrice.Set(txPrices[mid])
		} else {
			sum := new(big.Int).Add(txPrices[mid-1], txPrices[mid])
			stats.MedianTxGasPrice.Quo(sum, bigTwo)
		}
	}
	n := big.NewInt(int64(len(samples)))
	stats.AvgGasUsed.Quo(sumUsed, n)
	stats.AvgGasLimit.Quo(sumLimit, n)

	if len(samples) >= 2 {
		newest := hexToBig(samples[0].Result.Timestamp).Int64()
		oldest := hexToBig(samples[len(samples)-1].Result.Timestamp).Int64()
		span := newest - oldest
		gaps := int64(len(samples) - 1)
		if span > 0 && gaps > 0 {
			stats.AvgBlockTimeSec = float64(span) / float64(gaps)
		}
	}
	return stats
}

// MempoolSample is the minimal projection we need from each pending tx.
type MempoolSample struct {
	Hash     string `bson:"_id"`
	Gas      string `bson:"gas"`
	GasPrice string `bson:"gasPrice"`
}

// GetPendingMempoolSnapshot returns every pending (status="pending") tx with
// just the hash + gas + gasPrice fields. The mempool is small enough on QRL
// today that pulling the full set and doing big-int math in Go is simpler and
// more correct than trying to compare hex-encoded big ints inside Mongo.
func GetPendingMempoolSnapshot() ([]MempoolSample, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	col := configs.GetCollection(configs.DB, PENDING_COLLECTION)
	opts := options.Find().SetProjection(primitive.D{
		{Key: "gas", Value: 1},
		{Key: "gasPrice", Value: 1},
	})
	cur, err := col.Find(ctx, bson.M{"status": "pending"}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var out []MempoolSample
	if err := cur.All(ctx, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// MempoolStats summarizes a mempool snapshot relative to a single tx.
type MempoolStats struct {
	Count           int
	MedianGasPrice  *big.Int
	GasAhead        *big.Int // sum of `gas` for txs with strictly higher gasPrice than the target
	YourGasPrice    *big.Int
}

// ComputeMempoolStatsFor walks the snapshot once, computing the median
// gasPrice across the whole set and the gas-ahead total relative to
// `targetGasPriceHex` (the tx the ETA is being computed for).
func ComputeMempoolStatsFor(samples []MempoolSample, targetGasPriceHex string) MempoolStats {
	stats := MempoolStats{
		Count:          len(samples),
		MedianGasPrice: big.NewInt(0),
		GasAhead:       big.NewInt(0),
		YourGasPrice:   hexToBig(targetGasPriceHex),
	}
	if len(samples) == 0 {
		return stats
	}

	prices := make([]*big.Int, 0, len(samples))
	for _, s := range samples {
		gp := hexToBig(s.GasPrice)
		prices = append(prices, gp)
		if gp.Cmp(stats.YourGasPrice) > 0 {
			stats.GasAhead.Add(stats.GasAhead, hexToBig(s.Gas))
		}
	}
	sort.Slice(prices, func(i, j int) bool { return prices[i].Cmp(prices[j]) < 0 })
	mid := len(prices) / 2
	if len(prices)%2 == 1 {
		stats.MedianGasPrice.Set(prices[mid])
	} else {
		sum := new(big.Int).Add(prices[mid-1], prices[mid])
		stats.MedianGasPrice.Quo(sum, bigTwo)
	}
	return stats
}

// GasPriceBucket is one bin in the mempool gas-price histogram surfaced on the
// /gas page. Bounds are reported in Shor (QRL-native; 1 Shor = 10^9 Planck).
type GasPriceBucket struct {
	ShorLow  float64 `json:"shorLow"`
	ShorHigh float64 `json:"shorHigh"`
	Count    int     `json:"count"`
}

// MempoolGasPriceHistogram bins the snapshot into up to `bins` evenly-spaced
// Shor buckets between the min and max gasPrice. Returns a single-bin result
// if all txs share the same price (or the mempool is empty).
func MempoolGasPriceHistogram(samples []MempoolSample, bins int) []GasPriceBucket {
	if bins <= 0 {
		bins = 10
	}
	if len(samples) == 0 {
		return []GasPriceBucket{}
	}
	values := make([]float64, 0, len(samples))
	for _, s := range samples {
		gp := hexToBig(s.GasPrice)
		// Shor = Planck / 10^9. float64 lossiness is fine here — this is for
		// a histogram, not accounting.
		q := new(big.Int).Quo(gp, bigShor)
		values = append(values, float64(q.Int64()))
	}
	sort.Float64s(values)
	minV, maxV := values[0], values[len(values)-1]
	if maxV == minV {
		return []GasPriceBucket{{ShorLow: minV, ShorHigh: maxV, Count: len(values)}}
	}
	width := (maxV - minV) / float64(bins)
	out := make([]GasPriceBucket, bins)
	for i := 0; i < bins; i++ {
		out[i] = GasPriceBucket{
			ShorLow:  minV + width*float64(i),
			ShorHigh: minV + width*float64(i+1),
		}
	}
	for _, v := range values {
		idx := int((v - minV) / width)
		if idx >= bins {
			idx = bins - 1
		}
		out[idx].Count++
	}
	return out
}

// ---- /gas/history ---------------------------------------------------------
//
// The syncer maintains a `gasHistory` collection with one row per block. The
// API serves it as-is for short ranges (24h ≈ 1440 rows at 60 s slots) and
// downsamples to one row per hour for the 7d view.

const GAS_HISTORY_COLLECTION = "gasHistory"

type GasHistoryRow struct {
	BlockNumberInt int64  `bson:"blockNumberInt" json:"blockNumber"`
	Timestamp      int64  `bson:"timestamp"      json:"timestamp"`
	GasUsed        string `bson:"gasUsed"        json:"gasUsed"`
	GasLimit       string `bson:"gasLimit"       json:"gasLimit"`
	BaseFeePerGas  string `bson:"baseFeePerGas"  json:"baseFeePerGas"`
	TxCount        int    `bson:"txCount"        json:"txCount"`
}

// GetGasHistory returns gas-history rows newer than `since` (Unix seconds),
// sorted oldest→newest. When `bucketSec > 0` the rows are downsampled to one
// row per bucket window using the latest row inside each bucket.
func GetGasHistory(sinceUnix int64, bucketSec int64) ([]GasHistoryRow, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	col := configs.GetCollection(configs.DB, GAS_HISTORY_COLLECTION)
	opts := options.Find().SetSort(primitive.D{{Key: "timestamp", Value: 1}})
	cur, err := col.Find(ctx, bson.M{"timestamp": bson.M{"$gte": sinceUnix}}, opts)
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	var rows []GasHistoryRow
	if err := cur.All(ctx, &rows); err != nil {
		return nil, err
	}

	if bucketSec <= 0 || len(rows) == 0 {
		return rows, nil
	}

	// Downsample: keep the latest row per bucket window.
	byBucket := make(map[int64]GasHistoryRow)
	var order []int64
	for _, r := range rows {
		b := (r.Timestamp / bucketSec) * bucketSec
		if existing, ok := byBucket[b]; !ok || r.BlockNumberInt > existing.BlockNumberInt {
			if _, seen := byBucket[b]; !seen {
				order = append(order, b)
			}
			byBucket[b] = r
		}
	}
	out := make([]GasHistoryRow, 0, len(order))
	sort.Slice(order, func(i, j int) bool { return order[i] < order[j] })
	for _, b := range order {
		out = append(out, byBucket[b])
	}
	return out, nil
}
