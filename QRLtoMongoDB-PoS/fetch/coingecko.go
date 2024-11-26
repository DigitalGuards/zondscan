package fetch

import (
	"QRLtoMongoDB-PoS/configs"
	"QRLtoMongoDB-PoS/models"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

const (
	maxRetries    = 3
	retryInterval = 5 * time.Second
	timeout       = 10 * time.Second
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
			time.Sleep(retryInterval)
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
		zap.Float32("price", data.MarketData.CurrentPrice.USD))

	return &data, nil
}
