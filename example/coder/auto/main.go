package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/qntx/gows"
	"github.com/qntx/gows/coder"
)

type UserMessage struct {
	User    string `json:"user"`
	Content string `json:"content"`
}

func main() {
	msgChan := make(chan UserMessage, 10)
	errChan := make(chan error, 10)

	client := coder.New(coder.Config{
		URL:       "wss://echo.websocket.org",
		Listening: true,
		OnConnect: func() {
			fmt.Println("🔗 Connected to wss://echo.websocket.org!")
		},
	})

	client.On(gows.EventMessage, func(msgType gows.MessageType, data []byte) {
		fmt.Printf("Received raw data: %s\n", string(data))

		if msgType != gows.MessageText {
			return
		}

		var msg UserMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			errChan <- err
			return
		}

		msgChan <- msg
	})

	if err := client.Connect(context.Background()); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	go func() {
		time.Sleep(2 * time.Second)

		msgToSend := UserMessage{
			User:    "Gemini",
			Content: "Hello, this is a test message!",
		}

		jsonData, err := json.Marshal(msgToSend)
		if err != nil {
			return
		}

		fmt.Printf("🚀 Sending JSON data: %s\n", string(jsonData))
		if err := client.Write(context.Background(), gows.MessageText, jsonData); err != nil {
			log.Printf("Error writing message: %v", err)
		}
	}()

	go func() {
		fmt.Println("--- Starting event processing loop. Waiting for messages... ---")
		for {
			select {
			case msg := <-msgChan:
				fmt.Printf("✅ [APP LOGIC] Successfully processed message from user '%s' with content: '%s'\n", msg.User, msg.Content)
			case err := <-errChan:
				fmt.Printf("🚨 [APP LOGIC] An error occurred: %v\n", err)
			case <-time.After(10 * time.Second):
				fmt.Println("Timeout: No message received after 10 seconds.")
			}
		}
	}()

	time.Sleep(5 * time.Second)
	fmt.Println("--- Example finished. ---")
}
