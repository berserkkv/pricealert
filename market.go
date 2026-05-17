package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// FetchFuturesPrice returns the latest USDT-M futures price for a symbol.
func FetchFuturesPrice(symbol string) (float64, error) {
	url := fmt.Sprintf("https://fapi.binance.com/fapi/v1/ticker/price?symbol=%s", symbol)
	resp, err := http.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	var result struct {
		Price string `json:"price"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, err
	}

	var price float64
	_, err = fmt.Sscanf(result.Price, "%f", &price)
	return price, err
}
