// Package coder provides a configurable WebSocket client.
// It leverages the coder/websocket library for underlying WebSocket functionality
// and is designed for concurrent safety.
package coder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/qntx/gows"
)

var (
	// Errors - Connection.
	ErrNotConnected     = errors.New("gows/coder: websocket client is not connected")
	ErrAlreadyConnected = errors.New("gows/coder: websocket client is already connected")
	ErrNotListening     = errors.New("gows/coder: websocket client is not listening")
	ErrAlreadyListening = errors.New("gows/coder: websocket client is already listening")

	// Errors - Events.
	ErrInvalidEventType = errors.New("gows/coder: invalid event type")

	// Errors - Callbacks.
	ErrNoOnConnect = errors.New("gows/coder: OnConnect callback is not configured")
	ErrNoOnClose   = errors.New("gows/coder: OnClose callback is not configured")
	ErrNoOnMessage = errors.New("gows/coder: OnMessage callback is not configured")
	ErrNoOnError   = errors.New("gows/coder: OnError callback is not configured")
)

// Config holds the configuration for the client.
type Config struct {
	Context     context.Context
	URL         string
	Heartbeat   time.Duration
	ReadLimit   int64
	DialOptions *websocket.DialOptions

	Listening bool // Automatically start listening for messages after connection
	OnConnect func()
	OnClose   func()
	OnMessage func(gows.MessageType, []byte)
	OnError   func(error)
}

var _ gows.Client = (*Client)(nil)

// Client is a thread-safe WebSocket client wrapper for coder/websocket.
// It allows for one concurrent reader and multiple concurrent writers.
type Client struct {
	cfg Config

	mu          sync.Mutex
	conn        *websocket.Conn
	httpResp    *http.Response
	isConnected bool
	isListening bool

	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new WebSocket client.
func New(cfg Config) *Client {
	if cfg.URL == "" {
		panic("URL is required")
	}

	if cfg.Context == nil {
		cfg.Context = context.Background()
	}

	ctx, cancel := context.WithCancel(cfg.Context)

	return &Client{
		cfg:    cfg,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Connect establishes a WebSocket connection.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isConnected {
		return ErrAlreadyConnected
	}

	var err error

	c.conn, c.httpResp, err = websocket.Dial(ctx, c.cfg.URL, c.cfg.DialOptions)
	if err != nil {
		return fmt.Errorf("failed to dial websocket: %w", err)
	}

	if c.cfg.ReadLimit > 0 {
		c.conn.SetReadLimit(c.cfg.ReadLimit)
	}

	if c.cfg.Heartbeat > 0 {
		go c.heartbeat(c.ctx)
	}

	c.isConnected = true

	// Call OnConnect callback asynchronously to avoid blocking
	if c.cfg.OnConnect != nil {
		c.safeCallback(c.cfg.OnConnect)
	}

	// Automatically start listening for messages if configured and OnMessage callback is set
	if c.cfg.Listening && c.cfg.OnMessage != nil {
		c.isListening = true
		go c.messageListener(c.ctx)
	}

	return nil
}

// Close gracefully closes the WebSocket connection.
// It's safe to call Close multiple times.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isConnected {
		return nil
	}

	c.isConnected = false

	if c.cancel != nil {
		c.cancel()
	}

	err := c.conn.Close(websocket.StatusNormalClosure, "closing connection")
	c.conn = nil

	// Call OnClose callback asynchronously to avoid blocking
	if c.cfg.OnClose != nil {
		c.safeCallback(c.cfg.OnClose)
	}

	return err
}

// Read reads a single message. It directly uses the underlying library's
// convenience method for simplicity and efficiency.
// IMPORTANT: Your application should have only ONE goroutine calling Read.
func (c *Client) Read(ctx context.Context, v any) (gows.MessageType, []byte, error) {
	conn, err := c.getConn()
	if err != nil {
		return 0, nil, err
	}

	typ, p, err := conn.Read(ctx)
	if err != nil {
		return 0, nil, err
	}

	if v != nil {
		if err := json.Unmarshal(p, v); err != nil {
			return 0, nil, err
		}
	}

	return gows.MessageType(typ), p, nil
}

// Reader returns a streaming reader for the next message.
// This is useful for very large messages that shouldn't be loaded into memory at once.
// IMPORTANT: Your application should have only ONE goroutine calling Reader.
func (c *Client) Reader(ctx context.Context) (gows.MessageType, io.Reader, error) {
	conn, err := c.getConn()
	if err != nil {
		return 0, nil, err
	}

	typ, r, err := conn.Reader(ctx)
	if err != nil {
		return 0, nil, err
	}

	return gows.MessageType(typ), r, nil
}

// Write writes a single message. It is safe for concurrent use by multiple goroutines.
func (c *Client) Write(ctx context.Context, typ gows.MessageType, p []byte) error {
	conn, err := c.getConn()
	if err != nil {
		return err
	}

	return conn.Write(ctx, websocket.MessageType(typ), p)
}

// Writer returns a streaming writer for a new message.
func (c *Client) Writer(ctx context.Context, typ gows.MessageType) (io.WriteCloser, error) {
	conn, err := c.getConn()
	if err != nil {
		return nil, err
	}

	w, err := conn.Writer(ctx, websocket.MessageType(typ))
	if err != nil {
		return nil, err
	}

	return w, nil
}

// HandshakeResponse returns the HTTP response from the initial WebSocket handshake.
// It can be useful for inspecting headers, cookies, or the status code.
// The response is nil if the client has not connected yet.
func (c *Client) HandshakeResponse() *http.Response {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.httpResp
}

// StartListening starts a background goroutine that continuously listens for incoming messages
// and calls the OnMessage callback for each received message.
// This method should be called after Connect() and will return an error if not connected.
// Only one listening goroutine can be active at a time.
func (c *Client) StartListening() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isConnected {
		return ErrNotConnected
	}

	if c.isListening {
		return ErrAlreadyListening
	}

	if c.cfg.OnMessage == nil {
		return ErrNoOnMessage
	}

	c.isListening = true

	go c.messageListener(c.ctx)

	return nil
}

// StopListening stops the background message listening goroutine.
func (c *Client) StopListening() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.isListening = false
}

// IsListening returns whether the client is currently listening for messages.
func (c *Client) IsListening() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.isListening
}

// Supported event types: EventConnect, EventClose, EventMessage, EventError.
func (c *Client) On(eventType gows.EventType, callback any) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	switch eventType {
	case gows.EventConnect:
		if cb, ok := callback.(func()); ok {
			c.cfg.OnConnect = cb
		} else {
			return ErrInvalidEventType
		}
	case gows.EventClose:
		if cb, ok := callback.(func()); ok {
			c.cfg.OnClose = cb
		} else {
			return ErrInvalidEventType
		}
	case gows.EventMessage:
		if cb, ok := callback.(func(gows.MessageType, []byte)); ok {
			c.cfg.OnMessage = cb
		} else {
			return ErrInvalidEventType
		}
	case gows.EventError:
		if cb, ok := callback.(func(error)); ok {
			c.cfg.OnError = cb
		} else {
			return ErrInvalidEventType
		}
	default:
		return ErrInvalidEventType
	}

	return nil
}

// messageListener continuously reads incoming messages and calls the OnMessage callback.
func (c *Client) messageListener(ctx context.Context) {
	defer func() {
		c.mu.Lock()
		c.isListening = false
		c.mu.Unlock()
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Check if we should continue listening
		c.mu.Lock()
		if !c.isListening || !c.isConnected {
			c.mu.Unlock()

			return
		}

		conn := c.conn
		c.mu.Unlock()

		if conn == nil {
			return
		}

		typ, p, err := conn.Read(ctx)
		if err != nil {
			// Connection error, stop listening and notify user
			c.safeCallbackWithError(c.cfg.OnError, err)

			return
		}

		// Call OnMessage callback safely
		c.safeCallbackWithMessage(c.cfg.OnMessage, gows.MessageType(typ), p)
	}
}

// getConn safely retrieves the current connection object.
func (c *Client) getConn() (*websocket.Conn, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.isConnected {
		return nil, ErrNotConnected
	}

	return c.conn, nil
}

// heartbeat sends periodic pings to keep the connection alive.
func (c *Client) heartbeat(ctx context.Context) {
	t := time.NewTicker(c.cfg.Heartbeat)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done(): // This context is cancelled by c.Close()
			return
		case <-t.C:
		}

		err := c.conn.Ping(ctx)
		if err != nil {
			c.safeCallbackWithError(c.cfg.OnError, err)

			return
		}

		t.Reset(c.cfg.Heartbeat)
	}
}

// safeCallback calls the callback function asynchronously to avoid blocking.
func (c *Client) safeCallback(cb func()) {
	if cb != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log the panic but don't crash the client
					// In a production environment, you might want to use a proper logger
				}
			}()
			cb()
		}()
	}
}

// safeCallbackWithError calls the error callback function asynchronously.
func (c *Client) safeCallbackWithError(cb func(error), err error) {
	if cb != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log the panic but don't crash the client
					// In a production environment, you might want to use a proper logger
				}
			}()
			cb(err)
		}()
	}
}

// safeCallbackWithMessage calls the callback function asynchronously to avoid blocking.
func (c *Client) safeCallbackWithMessage(cb func(gows.MessageType, []byte), typ gows.MessageType, p []byte) {
	if cb != nil {
		go func() {
			defer func() {
				if r := recover(); r != nil {
					// Log the panic but don't crash the client
					// In a production environment, you might want to use a proper logger
				}
			}()
			cb(typ, p)
		}()
	}
}
