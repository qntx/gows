// Example demonstrating the use of On method for dynamic callback configuration
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
	client := coder.New(coder.Config{
		URL:       "wss://echo.websocket.org",
		Listening: true,
	})

	client.On(gows.EventConnect, func() {
		fmt.Println("🔗 Connected using On method callback!")
	})

	client.On(gows.EventClose, func() {
		fmt.Println("❌ Disconnected using On method callback!")
	})

	client.On(gows.EventMessage, func(msgType gows.MessageType, data []byte) {
		fmt.Printf("📨 On-method received [%v]: %s\n", msgType, string(data))
	})

	ctx := context.Background()
	if err := client.Connect(ctx); err != nil {
		log.Fatal("Failed to connect:", err)
	}
	defer client.Close()

	// Verify listening is active
	if client.IsListening() {
		fmt.Println("✅ Client is listening with dynamically configured callbacks")
	}

	// Test callback reconfiguration
	fmt.Println("\n🔄 Reconfiguring OnMessage callback...")
	client.On(gows.EventMessage, func(msgType gows.MessageType, data []byte) {
		fmt.Printf("🔄 UPDATED callback [%v]: %s\n", msgType, string(data))
	})

	// Send test messages
	messages := []string{
		"Message 1 - Initial callback",
		"Message 2 - Updated callback",
		"Message 3 - Updated callback",
	}

	for _, msg := range messages {
		if err := client.Write(ctx, gows.MessageText, []byte(msg)); err != nil {
			log.Printf("Failed to send message: %v", err)
			continue
		}

		time.Sleep(2 * time.Second)
	}

	// Wait for final responses
	time.Sleep(2 * time.Second)

	fmt.Println("\n✨ Dynamic callback configuration example completed!")
}
