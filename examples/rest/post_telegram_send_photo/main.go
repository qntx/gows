package main

import (
	"fmt"
	"os"

	"github.com/joho/godotenv"
	"github.com/qntx/gonet/rest"
)

// TelegramResponse represents a Telegram API response.
type TelegramResponse struct {
	Ok          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code,omitempty"`
	Description string `json:"description,omitempty"`
	Result      struct {
		MessageID int `json:"message_id"`
		Chat      struct {
			ID   int64  `json:"id"`
			Type string `json:"type"`
		} `json:"chat"`
	} `json:"result,omitempty"`
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
		fmt.Println("Missing TELEGRAM_BOT_TOKEN or TELEGRAM_CHAT_ID")
		return
	}

	// Initialize REST client
	client, err := rest.New(
		fmt.Sprintf("https://api.telegram.org/bot%s", botToken),
		rest.WithProxy("http://127.0.0.1:7890"),
		rest.WithDebug(true),
	)
	if err != nil {
		fmt.Printf("Failed to create client: %v\n", err)
		return
	}

	// Prepare response data
	var respData TelegramResponse

	// Create and configure request
	req, err := client.R(
		rest.WithForm(map[string]any{
			"chat_id": chatID,
			"caption": "Hello from Go!",
		}),
		rest.WithFile("photo", "photo.png"),
		rest.WithResult(&respData),
	)
	if err != nil {
		fmt.Printf("Failed to create request: %v\n", err)
		return
	}

	// Execute request
	resp, err := req.Post("/sendPhoto")
	if err != nil {
		fmt.Printf("Failed to send photo: %v\n", err)
		return
	}

	// Handle response
	if !resp.IsSuccess() || !respData.Ok {
		fmt.Printf("Telegram API error: %d - %s\n", respData.ErrorCode, respData.Description)
		return
	}

	// Success case
	fmt.Println("Photo sent successfully!")
	fmt.Printf("Message ID: %d\n", respData.Result.MessageID)
}
