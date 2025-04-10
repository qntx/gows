package websocket

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/fatih/color"
)

// MessageType represents the types of WebSocket messages for debugging purposes.
type MessageType string

// WebSocket message types for debugging.
const (
	MessageConnect MessageType = "CONNECT"
	MessageText    MessageType = "TEXT"
	MessageBinary  MessageType = "BINARY"
	MessagePing    MessageType = "PING"
	MessagePong    MessageType = "PONG"
	MessageClose   MessageType = "CLOSE"
	MessageRetry   MessageType = "RETRY"
	MessageError   MessageType = "ERROR"
)

// PrintMessage prints WebSocket message details with colorized formatting for debugging.
// It handles various message types like text, binary, ping, pong, close, retry, and errors.
func PrintMessage(msgType MessageType, data []byte, info string, duration ...time.Duration) {
	cyanBold := color.New(color.FgHiCyan, color.Bold)
	_, _ = cyanBold.Println("╔══════════════════ WebSocket Message ══════════════╗")

	// Print message type with colored icon
	green := color.New(color.FgGreen)
	_, _ = green.Print("  📡 Type: ")

	typeColor := getColorForMessageType(msgType)
	_, _ = typeColor.Printf("%s", msgType)
	fmt.Println()

	// Print additional info if provided
	if info != "" {
		_, _ = green.Print("  ℹ️  Info: ")
		fmt.Println(info)
	}

	// Print message content if exists
	if len(data) > 0 {
		_, _ = color.New(color.FgHiMagenta, color.Bold).Println("  📦 Content:")
		printMessageContent(data, msgType)
	}

	// Print duration if provided
	if len(duration) > 0 && duration[0] > 0 {
		yellow := color.New(color.FgHiYellow)
		_, _ = yellow.Print("  ⏱ Duration: ")
		fmt.Println(duration[0])
	}

	_, _ = cyanBold.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println()
}

// getColorForMessageType returns appropriate color for different message types.
func getColorForMessageType(msgType MessageType) *color.Color {
	switch msgType {
	case MessageConnect:
		return color.New(color.FgGreen, color.Bold)
	case MessageText:
		return color.New(color.FgBlue, color.Bold)
	case MessageBinary:
		return color.New(color.FgMagenta, color.Bold)
	case MessagePing, MessagePong:
		return color.New(color.FgCyan, color.Bold)
	case MessageClose:
		return color.New(color.FgYellow, color.Bold)
	case MessageRetry:
		return color.New(color.FgHiYellow, color.Bold)
	case MessageError:
		return color.New(color.FgRed, color.Bold)
	default:
		return color.New(color.FgWhite, color.Bold)
	}
}

// printMessageContent formats and prints message content based on its type.
func printMessageContent(data []byte, msgType MessageType) {
	if len(data) == 0 {
		fmt.Println("    <empty>")
		return
	}

	// Try to format JSON for text messages
	if msgType == MessageText && isJSON(data) {
		var prettyJSON bytes.Buffer
		if json.Indent(&prettyJSON, data, "    ", "  ") == nil {
			fmt.Println(prettyJSON.String())
			return
		}
	}

	// Default formatting for all message types
	lines := strings.Split(string(data), "\n")
	fmt.Println()

	for _, line := range lines {
		if line != "" {
			fmt.Printf("    %s\n", line)
		}
	}
}

// isJSON checks if a byte slice contains valid JSON data.
func isJSON(data []byte) bool {
	var js json.RawMessage
	return json.Unmarshal(data, &js) == nil
}

// PrintConnectMessage prints a connection event with debug formatting.
func PrintConnectMessage() {
	PrintMessage(MessageConnect, nil, "")
}

// PrintTextMessage prints a received text message with debug formatting.
func PrintTextMessage(data []byte, info string) {
	PrintMessage(MessageText, data, info)
}

// PrintBinaryMessage prints a received binary message with debug formatting.
func PrintBinaryMessage(data []byte, info string) {
	PrintMessage(MessageBinary, data, info)
}

// PrintPingMessage prints a received ping message with debug formatting.
func PrintPingMessage(data []byte, info string) {
	PrintMessage(MessagePing, data, info)
}

// PrintPongMessage prints a received pong message with debug formatting.
func PrintPongMessage(data []byte, info string) {
	PrintMessage(MessagePong, data, info)
}

// PrintCloseMessage prints a connection close event with debug formatting.
func PrintCloseMessage(code int, text string) {
	info := fmt.Sprintf("Code: %d", code)
	PrintMessage(MessageClose, []byte(text), info)
}

// PrintErrorMessage prints an error with debug formatting.
func PrintErrorMessage(err error) {
	if err == nil {
		return
	}
	PrintMessage(MessageError, []byte(err.Error()), "")
}

// PrintRetryMessage prints a reconnection attempt with debug formatting.
func PrintRetryMessage(attempt, maxAttempts uint, err error) {
	info := fmt.Sprintf("Attempt: %d/%d", attempt, maxAttempts)
	var data []byte
	if err != nil {
		data = []byte(err.Error())
	}
	PrintMessage(MessageRetry, data, info)
}
