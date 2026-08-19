package db

import (
	"backendAPI/configs"
	"backendAPI/marketdata"
	"backendAPI/models"
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Market fund-flow storage and rollups.
//
// Venues expose no historical trade tape (MEXC's aggTrades ignores a
// start/end range for this pair and returns null aggregate ids), so every
// series here is accumulated forward by the collector and can never be
// backfilled. Read paths therefore always report coverage alongside the
// numbers, and callers must not present an incomplete window as a full one.

// UpsertMarketTrades stores a tape slice idempotently and reports how many
// rows were new. The collector re-reads an overlapping window every poll, so
// all but the newest prints are already present; $setOnInsert keeps the
// first-written classification rather than rewriting unchanged rows.
func UpsertMarketTrades(ctx context.Context, trades []models.MarketTrade) (int, error) {
	if len(trades) == 0 {
		return 0, nil
	}
	if configs.MarketTradesCollection == nil {
		return 0, errors.New("market trades collection is not initialised")
	}

	writes := make([]mongo.WriteModel, 0, len(trades))
	for _, trade := range trades {
		// _id comes from the filter on upsert, so it is deliberately absent
		// from the update document: Mongo rejects an attempt to set it.
		doc := bson.M{
			"venue":         trade.Venue,
			"symbol":        trade.Symbol,
			"price":         trade.Price,
			"quantity":      trade.Quantity,
			"quoteQuantity": trade.QuoteQuantity,
			"time":          trade.Time,
			"at":            trade.At,
			"side":          trade.Side,
			"bucket":        trade.Bucket,
		}
		writes = append(writes, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": trade.ID}).
			SetUpdate(bson.M{"$setOnInsert": doc}).
			SetUpsert(true))
	}

	// Unordered so one rejected row cannot abort the rest of the batch.
	result, err := configs.MarketTradesCollection.BulkWrite(ctx, writes, options.BulkWrite().SetOrdered(false))
	if err != nil {
		// A bulk write can partially succeed. Report what landed so the
		// caller logs a truthful count instead of assuming zero.
		if result != nil {
			return int(result.UpsertedCount), fmt.Errorf("bulk upsert market trades: %w", err)
		}
		return 0, fmt.Errorf("bulk upsert market trades: %w", err)
	}
	return int(result.UpsertedCount), nil
}

func marketWindowFilter(venue string, from, to time.Time) bson.M {
	return bson.M{
		"venue": venue,
		"at":    bson.M{"$gte": from, "$lt": to},
	}
}

// signedQuantity projects a buy as positive and a sell as negative base
// quantity, which is the definition of net inflow used throughout.
func signedQuantity() bson.M {
	return bson.M{"$cond": bson.A{
		bson.M{"$eq": bson.A{"$side", "buy"}},
		"$quantity",
		bson.M{"$multiply": bson.A{"$quantity", -1}},
	}}
}

// MarketFlowBuckets returns the buy/sell split per size band over a window.
// Every known band is present in the result, zero-filled when it saw no
// trades, and ordered largest first so the caller renders a stable table.
func MarketFlowBuckets(ctx context.Context, venue string, from, to time.Time) ([]models.MarketFlowBucket, error) {
	if configs.MarketTradesCollection == nil {
		return nil, errors.New("market trades collection is not initialised")
	}
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: marketWindowFilter(venue, from, to)}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id":      bson.M{"bucket": "$bucket", "side": "$side"},
			"quantity": bson.M{"$sum": "$quantity"},
			"quote":    bson.M{"$sum": "$quoteQuantity"},
			"count":    bson.M{"$sum": 1},
		}}},
	}
	cursor, err := configs.MarketTradesCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate market flow buckets: %w", err)
	}
	defer cursor.Close(ctx)

	var rows []struct {
		ID struct {
			Bucket string `bson:"bucket"`
			Side   string `bson:"side"`
		} `bson:"_id"`
		Quantity float64 `bson:"quantity"`
		Quote    float64 `bson:"quote"`
		Count    int64   `bson:"count"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("decode market flow buckets: %w", err)
	}

	byBucket := make(map[string]*models.MarketFlowBucket, len(marketdata.OrderedSizeBuckets))
	out := make([]models.MarketFlowBucket, 0, len(marketdata.OrderedSizeBuckets))
	for _, bucket := range marketdata.OrderedSizeBuckets {
		out = append(out, models.MarketFlowBucket{Bucket: string(bucket)})
	}
	for i := range out {
		byBucket[out[i].Bucket] = &out[i]
	}

	for _, row := range rows {
		target, ok := byBucket[row.ID.Bucket]
		if !ok {
			// A row written by a build with more bands than this one knows.
			// Dropping it would understate the totals, so fold it into the
			// nearest band this build does have rather than lose the volume.
			target = byBucket[string(marketdata.BucketSmall)]
		}
		if row.ID.Side == "buy" {
			target.BuyQuantity += row.Quantity
			target.BuyQuote += row.Quote
			target.BuyTradeCount += row.Count
			continue
		}
		target.SellQuantity += row.Quantity
		target.SellQuote += row.Quote
		target.SellTradeCount += row.Count
	}
	for i := range out {
		out[i].NetQuantity = out[i].BuyQuantity - out[i].SellQuantity
		out[i].NetQuote = out[i].BuyQuote - out[i].SellQuote
	}
	return out, nil
}

// MarketFlowSeries buckets net inflow into fixed steps aligned to the
// window start, so the final step always ends at the window end rather than
// on an arbitrary calendar boundary. Empty steps are zero-filled, which is
// what makes a gap in collection visible as a flat run rather than as a
// gap the chart silently interpolates across.
func MarketFlowSeries(
	ctx context.Context,
	venue string,
	from, to time.Time,
	step time.Duration,
) ([]models.MarketFlowPoint, error) {
	if configs.MarketTradesCollection == nil {
		return nil, errors.New("market trades collection is not initialised")
	}
	if step <= 0 {
		return nil, errors.New("market flow series step must be positive")
	}
	fromMs := from.UnixMilli()
	stepMs := step.Milliseconds()
	steps := int((to.UnixMilli() - fromMs + stepMs - 1) / stepMs)
	if steps <= 0 {
		return []models.MarketFlowPoint{}, nil
	}

	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: marketWindowFilter(venue, from, to)}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id": bson.M{"$floor": bson.M{"$divide": bson.A{
				bson.M{"$subtract": bson.A{"$time", fromMs}},
				stepMs,
			}}},
			"buy": bson.M{"$sum": bson.M{"$cond": bson.A{
				bson.M{"$eq": bson.A{"$side", "buy"}}, "$quantity", 0,
			}}},
			"sell": bson.M{"$sum": bson.M{"$cond": bson.A{
				bson.M{"$eq": bson.A{"$side", "sell"}}, "$quantity", 0,
			}}},
		}}},
	}
	cursor, err := configs.MarketTradesCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate market flow series: %w", err)
	}
	defer cursor.Close(ctx)

	var rows []struct {
		Index float64 `bson:"_id"`
		Buy   float64 `bson:"buy"`
		Sell  float64 `bson:"sell"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("decode market flow series: %w", err)
	}

	points := make([]models.MarketFlowPoint, steps)
	for i := range points {
		points[i] = models.MarketFlowPoint{Time: fromMs + int64(i)*stepMs}
	}
	for _, row := range rows {
		index := int(row.Index)
		if index < 0 || index >= steps {
			continue
		}
		points[index].BuyQuantity = row.Buy
		points[index].SellQuantity = row.Sell
		points[index].NetQuantity = row.Buy - row.Sell
	}
	return points, nil
}

// MarketFlowDaily returns one net-inflow point per UTC day, oldest first.
// Days are calendar-aligned (not rolling) so the bars line up with the
// venue's own daily candles.
func MarketFlowDaily(ctx context.Context, venue string, from, to time.Time) ([]models.MarketFlowPoint, error) {
	if configs.MarketTradesCollection == nil {
		return nil, errors.New("market trades collection is not initialised")
	}
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: marketWindowFilter(venue, from, to)}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id": bson.M{"$dateTrunc": bson.M{"date": "$at", "unit": "day"}},
			"buy": bson.M{"$sum": bson.M{"$cond": bson.A{
				bson.M{"$eq": bson.A{"$side", "buy"}}, "$quantity", 0,
			}}},
			"sell": bson.M{"$sum": bson.M{"$cond": bson.A{
				bson.M{"$eq": bson.A{"$side", "sell"}}, "$quantity", 0,
			}}},
		}}},
		bson.D{{Key: "$sort", Value: bson.D{{Key: "_id", Value: 1}}}},
	}
	cursor, err := configs.MarketTradesCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("aggregate market flow daily: %w", err)
	}
	defer cursor.Close(ctx)

	var rows []struct {
		Day  time.Time `bson:"_id"`
		Buy  float64   `bson:"buy"`
		Sell float64   `bson:"sell"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, fmt.Errorf("decode market flow daily: %w", err)
	}

	byDay := make(map[int64]models.MarketFlowPoint, len(rows))
	for _, row := range rows {
		day := row.Day.UTC().UnixMilli()
		byDay[day] = models.MarketFlowPoint{
			Time:         day,
			BuyQuantity:  row.Buy,
			SellQuantity: row.Sell,
			NetQuantity:  row.Buy - row.Sell,
		}
	}

	// Zero-fill every day in range so a silent day renders as a flat bar
	// rather than shifting later bars leftwards.
	start := from.UTC().Truncate(24 * time.Hour)
	out := make([]models.MarketFlowPoint, 0, 8)
	for day := start; day.Before(to.UTC()); day = day.Add(24 * time.Hour) {
		key := day.UnixMilli()
		if point, ok := byDay[key]; ok {
			out = append(out, point)
			continue
		}
		out = append(out, models.MarketFlowPoint{Time: key})
	}
	return out, nil
}

// MarketFlowCoverage reports the stored extent for a venue so a response can
// state honestly how much of the requested window is actually backed by
// data. windowStart decides the Complete flag.
func MarketFlowCoverage(ctx context.Context, venue string, windowStart time.Time) (models.MarketFlowCoverage, error) {
	var coverage models.MarketFlowCoverage
	if configs.MarketTradesCollection == nil {
		return coverage, errors.New("market trades collection is not initialised")
	}
	pipeline := mongo.Pipeline{
		bson.D{{Key: "$match", Value: bson.M{"venue": venue}}},
		bson.D{{Key: "$group", Value: bson.M{
			"_id":   nil,
			"first": bson.M{"$min": "$time"},
			"last":  bson.M{"$max": "$time"},
			"count": bson.M{"$sum": 1},
		}}},
	}
	cursor, err := configs.MarketTradesCollection.Aggregate(ctx, pipeline)
	if err != nil {
		return coverage, fmt.Errorf("aggregate market flow coverage: %w", err)
	}
	defer cursor.Close(ctx)

	var rows []struct {
		First int64 `bson:"first"`
		Last  int64 `bson:"last"`
		Count int64 `bson:"count"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return coverage, fmt.Errorf("decode market flow coverage: %w", err)
	}
	if len(rows) == 0 || rows[0].Count == 0 {
		return coverage, nil
	}

	first, last := rows[0].First, rows[0].Last
	coverage.FirstTradeAt = &first
	coverage.LastTradeAt = &last
	coverage.TradeCount = rows[0].Count
	coverage.Complete = first <= windowStart.UnixMilli()
	return coverage, nil
}
