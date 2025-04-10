// Package rest provides utilities for handling HTTP responses in a RESTful client.
//
// It includes a Response type for managing raw HTTP responses, body data, and JSON unmarshaling.
package rest

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	json "github.com/bytedance/sonic"
)

// --------------------------------------------------------------------------------
// Errors

var (
	// ErrNilRequest indicates that a nil request was provided.
	ErrNilRequest = errors.New("request cannot be nil")
	// ErrNilResponse indicates that a nil raw response was provided.
	ErrNilResponse = errors.New("raw response cannot be nil")
)

// --------------------------------------------------------------------------------
// Types

// Response encapsulates the result of an executed HTTP request.
//
// It provides access to the raw response, body, and unmarshaled data with error handling.
type Response struct {
	request     *Request
	rawResponse *http.Response
	body        []byte
	err         error
	receivedAt  time.Time
}

// --------------------------------------------------------------------------------
// Constructors

// NewResponse creates a new Response from a Request and raw HTTP response.
//
// It reads and caches the response body, closing the raw response body afterward.
// If the response is JSON, it attempts to unmarshal it into the Request's result or errResult fields.
//
// Returns an error if the request or response is nil, or if body reading/unmarshaling fails.
//
// Example:
//
//	resp, err := NewResponse(req, rawResp)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(resp.Status())
func NewResponse(req *Request, raw *http.Response) (*Response, error) {
	if req == nil {
		return nil, ErrNilRequest
	}

	if raw == nil {
		return nil, ErrNilResponse
	}

	resp := &Response{
		request:     req,
		rawResponse: raw,
		receivedAt:  time.Now(),
	}

	if err := resp.readBody(); err != nil {
		return nil, err
	}

	return resp, nil
}

// --------------------------------------------------------------------------------
// Public Methods

// Status returns the HTTP status string (e.g., "200 OK").
//
// It provides the full status text from the raw response.
func (r *Response) Status() string {
	return r.rawResponse.Status
}

// StatusCode returns the HTTP status code (e.g., 200).
//
// It provides the numeric status code from the raw response.
func (r *Response) StatusCode() int {
	return r.rawResponse.StatusCode
}

// Header returns the HTTP response headers.
//
// It provides a copy of the raw response headers.
func (r *Response) Header() http.Header {
	return r.rawResponse.Header
}

// Cookies returns the cookies set in the HTTP response.
//
// It provides a slice of cookies from the raw response.
func (r *Response) Cookies() []*http.Cookie {
	return r.rawResponse.Cookies()
}

// String returns the response body as a string.
//
// It converts the cached body bytes to a string for convenience.
func (r *Response) String() string {
	return string(r.body)
}

// Bytes returns the response body as a byte slice.
//
// It provides direct access to the cached body data.
func (r *Response) Bytes() []byte {
	return r.body
}

// Error returns any error encountered during response processing.
//
// It returns nil if no error occurred while reading or unmarshaling the response.
func (r *Response) Error() error {
	return r.err
}

// Duration returns the time elapsed from request start to response receipt.
//
// It calculates the duration based on the request's start time and response receipt time.
func (r *Response) Duration() time.Duration {
	return r.receivedAt.Sub(r.request.time)
}

// ReceivedAt returns the time the response was received.
//
// It provides the exact timestamp when the response was fully processed.
func (r *Response) ReceivedAt() time.Time {
	return r.receivedAt
}

// IsSuccess checks if the status code is in the 2xx range (200-299).
//
// It indicates whether the request was successful based on HTTP conventions.
func (r *Response) IsSuccess() bool {
	return r.StatusCode() >= http.StatusOK && r.StatusCode() < http.StatusMultipleChoices
}

// IsError checks if the status code is in the 4xx or 5xx range (>= 400).
//
// It indicates whether the request resulted in a client or server error.
func (r *Response) IsError() bool {
	return r.StatusCode() >= http.StatusBadRequest
}

// Result returns the unmarshaled success data, if set.
//
// It provides the data unmarshaled from a successful JSON response into the Request's result field.
func (r *Response) Result() any {
	return r.request.result
}

// ErrorResult returns the unmarshaled error data, if set.
//
// It provides the data unmarshaled from an error JSON response into the Request's errResult field.
func (r *Response) ErrorResult() any {
	return r.request.errResult
}

// --------------------------------------------------------------------------------
// Private Methods

// readBody reads and caches the response body, unmarshaling JSON if applicable.
//
// It closes the raw response body after reading and logs errors via the client's logger.
// For JSON responses, it unmarshals into the appropriate result field based on the status code.
func (r *Response) readBody() error {
	if r.rawResponse.Body == nil {
		return nil
	}

	defer r.rawResponse.Body.Close()

	body, err := io.ReadAll(r.rawResponse.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %w", err)
	}

	r.body = body

	// Skip JSON unmarshaling if content type isn't JSON
	if !strings.Contains(r.Header().Get("Content-Type"), "application/json") {
		return nil
	}

	if r.IsSuccess() && r.request.result != nil {
		if err := json.Unmarshal(body, r.request.result); err != nil {
			return fmt.Errorf("failed to unmarshal success response: %w", err)
		}

		return nil
	}

	if r.IsError() && r.request.errResult != nil {
		if err := json.Unmarshal(body, r.request.errResult); err != nil {
			return fmt.Errorf("failed to unmarshal error response: %w", err)
		}
	}

	return nil
}
