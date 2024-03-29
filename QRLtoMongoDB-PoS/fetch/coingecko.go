package fetch

import (
	"QRLtoMongoDB-PoS/configs"
	"QRLtoMongoDB-PoS/models"
	"encoding/json"
	"fmt"
	"net/http"
)

func FetchCoinGeckoData() (*models.MarketDataResponse, error) {
	resp, err := http.Get(configs.COINGECKO_URL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data models.MarketDataResponse
	err = json.NewDecoder(resp.Body).Decode(&data)
	if err != nil {
		return nil, err
	}

	fmt.Println(data)

	return &data, nil
}
