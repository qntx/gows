package coder

import (
	"context"
	"time"

	"github.com/coder/websocket"
	"github.com/qntx/gows"
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

func NewConfig() *Config {
	return &Config{}
}

func DefaultConfig() *Config {
	return &Config{
		Context:     context.Background(),
		URL:         "", // Must be set by caller
		Heartbeat:   30 * time.Second,
		ReadLimit:   0,
		DialOptions: nil,
		Listening:   true,
		OnConnect:   nil,
		OnClose:     nil,
		OnMessage:   nil,
		OnError:     nil,
	}
}

func (c *Config) WithContext(ctx context.Context) *Config {
	c.Context = ctx
	return c
}

func (c *Config) WithURL(url string) *Config {
	c.URL = url
	return c
}

func (c *Config) WithHeartbeat(heartbeat time.Duration) *Config {
	c.Heartbeat = heartbeat
	return c
}

func (c *Config) WithReadLimit(limit int64) *Config {
	c.ReadLimit = limit
	return c
}

func (c *Config) WithDialOptions(opts *websocket.DialOptions) *Config {
	c.DialOptions = opts
	return c
}

func (c *Config) WithListening(listening bool) *Config {
	c.Listening = listening
	return c
}

func (c *Config) WithOnConnect(callback func()) *Config {
	c.OnConnect = callback
	return c
}

func (c *Config) WithOnClose(callback func()) *Config {
	c.OnClose = callback
	return c
}

func (c *Config) WithOnMessage(callback func(gows.MessageType, []byte)) *Config {
	c.OnMessage = callback
	return c
}

func (c *Config) WithOnError(callback func(error)) *Config {
	c.OnError = callback
	return c
}

func (c *Config) Clone() Config {
	return Config{
		Context:     c.Context,
		URL:         c.URL,
		Heartbeat:   c.Heartbeat,
		ReadLimit:   c.ReadLimit,
		DialOptions: c.DialOptions,
		Listening:   c.Listening,
		OnConnect:   c.OnConnect,
		OnClose:     c.OnClose,
		OnMessage:   c.OnMessage,
		OnError:     c.OnError,
	}
}
