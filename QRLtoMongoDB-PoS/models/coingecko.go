package models

type MarketDataResponse struct {
	MarketData struct {
		MarketCap struct {
			USD float32 `json:"usd"`
		} `json:"market_cap"`
	} `json:"market_data"`
}
