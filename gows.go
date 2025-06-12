package gows

import (
	"context"
)

type MessageType int

const (
	// MessageText is for UTF-8 encoded text messages like JSON.
	MessageText MessageType = iota + 1
	// MessageBinary is for binary messages like protobufs.
	MessageBinary
)

type Client interface {
	// Connect establishes a WebSocket connection to the server.
	// It takes a context for cancellation and timeout control.
	// Returns an error if the connection fails.
	Connect(ctx context.Context) error
	// Close gracefully closes the WebSocket connection.
	// It should be idempotent.
	// Returns an error if closing the connection fails.
	Close() error

	// Read reads a message from the WebSocket connection.
	// It blocks until a message is received, the context is cancelled, or an error occurs.
	// The `v` parameter is an interface to allow unmarshalling into different types (e.g., a struct for JSON).
	// It returns the MessageType (Text or Binary), the raw message payload as a byte slice, and an error.
	// Returns io.EOF if the connection is closed by the peer.
	Read(ctx context.Context, v any) (MessageType, []byte, error)
	// Write sends a message to the WebSocket connection.
	// It blocks until the message is sent, the context is cancelled, or an error occurs.
	// `typ` specifies the MessageType (Text or Binary) of the message.
	// `p` is the byte slice containing the message payload.
	// Returns an error if the write operation fails or the connection is closed.
	Write(ctx context.Context, typ MessageType, p []byte) error
}
