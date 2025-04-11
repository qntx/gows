package main

import (
	"fmt"
	"os"
	"time"

	"github.com/qntx/gonet/websocket"
)

const (
	wsURL         = "wss://fstream.binance.com/ws/btcusdt@aggTrade"
	proxyURL      = "http://127.0.0.1:7890"
	timeout       = 30 * time.Second
	maxRetries    = 3
	retryMinDelay = 1 * time.Second
	retryMaxDelay = 30 * time.Second
	msgBufferSize = 1000
	errBufferSize = 1000
)

func main() {
	// Initialize and configure WebSocket client
	client, err := websocket.New(
		wsURL,
		websocket.WithTimeout(timeout),
		websocket.WithProxy(proxyURL),
		websocket.WithRetries(maxRetries, retryMinDelay, retryMaxDelay),
		websocket.WithMessages(msgBufferSize),
		websocket.WithErrors(errBufferSize),
		websocket.WithAsync(true),
		websocket.OnConnect(func(c *websocket.Client) { fmt.Println("Connected to Binance WebSocket") }),
		websocket.OnClose(func(code int, text string, c *websocket.Client) {
			fmt.Println("Connection closed", "code", code, "reason", text)
		}),
		websocket.WithDebug(true),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create WebSocket client: %v\n", err)
		os.Exit(1)
	}

	// Connect to WebSocket
	if err := client.Connect(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	// Handle messages and errors concurrently
	go handleMessages(client)
	go handleErrors(client)

	// Wait for shutdown signal and cleanup
	<-client.Context().Done()
}

func handleMessages(client *websocket.Client) {
	if !client.HasMessages() {
		return
	}

	msgCh := client.Messages()
	for {
		select {
		case msg, ok := <-msgCh:
			if !ok {
				return
			}
			fmt.Printf("Received message: %s\n", msg)
		case <-client.Context().Done():
			return
		}
	}
}

func handleErrors(client *websocket.Client) {
	if !client.HasErrors() {
		return
	}

	errCh := client.Errors()
	for {
		select {
		case err, ok := <-errCh:
			if !ok {
				return
			}
			fmt.Fprintf(os.Stderr, "Error occurred: %v\n", err)
		case <-client.Context().Done():
			return
		}
	}
}
