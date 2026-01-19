package fetch

import (
	"Zond2mongoDB/configs"
	"Zond2mongoDB/models"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

const (
	maxRetries = 3
	baseDelay  = 30 * time.Second // Increased base delay
	maxDelay   = 5 * time.Minute  // Maximum delay between retries
	timeout    = 10 * time.Second
)

func FetchCoinGeckoData() (*models.MarketDataResponse, error) {
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		data, err := fetchWithTimeout()
		if err == nil {
			return data, nil
		}

		lastErr = err
		configs.Logger.Warn("Failed to fetch CoinGecko data",
			zap.Error(err),
			zap.Int("attempt", attempt),
			zap.Int("maxRetries", maxRetries))

		if attempt < maxRetries {
			// Exponential backoff with jitter
			delay := time.Duration(float64(baseDelay) * float64(attempt) * (1 + 0.2*(float64(time.Now().UnixNano()%100)/100.0)))
			if delay > maxDelay {
				delay = maxDelay
			}

			configs.Logger.Info("Waiting before next retry",
				zap.Duration("delay", delay),
				zap.Int("attempt", attempt))

			time.Sleep(delay)
		}
	}

	return nil, fmt.Errorf("failed after %d attempts, last error: %v", maxRetries, lastErr)
}

func fetchWithTimeout() (*models.MarketDataResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", configs.COINGECKO_URL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("rate limit exceeded (429), please wait before retrying")
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var data models.MarketDataResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	configs.Logger.Info("Successfully fetched CoinGecko data",
		zap.Float32("marketCap", data.MarketData.MarketCap.USD),
		zap.Float32("price", data.MarketData.CurrentPrice.USD),
		zap.Float32("volume", data.MarketData.TotalVolume.USD))

	return &data, nil
}
