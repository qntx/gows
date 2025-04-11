// Package websocket provides a configurable WebSocket client with support for reconnection,
// message handling, and event-driven callbacks. It leverages the gorilla/websocket library
// for underlying WebSocket functionality and is designed for concurrent safety.
package websocket

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/qntx/gonet/util"
)

// --------------------------------------------------------------------------------
// Constants

// Constants defining default configuration values for the WebSocket client.
const (
	DefaultTimeout      = 30 * time.Second // Default timeout for connection and operations.
	DefaultRetryCount   = 3                // Default number of reconnection attempts.
	DefaultRetryWait    = 1 * time.Second  // Default initial delay between retries.
	DefaultRetryMaxWait = 30 * time.Second // Default maximum delay for exponential backoff.
	DefaultPingInterval = 30 * time.Second // Default interval for keep-alive pings.
	DefaultPingMessage  = "ping"           // Default payload for ping messages.
)

// --------------------------------------------------------------------------------
// Types

// Option defines a function that configures a Client and returns an error if configuration fails.
type Option func(*Client) error

// Config encapsulates settings for a WebSocket client, controlling connection, messaging, and resilience.
//
// All fields are optional; unset values fall back to defaults defined above.
type Config struct {
	Proxy             func(*http.Request) (*url.URL, error) // Proxy routing function; nil disables proxy.
	TLSClientConfig   *tls.Config                           // TLS settings for wss://; nil uses system defaults.
	Timeout           time.Duration                         // Timeout for handshake and operations.
	ReadBufferSize    int                                   // Read buffer size in bytes; 0 for default (typically 4096).
	WriteBufferSize   int                                   // Write buffer size in bytes; 0 for default (typically 4096).
	Subprotocols      []string                              // Supported subprotocols; nil for none.
	EnableCompression bool                                  // Enables RFC 7692 per-message compression if true.
	ReadLimit         int64                                 // Max message size in bytes; 0 for no limit.
	RetryCount        uint                                  // Max reconnection attempts; 0 disables retries.
	RetryWaitTime     time.Duration                         // Initial delay between retries.
	RetryMaxWaitTime  time.Duration                         // Max delay for exponential backoff.
	KeepAlive         bool                                  // Enables periodic pings if true.
	PingInterval      time.Duration                         // Interval between ping messages.
	PingMessage       []byte                                // Custom ping payload; must be short to avoid overhead.
	AsyncCallbacks    bool                                  // Runs callbacks asynchronously if true; use with caution in high-throughput scenarios.
	Debug             bool                                  // Enables debug logging of WebSocket messages and events.
}

// Client manages a WebSocket connection with configurable behavior and event-driven hooks.
//
// It is safe for concurrent use when sending messages or accessing state.
type Client struct {
	config          Config      // Client configuration settings.
	url             string      // WebSocket server endpoint (e.g., ws://example.com).
	header          http.Header // HTTP headers for the connection handshake.
	conn            *websocket.Conn
	dialer          *websocket.Dialer
	connected       bool
	closed          bool
	connMu          sync.RWMutex    // Protects connection state (conn, connected, closed).
	sendMu          sync.Mutex      // Ensures thread-safe message sending.
	wg              sync.WaitGroup  // Tracks active goroutines for clean shutdown.
	reconnectCh     chan struct{}   // Signals reconnection attempts; buffered to avoid blocking.
	messagesCh      chan []byte     // Optional channel for received messages.
	errorsCh        chan error      // Optional channel for error reporting.
	ctx             context.Context // Lifecycle context; cancelled on Close().
	cancel          context.CancelFunc
	onConnected     func(*Client)                    // Callback for successful connection.
	onTextMessage   func([]byte, *Client)            // Callback for text messages.
	onBinaryMessage func([]byte, *Client)            // Callback for binary messages.
	onPingReceived  func(string, *Client)            // Callback for ping receipt.
	onPongReceived  func(string, *Client)            // Callback for pong receipt.
	onClosed        func(int, string, *Client)       // Callback for connection closure with code and reason.
	onError         func(error, *Client)             // Callback for error handling.
	onRetry         func(uint, uint, error, *Client) // Callback for reconnection attempts
}

// --------------------------------------------------------------------------------
// Initialization

// New creates a new WebSocket client with the specified endpoint and options.
//
// It returns the client and an error if any option fails to apply.
// The client is not connected until Connect() is called.
func New(endpoint string, opts ...Option) (*Client, error) {
	ctx, cancel := context.WithCancel(context.Background())
	c := &Client{
		config: Config{
			Timeout:          DefaultTimeout,
			RetryCount:       DefaultRetryCount,
			RetryWaitTime:    DefaultRetryWait,
			RetryMaxWaitTime: DefaultRetryMaxWait,
			PingInterval:     DefaultPingInterval,
			PingMessage:      []byte(DefaultPingMessage),
		},
		url:         endpoint,
		header:      make(http.Header),
		reconnectCh: make(chan struct{}, 1),
		ctx:         ctx,
		cancel:      cancel,
	}

	// Apply all provided options
	return c.With(opts...)
}

// With applies a list of options to the Client and returns the modified instance along with any error.
//
// It follows the functional options pattern for elegant configuration with explicit error handling.
func (c *Client) With(opts ...Option) (*Client, error) {
	for i, opt := range opts {
		if opt == nil {
			continue // Skip nil options gracefully
		}

		if err := opt(c); err != nil {
			return c, fmt.Errorf("failed to apply option at index %d: %w", i, err)
		}
	}

	return c, nil
}

// --------------------------------------------------------------------------------
// Connection Management

// Connect establishes a WebSocket connection using the configured settings.
//
// It returns an error if the handshake fails or the client is already closed.
func (c *Client) Connect() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.closed {
		return errors.New("client is closed")
	}

	c.dialer = &websocket.Dialer{
		Proxy:             c.config.Proxy,
		TLSClientConfig:   c.config.TLSClientConfig,
		HandshakeTimeout:  c.config.Timeout,
		ReadBufferSize:    c.config.ReadBufferSize,
		WriteBufferSize:   c.config.WriteBufferSize,
		Subprotocols:      c.config.Subprotocols,
		EnableCompression: c.config.EnableCompression,
	}

	if c.config.Debug {
		PrintConnectMessage()
	}

	conn, _, err := c.dialer.DialContext(c.ctx, c.url, c.header)
	if err != nil {
		if c.config.Debug {
			PrintErrorMessage(fmt.Errorf("dial failed: %w", err))
		}
		return fmt.Errorf("dial failed: %w", err)
	}

	c.conn = conn
	c.connected = true
	conn.SetReadLimit(c.config.ReadLimit)
	c.setupHandlers()

	if c.onConnected != nil {
		c.invoke(func() { c.onConnected(c) })
	}

	c.wg.Add(1)

	go c.run()

	return nil
}

// Close gracefully terminates the client, closing the connection and freeing resources.
//
// It blocks until all goroutines complete; returns any error from closing the connection.
func (c *Client) Close() {
	c.cancel()
	c.wg.Wait()
}

// Connected reports whether the client is currently connected to the server.
func (c *Client) Connected() bool {
	c.connMu.RLock()
	defer c.connMu.RUnlock()

	return c.connected
}

// Closed reports whether the client has been permanently shut down.
func (c *Client) Closed() bool {
	c.connMu.RLock()
	defer c.connMu.RUnlock()

	return c.closed
}

// Context returns the client's lifecycle context for external monitoring.
func (c *Client) Context() context.Context {
	return c.ctx
}

// --------------------------------------------------------------------------------
// Message Handling

// SendText sends a text message over the WebSocket connection.
//
// It returns an error if the send operation fails.
func (c *Client) SendText(msg []byte) error {
	return c.send(websocket.TextMessage, msg)
}

// SendBinary sends a binary message over the WebSocket connection.
//
// It returns an error if the send operation fails.
func (c *Client) SendBinary(data []byte) error {
	return c.send(websocket.BinaryMessage, data)
}

// HasMessages checks if the messages channel is enabled.
func (c *Client) HasMessages() bool {
	return c.messagesCh != nil
}

// HasErrors checks if the errors channel is enabled.
func (c *Client) HasErrors() bool {
	return c.errorsCh != nil
}

// Messages provides a read-only channel for receiving messages, if enabled.
//
// Logs a warning if the channel is not configured.
func (c *Client) Messages() <-chan []byte {
	return c.messagesCh
}

// Errors provides a read-only channel for receiving errors, if enabled.
//
// Logs a warning if the channel is not configured.
func (c *Client) Errors() <-chan error {
	return c.errorsCh
}

// --------------------------------------------------------------------------------
// Lifecycle Management (Private)

// run manages the client's main loop, handling messages, errors, and reconnection.
//
// It runs until the context is cancelled or an unrecoverable error occurs.
func (c *Client) run() {
	defer c.wg.Done()
	defer c.shutdown()

	if c.config.KeepAlive {
		c.wg.Add(1)

		go func() {
			defer c.wg.Done()
			c.keepAlive()
		}()
	}

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.reconnectCh:
			c.reconnect()
		default:
			if err := c.read(); err != nil {
				c.handleError(err)
			}
		}
	}
}

// setupHandlers configures WebSocket event handlers for ping, pong, and close events.
func (c *Client) setupHandlers() {
	c.conn.SetPingHandler(func(data string) error {
		if c.onPingReceived != nil {
			c.invoke(func() { c.onPingReceived(data, c) })
		}

		if c.config.Debug {
			PrintPingMessage([]byte(data), "Received from server")
		}

		return c.sendPong([]byte(data))
	})

	c.conn.SetPongHandler(func(data string) error {
		if c.onPongReceived != nil {
			c.invoke(func() { c.onPongReceived(data, c) })
		}

		if c.config.Debug {
			PrintPongMessage([]byte(data), "Received from server")
		}

		return nil
	})

	c.conn.SetCloseHandler(func(code int, text string) error {
		if c.onClosed != nil {
			c.invoke(func() { c.onClosed(code, text, c) })
		}

		if c.config.Debug {
			PrintCloseMessage(code, text)
		}

		return nil
	})
}

// keepAlive sends periodic ping messages to maintain the connection.
//
// It runs until the context is cancelled or a ping fails.
func (c *Client) keepAlive() {
	ticker := time.NewTicker(c.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			if c.Connected() {
				if c.config.Debug {
					PrintPingMessage(c.config.PingMessage, "Sending keep-alive ping")
				}

				if err := c.sendPing(c.config.PingMessage); err != nil {
					c.handleError(fmt.Errorf("keep-alive ping failed: %w", err))

					return
				}
			}
		}
	}
}

// reconnect attempts to re-establish the connection after a failure.
//
// It respects the retry count and uses exponential backoff with jitter for delays.
func (c *Client) reconnect() {
	c.disconnect()

	for i := range c.config.RetryCount {
		attemptNumber := i + 1

		err := c.Connect()

		if c.onRetry != nil {
			c.invoke(func() { c.onRetry(attemptNumber, c.config.RetryCount, err, c) })
		}

		if c.config.Debug && err != nil {
			PrintRetryMessage(attemptNumber, c.config.RetryCount, err)
		}

		if err == nil {
			return
		}

		if err := util.Wait(c.ctx, i+1, c.config.RetryWaitTime, c.config.RetryMaxWaitTime, util.DefaultJitterFactor); err != nil {
			return
		}
	}

	c.cancel()
}

// shutdown closes the connection and cleans up resources.
//
// It ensures a graceful shutdown and logs any errors encountered.
func (c *Client) shutdown() error {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	if c.closed {
		return nil
	}

	c.closed = true

	if c.conn != nil {
		closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")

		if c.config.Debug {
			PrintCloseMessage(websocket.CloseNormalClosure, "Normal closure")
		}

		if err := c.conn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
			return fmt.Errorf("failed to send close message: %w", err)
		}

		if err := c.conn.Close(); err != nil {
			return fmt.Errorf("connection close failed: %w", err)
		}

		c.conn = nil
		c.connected = false
	}

	closeChannel(c.messagesCh)
	closeChannel(c.errorsCh)

	return nil
}

// disconnect terminates the current connection without cleanup.
//
// It is used during reconnection attempts.
func (c *Client) disconnect() {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	c.connected = false
	if c.conn != nil {
		_ = c.conn.Close()
		c.conn = nil
	}
}

// --------------------------------------------------------------------------------
// Message Handling (Private)

// read processes incoming WebSocket messages and triggers callbacks.
//
// It returns an error if reading fails, triggering reconnection if appropriate.
func (c *Client) read() error {
	conn := c.connection()
	if conn == nil {
		return errors.New("no active connection")
	}

	msgType, data, err := conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("message read failed: %w", err)
	}

	c.sendMessage(data)

	if c.config.Debug {
		switch msgType {
		case websocket.TextMessage:
			PrintTextMessage(data, "Received from server")
		case websocket.BinaryMessage:
			PrintBinaryMessage(data, "Received from server")
		}
	}

	switch msgType {
	case websocket.TextMessage:
		if c.onTextMessage != nil {
			c.invoke(func() { c.onTextMessage(data, c) })
		}
	case websocket.BinaryMessage:
		if c.onBinaryMessage != nil {
			c.invoke(func() { c.onBinaryMessage(data, c) })
		}
	}

	return nil
}

// send transmits a message of the specified type over the connection.
//
// It ensures thread safety and sets a write deadline based on the timeout.
func (c *Client) send(msgType int, data []byte) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	conn := c.connection()
	if conn == nil {
		return errors.New("disconnected")
	}

	if err := conn.SetWriteDeadline(time.Now().Add(c.config.Timeout)); err != nil {
		return fmt.Errorf("set write deadline failed: %w", err)
	}

	if err := conn.WriteMessage(msgType, data); err != nil {
		c.handleError(fmt.Errorf("write failed: %w", err))

		return err
	}

	if c.config.Debug {
		switch msgType {
		case websocket.TextMessage:
			PrintTextMessage(data, "Sent to server")
		case websocket.BinaryMessage:
			PrintBinaryMessage(data, "Sent to server")
		}
	}

	return nil
}

// sendPing sends a ping message to the server.
//
// It uses the configured ping message payload.
func (c *Client) sendPing(data []byte) error {
	return c.send(websocket.PingMessage, data)
}

// sendPong sends a pong message in response to a ping.
//
// It typically echoes the received ping data.
func (c *Client) sendPong(data []byte) error {
	return c.send(websocket.PongMessage, data)
}

// --------------------------------------------------------------------------------
// Utilities (Private)

// connection returns the current WebSocket connection with read safety.
//
// It uses a read lock to allow concurrent access to the connection state.
func (c *Client) connection() *websocket.Conn {
	c.connMu.RLock()
	defer c.connMu.RUnlock()

	return c.conn
}

// handleError logs an error and triggers reconnection if appropriate.
//
// It sends the error to the errors channel if configured.
func (c *Client) handleError(err error) {
	if c.config.Debug {
		PrintErrorMessage(err)
	}

	c.sendError(err)

	// Call error callback if configured
	if c.onError != nil {
		c.invoke(func() { c.onError(err, c) })
	}

	c.connMu.RLock()
	defer c.connMu.RUnlock()

	if c.closed || c.ctx.Err() != nil {
		return
	}

	select {
	case c.reconnectCh <- struct{}{}:
	default:
	}
}

// invoke executes a callback synchronously or asynchronously based on config.
//
// It skips execution if the callback is nil.
func (c *Client) invoke(fn func()) {
	if fn == nil {
		return
	}

	if c.config.AsyncCallbacks {
		go fn()
	} else {
		fn()
	}
}

// closeChannel safely closes a channel if it exists.
// Supports both message and error channels via type switching.
func closeChannel(ch any) {
	if ch == nil {
		return
	}

	// Use defer and recover to handle potential panics from closing already closed channels
	defer func() {
		if r := recover(); r != nil {
			// Channel was likely already closed, ignore the panic
		}
	}()

	switch v := ch.(type) {
	case chan []byte:
		close(v)
	case chan error:
		close(v)
	}
}

// sendMessage delivers a received message to the messages channel.
//
// It drops the message if the channel is full or the context is cancelled.
func (c *Client) sendMessage(data []byte) {
	if c.messagesCh == nil {
		return
	}
	select {
	case c.messagesCh <- data:
	case <-c.ctx.Done():
	case <-time.After(c.config.Timeout):
		c.sendError(errors.New("message dropped: channel full or timeout"))
	}
}

// sendError delivers an error to the errors channel.
//
// It drops the error if the channel is full or the context is cancelled.
func (c *Client) sendError(err error) {
	if c.errorsCh == nil {
		return
	}
	select {
	case c.errorsCh <- err:
	case <-c.ctx.Done():
	case <-time.After(c.config.Timeout):
	}
}

// --------------------------------------------------------------------------------
// Option Functions

// WithProxy configures the proxy using a URL string or custom function.
//
// Returns an error if the proxy URL is invalid or the type is unsupported.
func WithProxy(proxy any) Option {
	return func(c *Client) error {
		switch p := proxy.(type) {
		case string:
			if p == "" {
				c.config.Proxy = nil

				return nil
			}

			u, err := url.Parse(p)
			if err != nil {
				return fmt.Errorf("invalid proxy URL %q: %w", p, err)
			}

			c.config.Proxy = http.ProxyURL(u)
		case func(*http.Request) (*url.URL, error):
			c.config.Proxy = p
		case nil:
			c.config.Proxy = nil
		default:
			return fmt.Errorf("unsupported proxy type: %T", proxy)
		}

		return nil
	}
}

// WithEnvProxy enables proxy settings from environment variables.
func WithEnvProxy() Option {
	return func(c *Client) error {
		c.config.Proxy = http.ProxyFromEnvironment
		return nil
	}
}

// WithTLS sets the TLS configuration for secure connections.
func WithTLS(cfg *tls.Config) Option {
	return func(c *Client) error {
		c.config.TLSClientConfig = cfg

		return nil
	}
}

// WithTimeout sets the timeout for connection handshakes and write operations.
//
// Returns an error if the timeout is negative.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) error {
		if timeout < 0 {
			return fmt.Errorf("timeout cannot be negative: %v", timeout)
		}

		c.config.Timeout = timeout

		return nil
	}
}

// WithBuffers configures the read and write buffer sizes in bytes.
//
// Returns an error if either buffer size is negative.
func WithBuffers(read, write int) Option {
	return func(c *Client) error {
		if read < 0 || write < 0 {
			return fmt.Errorf("buffer sizes cannot be negative: read=%d, write=%d", read, write)
		}

		c.config.ReadBufferSize = read
		c.config.WriteBufferSize = write

		return nil
	}
}

// WithSubprotocols specifies supported WebSocket subprotocols.
func WithSubprotocols(protos ...string) Option {
	return func(c *Client) error {
		c.config.Subprotocols = protos

		return nil
	}
}

// WithCompression enables or disables RFC 7692 per-message compression.
func WithCompression(enable bool) Option {
	return func(c *Client) error {
		c.config.EnableCompression = enable

		return nil
	}
}

// WithReadLimit sets the maximum allowed message size in bytes.
//
// Returns an error if the limit is negative.
func WithReadLimit(limit int64) Option {
	return func(c *Client) error {
		if limit < 0 {
			return fmt.Errorf("read limit cannot be negative: %d", limit)
		}

		c.config.ReadLimit = limit

		return nil
	}
}

// WithRetries configures reconnection behavior with attempt count and backoff timing.
//
// If timing values are negative, it logs warnings and uses default values.
func WithRetries(count uint, wait, maxWait time.Duration) Option {
	return func(c *Client) error {
		if wait < 0 {
			wait = DefaultRetryWait
		}

		if maxWait < 0 {

			maxWait = DefaultRetryMaxWait
		}

		c.config.RetryCount = count
		c.config.RetryWaitTime = wait
		c.config.RetryMaxWaitTime = maxWait

		return nil
	}
}

// WithKeepAlive enables periodic ping messages with a custom interval and payload.
//
// Returns an error if the interval is negative.
func WithKeepAlive(interval time.Duration, msg []byte) Option {
	return func(c *Client) error {
		if interval < 0 {
			return fmt.Errorf("ping interval cannot be negative: %v", interval)
		}

		c.config.KeepAlive = true
		c.config.PingInterval = interval
		c.config.PingMessage = msg

		return nil
	}
}

// WithAsync enables asynchronous execution of callbacks.
func WithAsync(enable bool) Option {
	return func(c *Client) error {
		c.config.AsyncCallbacks = enable

		return nil
	}
}

// WithHeader adds a single key-value pair to the handshake headers.
//
// Returns an error if the key is empty.
func WithHeader(key, value string) Option {
	return func(c *Client) error {
		if key == "" {
			return errors.New("header key cannot be empty")
		}

		c.header.Set(key, value)

		return nil
	}
}

// WithHeaders applies multiple headers to the handshake from a map.
//
// Returns an error if any key is empty.
func WithHeaders(headers map[string]string) Option {
	return func(c *Client) error {
		for k, v := range headers {
			if k == "" {
				return errors.New("header key cannot be empty")
			}

			c.header.Set(k, v)
		}

		return nil
	}
}

// WithMessages enables a buffered channel for receiving messages.
//
// Returns an error if capacity is negative.
func WithMessages(capacity int) Option {
	return func(c *Client) error {
		if capacity < 0 {
			return fmt.Errorf("messages channel capacity cannot be negative: %d", capacity)
		}

		if c.messagesCh == nil {
			c.messagesCh = make(chan []byte, capacity)
		}

		return nil
	}
}

// WithErrors enables a buffered channel for error reporting.
//
// Returns an error if capacity is negative.
func WithErrors(capacity int) Option {
	return func(c *Client) error {
		if capacity < 0 {
			return fmt.Errorf("errors channel capacity cannot be negative: %d", capacity)
		}

		if c.errorsCh == nil {
			c.errorsCh = make(chan error, capacity)
		}

		return nil
	}
}

// WithContext replaces the default lifecycle context with a custom one.
//
// Returns an error if the context is nil.
func WithContext(ctx context.Context) Option {
	return func(c *Client) error {
		if ctx == nil {
			return errors.New("context cannot be nil")
		}

		c.ctx, c.cancel = context.WithCancel(ctx)

		return nil
	}
}

// OnConnect registers a callback for successful connection.
func OnConnect(fn func(*Client)) Option {
	return func(c *Client) error {
		c.onConnected = fn

		return nil
	}
}

// OnText registers a callback for incoming text messages.
func OnText(fn func([]byte, *Client)) Option {
	return func(c *Client) error {
		c.onTextMessage = fn

		return nil
	}
}

// OnBinary registers a callback for incoming binary messages.
func OnBinary(fn func([]byte, *Client)) Option {
	return func(c *Client) error {
		c.onBinaryMessage = fn

		return nil
	}
}

// OnPing registers a callback for received ping messages.
func OnPing(fn func(string, *Client)) Option {
	return func(c *Client) error {
		c.onPingReceived = fn

		return nil
	}
}

// OnPong registers a callback for received pong messages.
func OnPong(fn func(string, *Client)) Option {
	return func(c *Client) error {
		c.onPongReceived = fn

		return nil
	}
}

// OnClose registers a callback for connection closure.
func OnClose(fn func(int, string, *Client)) Option {
	return func(c *Client) error {
		c.onClosed = fn

		return nil
	}
}

// OnError registers a callback for error handling.
func OnError(fn func(error, *Client)) Option {
	return func(c *Client) error {
		c.onError = fn

		return nil
	}
}

// OnRetry registers a callback for reconnection attempts.
func OnRetry(fn func(uint, uint, error, *Client)) Option {
	return func(c *Client) error {
		c.onRetry = fn

		return nil
	}
}

// WithDebug enables or disables debug logging of WebSocket messages and events.
func WithDebug(enable bool) Option {
	return func(c *Client) error {
		c.config.Debug = enable
		return nil
	}
}

// --------------------------------------------------------------------------------
// Chaining Methods

// Proxy configures the proxy using a URL string or custom function and returns the Client for chaining.
func (c *Client) Proxy(proxy any) *Client {
	c.With(WithProxy(proxy))
	return c
}

// EnvProxy enables proxy settings from environment variables and returns the Client for chaining.
func (c *Client) EnvProxy() *Client {
	c.With(WithEnvProxy())
	return c
}

// TLS sets the TLS configuration for secure connections and returns the Client for chaining.
func (c *Client) TLS(cfg *tls.Config) *Client {
	c.With(WithTLS(cfg))
	return c
}

// Timeout sets the timeout for connection handshakes and write operations and returns the Client for chaining.
func (c *Client) Timeout(timeout time.Duration) *Client {
	c.With(WithTimeout(timeout))
	return c
}

// Buffers configures the read and write buffer sizes in bytes and returns the Client for chaining.
func (c *Client) Buffers(read, write int) *Client {
	c.With(WithBuffers(read, write))
	return c
}

// Subprotocols specifies supported WebSocket subprotocols and returns the Client for chaining.
func (c *Client) Subprotocols(protos ...string) *Client {
	c.With(WithSubprotocols(protos...))
	return c
}

// Compression enables or disables RFC 7692 per-message compression and returns the Client for chaining.
func (c *Client) Compression(enable bool) *Client {
	c.With(WithCompression(enable))
	return c
}

// ReadLimit sets the maximum allowed message size in bytes and returns the Client for chaining.
func (c *Client) ReadLimit(limit int64) *Client {
	c.With(WithReadLimit(limit))
	return c
}

// Retries configures reconnection behavior with attempt count and backoff timing and returns the Client for chaining.
func (c *Client) Retries(count uint, wait, maxWait time.Duration) *Client {
	c.With(WithRetries(count, wait, maxWait))
	return c
}

// KeepAlive enables periodic ping messages with a custom interval and payload and returns the Client for chaining.
func (c *Client) KeepAlive(interval time.Duration, msg []byte) *Client {
	c.With(WithKeepAlive(interval, msg))
	return c
}

// Async enables asynchronous execution of callbacks and returns the Client for chaining.
func (c *Client) Async(enable bool) *Client {
	c.With(WithAsync(enable))
	return c
}

// Header adds a single key-value pair to the handshake headers and returns the Client for chaining.
func (c *Client) Header(key, value string) *Client {
	c.With(WithHeader(key, value))
	return c
}

// Headers applies multiple headers to the handshake from a map and returns the Client for chaining.
func (c *Client) Headers(headers map[string]string) *Client {
	c.With(WithHeaders(headers))
	return c
}

// SetMessages enables a buffered channel for receiving messages and returns the Client for chaining.
func (c *Client) SetMessages(capacity int) *Client {
	c.With(WithMessages(capacity))
	return c
}

// SetErrors enables a buffered channel for error reporting and returns the Client for chaining.
func (c *Client) SetErrors(capacity int) *Client {
	c.With(WithErrors(capacity))
	return c
}

// SetContext replaces the default lifecycle context with a custom one and returns the Client for chaining.
func (c *Client) SetContext(ctx context.Context) *Client {
	c.With(WithContext(ctx))
	return c
}

// OnConnect registers a callback for successful connection and returns the Client for chaining.
func (c *Client) OnConnect(fn func(*Client)) *Client {
	c.With(OnConnect(fn))
	return c
}

// OnText registers a callback for incoming text messages and returns the Client for chaining.
func (c *Client) OnText(fn func([]byte, *Client)) *Client {
	c.With(OnText(fn))
	return c
}

// OnBinary registers a callback for incoming binary messages and returns the Client for chaining.
func (c *Client) OnBinary(fn func([]byte, *Client)) *Client {
	c.With(OnBinary(fn))
	return c
}

// OnPing registers a callback for received ping messages and returns the Client for chaining.
func (c *Client) OnPing(fn func(string, *Client)) *Client {
	c.With(OnPing(fn))
	return c
}

// OnPong registers a callback for received pong messages and returns the Client for chaining.
func (c *Client) OnPong(fn func(string, *Client)) *Client {
	c.With(OnPong(fn))
	return c
}

// OnClose registers a callback for connection closure and returns the Client for chaining.
func (c *Client) OnClose(fn func(int, string, *Client)) *Client {
	c.With(OnClose(fn))
	return c
}

// OnError registers a callback for error handling and returns the Client for chaining.
func (c *Client) OnError(fn func(error, *Client)) *Client {
	c.With(OnError(fn))
	return c
}

// OnRetry registers a callback for reconnection attempts and returns the Client for chaining.
func (c *Client) OnRetry(fn func(uint, uint, error, *Client)) *Client {
	c.With(OnRetry(fn))
	return c
}

// Debug enables or disables debug logging and returns the Client for chaining.
func (c *Client) Debug(enable bool) *Client {
	c.With(WithDebug(enable))
	return c
}
