// Example demonstrating the use of AutoListening configuration
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/qntx/gows"
	"github.com/qntx/gows/coder"
)

func main() {
	cfg := coder.Config{
		URL:           "wss://echo.websocket.org",
		AutoListening: true, // Automatically start listening after connection
		OnConnect: func() {
			fmt.Println("Connected and automatically started listening!")
		},
		OnClose: func() {
			fmt.Println("Disconnected from WebSocket server!")
		},
		OnMessage: func(msgType gows.MessageType, data []byte) {
			fmt.Printf("Auto-received message [%v]: %s\n", msgType, string(data))
		},
	}

	client := coder.New(cfg)

	// Connect to the server - listening will start automatically
	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		log.Fatal("Failed to connect:", err)
	}
	defer client.Close()

	// Verify listening is active
	if client.IsListening() {
		fmt.Println("✅ Client is automatically listening for messages")
	} else {
		fmt.Println("❌ Client is not listening")
	}

	// Send some test messages
	messages := []string{
		"Auto listening test message 1",
		"Auto listening test message 2",
		"Auto listening test message 3",
	}

	for _, msg := range messages {
		if err := client.Write(ctx, gows.MessageText, []byte(msg)); err != nil {
			log.Printf("Failed to send message: %v", err)
			continue
		}
		time.Sleep(1 * time.Second)
	}

	// Wait for responses
	time.Sleep(2 * time.Second)

	fmt.Println("Auto-start listening example completed!")
}
