// 	
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
		URL: "wss://echo.websocket.org",
		OnConnect: func() {
			fmt.Println("Connected to WebSocket server!")
		},
		OnClose: func() {
			fmt.Println("Disconnected from WebSocket server!")
		},
		OnMessage: func(msgType gows.MessageType, data []byte) {
			fmt.Printf("Received message [%v]: %s\n", msgType, string(data))
		},
	}

	client := coder.New(cfg)

	// Connect to the server
	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		log.Fatal("Failed to connect:", err)
	}
	defer client.Close()

	// Start listening for messages in background
	if err := client.StartListening(); err != nil {
		log.Fatal("Failed to start listening:", err)
	}

	// Send some test messages
	messages := []string{
		"Hello WebSocket!",
		"This is a test message",
		"Message from Go client",
	}

	for _, msg := range messages {
		if err := client.Write(ctx, gows.MessageText, []byte(msg)); err != nil {
			log.Printf("Failed to send message: %v", err)
			continue
		}
		time.Sleep(1 * time.Second)
	}

	// Wait a bit to receive responses
	time.Sleep(3 * time.Second)

	// Stop listening (optional, will be stopped automatically on Close)
	client.StopListening()

	fmt.Println("Example completed!")
}
