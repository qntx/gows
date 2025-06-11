package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/qntx/gows/coder"
)

type BinanceTrade struct {
	EventType string `json:"e"`
	EventTime int64  `json:"E"`
	Symbol    string `json:"s"`
	TradeID   int64  `json:"t"`
	Price     string `json:"p"`
	Quantity  string `json:"q"`
	TradeTime int64  `json:"T"`
	IsMaker   bool   `json:"m"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	binanceURL := "wss://stream.binance.com:9443/ws/btcusdt@trade"

	client := coder.New(coder.Config{
		Context:   ctx,
		URL:       binanceURL,
		Heartbeat: 3 * time.Minute,
		ReadLimit: 1024 * 1024,
	})

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := client.Connect(connectCtx); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	log.Println("Successfully connected to Binance trade stream.")

	go func() {
		for {
			var trade BinanceTrade
			if _, _, err := client.Read(ctx, &trade); err != nil {
				if ctx.Err() == context.Canceled {
					log.Println("ReadJSON loop canceled, shutting down.")
				} else {
					log.Printf("Error reading or decoding JSON: %v", err)
				}
				return
			}

			log.Printf(
				"New Trade on %s: Price=%s, Quantity=%s, Time=%s",
				trade.Symbol,
				trade.Price,
				trade.Quantity,
				time.UnixMilli(trade.TradeTime).Format(time.RFC3339),
			)
		}
	}()

	log.Println("Application started. Press Ctrl+C to exit.")
	<-ctx.Done()

	log.Println("Shutting down gracefully...")
	if err := client.Close(); err != nil {
		log.Printf("Error closing websocket connection: %v", err)
	}
	log.Println("Connection closed.")
}
