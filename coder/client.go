// Package coder provides a configurable WebSocket client.
// It leverages the coder/websocket library for underlying WebSocket functionality
// and is designed for concurrent safety.
package coder

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/qntx/gows"
)

const (
	DefaultTimeout = 30 * time.Second
)

var (
	ErrNotConnected     = fmt.Errorf("websocket client is not connected")
	ErrAlreadyConnected = fmt.Errorf("websocket client is already connected")
)

// Config holds the configuration for the client.
type Config struct {
	URL         string
	Timeout     time.Duration
	Heartbeat   time.Duration
	ReadLimit   int64
	DialOptions *websocket.DialOptions

	OnConnect    func()
	OnDisconnect func(err error)
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

	ctx    context.Context
	cancel context.CancelFunc
}

// New creates a new WebSocket client.
func New(cfg Config) *Client {
	if cfg.Timeout == 0 {
		cfg.Timeout = DefaultTimeout
	}

	return &Client{
		cfg: cfg,
	}
}

// Connect establishes a WebSocket connection.
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isConnected {
		return ErrAlreadyConnected
	}

	dialCtx, dialCancel := context.WithTimeout(ctx, c.cfg.Timeout)
	defer dialCancel()

	conn, httpResp, err := websocket.Dial(dialCtx, c.cfg.URL, c.cfg.DialOptions)
	if err != nil {
		return fmt.Errorf("failed to dial websocket: %w", err)
	}

	if c.cfg.ReadLimit > 0 {
		conn.SetReadLimit(c.cfg.ReadLimit)
	}

	c.ctx, c.cancel = context.WithCancel(context.Background())
	c.conn = conn
	c.httpResp = httpResp
	c.isConnected = true

	if c.cfg.Heartbeat > 0 {
		go c.heartbeat(c.ctx)
	}

	if c.cfg.OnConnect != nil {
		c.cfg.OnConnect()
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

	if c.cfg.OnDisconnect != nil {
		c.cfg.OnDisconnect(err)
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
func (c *Client) Reader(ctx context.Context, v any) (gows.MessageType, io.Reader, error) {
	conn, err := c.getConn()
	if err != nil {
		return 0, nil, err
	}

	typ, r, err := conn.Reader(ctx)
	if err != nil {
		return 0, nil, err
	}

	if v != nil {
		if err := json.NewDecoder(r).Decode(v); err != nil {
			return 0, nil, err
		}
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

	return conn.Writer(ctx, websocket.MessageType(typ))
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

// HandshakeResponse returns the HTTP response from the initial WebSocket handshake.
// It can be useful for inspecting headers, cookies, or the status code.
// The response is nil if the client has not connected yet.
func (c *Client) HandshakeResponse() *http.Response {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.httpResp
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
			pingCtx, cancel := context.WithTimeout(ctx, c.cfg.Timeout)
			c.conn.Ping(pingCtx)
			cancel()
		}
	}
}
