package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
	"time"

	"github.com/qntx/gows"
	"github.com/qntx/gows/coder"
)

type Trade struct {
	EventType string `json:"e"`
	EventTime int64  `json:"E"`
	Symbol    string `json:"s"`
	TradeID   int64  `json:"t"`
	Price     string `json:"p"`
	Quantity  string `json:"q"`
	TradeTime int64  `json:"T"`
	IsMaker   bool   `json:"m"`
	Ignore    bool   `json:"M"`
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	client := coder.New(&coder.Config{
		Context: ctx,
		URL:     "wss://stream.binance.com:9443/ws/btcusdt@trade",
		// Heartbeat: time.Second * 15,
		ReadLimit: 1024 * 1024,
	})

	c, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := client.Connect(c); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	go handleMessages(ctx, client)

	<-ctx.Done()
	log.Println("Shutting down...")
}

func handleMessages(ctx context.Context, client *coder.Client) {
	for {
		var trade Trade

		typ, p, err := client.Read(ctx, &trade)
		if err != nil {
			if ctx.Err() == context.Canceled {
				log.Println("Read loop canceled, shutting down.")
			} else {
				log.Printf("Error reading or decoding JSON: %v", err)
			}

			return
		}

		if typ != gows.MessageText {
			log.Printf("Received non-text message: %v", typ)

			continue
		}

		log.Printf("Raw message: %s", p)

		log.Printf(
			"New Trade on %s: Price=%s, Quantity=%s, IsMaker=%t",
			trade.Symbol,
			trade.Price,
			trade.Quantity,
			trade.IsMaker,
		)
	}
}
