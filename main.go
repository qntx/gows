package main

import (
	"errors"
	"time"

	"github.com/qntx/gonet/websocket"
)

func main() {
	// Simulate connection event
	websocket.PrintConnectMessage()
	time.Sleep(500 * time.Millisecond)

	// Simulate text message
	websocket.PrintTextMessage([]byte(`{"message": "Hello, WebSocket!", "type": "greeting"}`), "Received from server")
	time.Sleep(500 * time.Millisecond)

	// Simulate binary message
	binaryData := []byte{0x01, 0x02, 0x03, 0x04, 0x05}
	websocket.PrintBinaryMessage(binaryData, "Received from server")
	time.Sleep(500 * time.Millisecond)

	// Simulate ping message
	websocket.PrintPingMessage([]byte("ping"), "Keep alive")
	time.Sleep(500 * time.Millisecond)

	// Simulate pong message
	websocket.PrintPongMessage([]byte("pong"), "Response to ping")
	time.Sleep(500 * time.Millisecond)

	// Simulate error message
	websocket.PrintErrorMessage(errors.New("Connection timed out"))
	time.Sleep(500 * time.Millisecond)

	// Simulate retry connection
	websocket.PrintRetryMessage(2, 5, errors.New("Connection failed, retrying"))
	time.Sleep(500 * time.Millisecond)

	// Simulate connection closure
	websocket.PrintCloseMessage(1000, "Normal closure")
}
