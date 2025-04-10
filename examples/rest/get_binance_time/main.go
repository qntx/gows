package main

import (
	"fmt"
	"time"

	"github.com/qntx/gonet/rest"
)

// BinanceTime represents the Binance API time endpoint response.
type BinanceTime struct {
	ServerTime int64 `json:"serverTime"`
}

// BinanceErrorResponse represents a Binance API error response.
type BinanceErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
}

func main() {
	// Create client with options
	client, err := rest.New(
		"https://api.binance.com",
		rest.WithProxy("http://127.0.0.1:7890"),
		rest.WithDebug(true),
	)
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		return
	}

	// Define response structures
	var result BinanceTime
	var errorResp BinanceErrorResponse

	// Create and configure request
	req, err := client.R(
		rest.WithResult(&result),
		rest.WithError(&errorResp),
	)
	if err != nil {
		fmt.Printf("Failed to create request: %v\n", err)
		return
	}

	// Execute request
	resp, err := req.Get("/api/v3/time")
	if err != nil {
		fmt.Printf("Failed to get server time: %v\n", err)
		return
	}

	// Log raw response
	fmt.Printf("Raw response: %s\n", resp.String())

	// Handle response
	if !resp.IsSuccess() {
		fmt.Printf("Request failed with status: %s\n", resp.Status())
		if errorResp.Code != 0 {
			fmt.Printf("Binance API error: %d - %s\n", errorResp.Code, errorResp.Message)
		}
		return
	}

	// Process successful response
	serverTime := time.UnixMilli(result.ServerTime)
	now := time.Now()
	fmt.Printf("Binance server time: %v\n", serverTime)
	fmt.Printf("Local time: %v\n", now)
	fmt.Printf("Time difference: %v\n", now.Sub(serverTime))
}
