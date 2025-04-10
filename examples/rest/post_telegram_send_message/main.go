package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/qntx/gonet/rest"
)

// TelegramSuccessResponse represents a successful Telegram API response.
type TelegramSuccessResponse struct {
	Ok     bool `json:"ok"`
	Result struct {
		MessageID int `json:"message_id"`
		From      struct {
			ID        int64  `json:"id"`
			IsBot     bool   `json:"is_bot"`
			FirstName string `json:"first_name"`
			Username  string `json:"username,omitempty"`
		} `json:"from"`
		Chat struct {
			ID        int64  `json:"id"`
			FirstName string `json:"first_name"`
			LastName  string `json:"last_name,omitempty"`
			Username  string `json:"username,omitempty"`
			Type      string `json:"type"`
		} `json:"chat"`
		Date     int64  `json:"date"`
		Text     string `json:"text"`
		Entities []struct {
			Offset int    `json:"offset"`
			Length int    `json:"length"`
			Type   string `json:"type"`
		} `json:"entities,omitempty"`
	} `json:"result"`
}

// TelegramErrorResponse represents a Telegram API error response.
type TelegramErrorResponse struct {
	Ok          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		fmt.Printf("Error loading .env: %v\n", err)
		return
	}

	botToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")
	if botToken == "" || chatID == "" {
		fmt.Println("TELEGRAM_BOT_TOKEN or TELEGRAM_CHAT_ID not set in .env")
		return
	}

	// Create client
	client, err := rest.New(
		fmt.Sprintf("https://api.telegram.org/bot%s", botToken),
		rest.WithProxy("http://127.0.0.1:7890"),
		rest.WithDebug(true),
	)
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		return
	}

	// Define response structures
	var successResp TelegramSuccessResponse
	var errorResp TelegramErrorResponse

	// Create and configure request
	req, err := client.R(
		rest.WithQuery("chat_id", chatID),
		rest.WithQuery("text", "Hello from Go!"),
		rest.WithResult(&successResp),
		rest.WithError(&errorResp),
	)
	if err != nil {
		fmt.Printf("Failed to create request: %v\n", err)
		return
	}

	// Execute request
	resp, err := req.Post("/sendMessage")
	if err != nil {
		fmt.Printf("Failed to send message: %v\n", err)
		return
	}

	// Log raw response
	fmt.Printf("Raw response: %s\n", resp.String())

	// Handle response
	if !resp.IsSuccess() {
		fmt.Printf("Request failed with status: %s\n", resp.Status())
		if !errorResp.Ok {
			fmt.Printf("Telegram API error: %d - %s\n", errorResp.ErrorCode, errorResp.Description)
		}
		return
	}

	if !successResp.Ok {
		fmt.Println("Unexpected response format")
		return
	}

	// Success case
	fmt.Println("Message sent successfully!")
	fmt.Printf("Message ID: %d\n", successResp.Result.MessageID)
}
