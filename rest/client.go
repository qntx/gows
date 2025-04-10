// Package rest provides a configurable HTTP client for making RESTful requests.
//
// It supports retries, timeouts, and proxies using a functional options pattern
// for flexible configuration.
package rest

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// --------------------------------------------------------------------------------
// Constants

const (
	// DefaultTimeout is the default HTTP request timeout.
	DefaultTimeout = 10 * time.Second
	// DefaultRetryCount is the default number of retries for failed requests.
	DefaultRetryCount = 0
	// DefaultRetryWaitTime is the initial wait time between retries.
	DefaultRetryWaitTime = 100 * time.Millisecond
	// DefaultRetryMaxWaitTime is the maximum wait time between retries.
	DefaultRetryMaxWaitTime = 2 * time.Second
)

// --------------------------------------------------------------------------------
// Errors

var (
	// ErrInvalidTransport indicates that the HTTP transport is not compatible.
	ErrInvalidTransport = errors.New("underlying transport is not an http.Transport")
)

// --------------------------------------------------------------------------------
// Types

// Client manages HTTP requests with configurable settings such as retries and timeouts.
type Client struct {
	baseURL          string
	debug            bool
	httpClient       *http.Client
	retryCount       uint
	retryWaitTime    time.Duration
	retryMaxWaitTime time.Duration
}

// Option defines a function to configure a Client instance.
type Option func(*Client) error

// --------------------------------------------------------------------------------
// Constructors

// New creates a new Client with default settings and applies the provided options.
//
// It trims the base URL for consistency. If an error occurs during option application,
// it returns the error with context.
//
// Example:
//
//	client, err := New("https://api.example.com",
//	    WithTimeout(15*time.Second),
//	    WithRetries(3, 200*time.Millisecond, 5*time.Second),
//	)
//	if err != nil {
//	    log.Fatal(err)
//	}
func New(baseURL string, opts ...Option) (*Client, error) {
	c := &Client{
		baseURL:          strings.TrimRight(baseURL, "/"),
		httpClient:       &http.Client{Timeout: DefaultTimeout},
		retryCount:       DefaultRetryCount,
		retryWaitTime:    DefaultRetryWaitTime,
		retryMaxWaitTime: DefaultRetryMaxWaitTime,
	}

	c.With(opts...)

	return c, nil
}

// --------------------------------------------------------------------------------
// Public Methods

// With applies a list of options to an existing Client and returns the modified instance.
//
// It supports dynamic reconfiguration of the Client. If an option fails, it returns
// the current state with an error.
//
// Example:
//
//	client, err := client.With(WithDebug(true), WithProxy("http://proxy:8080"))
//	if err != nil {
//	    log.Println("Configuration failed:", err)
//	}
func (c *Client) With(opts ...Option) (*Client, error) {
	for i, opt := range opts {
		if opt == nil {
			continue
		}

		if err := opt(c); err != nil {
			return c, fmt.Errorf("failed to apply option at index %d: %w", i, err)
		}
	}

	return c, nil
}

// R creates a new Request with the Client's configuration.
//
// It initializes a Request with inherited settings and applies any provided options.
//
// Example:
//
//	req, err := client.R(WithMethod("POST"), WithPath("/users"))
//	if err != nil {
//	    log.Println("Request creation failed:", err)
//	}
func (c *Client) R(opts ...RequestOption) (*Request, error) {
	r := &Request{
		header:           make(http.Header),
		query:            make(url.Values),
		form:             make(url.Values),
		retryCount:       c.retryCount,
		retryWaitTime:    c.retryWaitTime,
		retryMaxWaitTime: c.retryMaxWaitTime,
		client:           c,
		ctx:              context.Background(),
	}

	for _, opt := range opts {
		if err := opt(r); err != nil {
			return nil, fmt.Errorf("failed to apply request option: %w", err)
		}
	}

	return r, nil
}

// HTTPTransport returns the underlying HTTP transport for advanced configuration.
//
// It returns an error if the transport is not an *http.Transport.
func (c *Client) HTTPTransport() (*http.Transport, error) {
	transport, ok := c.httpClient.Transport.(*http.Transport)
	if !ok {
		return nil, ErrInvalidTransport
	}

	return transport, nil
}

// --------------------------------------------------------------------------------
// Configuration Options

// WithBaseURL sets the base URL for the Client.
//
// It trims trailing slashes to ensure consistency.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) error {
		c.baseURL = strings.TrimRight(baseURL, "/")
		return nil
	}
}

// WithDebug enables or disables debug mode for the Client.
func WithDebug(debug bool) Option {
	return func(c *Client) error {
		c.debug = debug
		return nil
	}
}

// WithTimeout sets the HTTP request timeout for the Client.
//
// It enforces a non-negative timeout, falling back to the default if invalid.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) error {
		if timeout < 0 {
			timeout = DefaultTimeout
		}
		c.httpClient.Timeout = timeout
		return nil
	}
}

// WithProxy configures the Client to use a specified proxy URL.
//
// It validates the proxy URL and updates the transport accordingly.
func WithProxy(proxyURL string) Option {
	return func(c *Client) error {
		pURL, err := url.Parse(proxyURL)
		if err != nil {
			return fmt.Errorf("invalid proxy URL: %w", err)
		}
		return setTransportProxy(c, http.ProxyURL(pURL))
	}
}

// WithEnvProxy configures the Client to use proxy settings from the environment.
func WithEnvProxy() Option {
	return func(c *Client) error {
		return setTransportProxy(c, http.ProxyFromEnvironment)
	}
}

// WithRetries configures retry settings for the Client.
//
// It ensures non-negative wait times, using defaults if invalid.
//
// Parameters:
//   - count: Number of retries for failed requests.
//   - waitTime: Initial wait time between retries.
//   - maxWaitTime: Maximum wait time between retries.
func WithRetries(count uint, waitTime, maxWaitTime time.Duration) Option {
	return func(c *Client) error {
		if waitTime < 0 {
			waitTime = DefaultRetryWaitTime
		}
		if maxWaitTime < 0 {
			maxWaitTime = DefaultRetryMaxWaitTime
		}
		c.retryCount = count
		c.retryWaitTime = waitTime
		c.retryMaxWaitTime = maxWaitTime
		return nil
	}
}

// --------------------------------------------------------------------------------
// Private Helpers

// setTransportProxy configures the HTTP transport with a proxy function.
//
// It initializes the transport if nil and ensures type compatibility.
func setTransportProxy(c *Client, proxy func(*http.Request) (*url.URL, error)) error {
	if c.httpClient.Transport == nil {
		c.httpClient.Transport = &http.Transport{}
	}
	transport, ok := c.httpClient.Transport.(*http.Transport)
	if !ok {
		return ErrInvalidTransport
	}
	transport.Proxy = proxy
	return nil
}
