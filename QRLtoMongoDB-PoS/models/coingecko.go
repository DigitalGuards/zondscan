package models

type MarketDataResponse struct {
	MarketData struct {
		MarketCap struct {
			USD float32 `json:"usd"`
		} `json:"market_cap"`
		CurrentPrice struct {
			USD float32 `json:"usd"`
		} `json:"current_price"`
	} `json:"market_data"`
}
