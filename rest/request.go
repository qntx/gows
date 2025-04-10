// Package rest provides utilities for building and executing HTTP requests.
//
// It includes a Request type for configuring and sending HTTP requests with support
// for retries, forms, and file uploads, using a functional options pattern.
package rest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	json "github.com/bytedance/sonic"
	"github.com/qntx/gonet/util"
)

// --------------------------------------------------------------------------------
// Errors

var (
	// ErrMethodRequired indicates that the request method is missing.
	ErrMethodRequired = errors.New("request method is required")
	// ErrURLRequired indicates that the request URL is missing.
	ErrURLRequired = errors.New("request URL is required")
	// ErrNoBody indicates that no request body was provided when required.
	ErrNoBody = errors.New("no body provided")
	// ErrEmptyKey indicates that a key (e.g., header, query, form) is empty.
	ErrEmptyKey = errors.New("key cannot be empty")
	// ErrEmptyValue indicates that a value (e.g., file path, reader) is empty or nil.
	ErrEmptyValue = errors.New("value cannot be empty or nil")
	// ErrNilContext indicates that a nil context was provided.
	ErrNilContext = errors.New("context cannot be nil")
)

// --------------------------------------------------------------------------------
// Types

// Request represents an HTTP request with configurable settings.
//
// It supports method, URL, headers, query parameters, body content, retries, and file uploads.
type Request struct {
	method           string
	url              string
	header           http.Header
	query            url.Values
	body             any
	form             url.Values
	files            []*MultipartField
	result           any
	errResult        any
	retryCount       uint
	retryWaitTime    time.Duration
	retryMaxWaitTime time.Duration
	time             time.Time
	client           *Client
	attempt          uint
	isMultipart      bool
	ctx              context.Context
}

// MultipartField represents a field in a multipart/form-data request.
//
// It supports either a file path or a reader for file content, with one being required.
type MultipartField struct {
	Name     string    // Form field name
	FileName string    // Name of the file as sent in the request
	FilePath string    // Local path to the file (optional)
	Reader   io.Reader // Reader for file content (optional)
}

// RequestOption defines a function to configure a Request instance.
type RequestOption func(*Request) error

// --------------------------------------------------------------------------------
// Public Methods

// With applies a list of options to the Request and returns the modified instance.
//
// It supports dynamic reconfiguration using the functional options pattern.
// If an option fails, it returns the current state with an error.
//
// Example:
//
//	req, err := req.With(WithMethod("POST"), WithBody(data))
//	if err != nil {
//	    log.Println("Configuration failed:", err)
//	}
func (r *Request) With(opts ...RequestOption) (*Request, error) {
	for i, opt := range opts {
		if opt == nil {
			continue
		}

		if err := opt(r); err != nil {
			return r, fmt.Errorf("failed to apply option at index %d: %w", i, err)
		}
	}

	return r, nil
}

// Get executes a GET request with the specified URL.
//
// It sets the method to GET and sends the request immediately.
func (r *Request) Get(url string) (*Response, error) {
	return r.execute(http.MethodGet, url)
}

// Post executes a POST request with the specified URL.
//
// It sets the method to POST and sends the request immediately.
func (r *Request) Post(url string) (*Response, error) {
	return r.execute(http.MethodPost, url)
}

// Put executes a PUT request with the specified URL.
//
// It sets the method to PUT and sends the request immediately.
func (r *Request) Put(url string) (*Response, error) {
	return r.execute(http.MethodPut, url)
}

// Delete executes a DELETE request with the specified URL.
//
// It sets the method to DELETE and sends the request immediately.
func (r *Request) Delete(url string) (*Response, error) {
	return r.execute(http.MethodDelete, url)
}

// Execute sends the configured HTTP request and returns the response.
//
// It retries on server errors (5xx) based on retry settings and requires a method and URL.
//
// Example:
//
//	resp, err := req.With(WithMethod("GET"), WithURL("/users")).Execute()
//	if err != nil {
//	    log.Println("Request failed:", err)
//	}
func (r *Request) Execute() (*Response, error) {
	if r.method == "" {
		return nil, ErrMethodRequired
	}

	if r.url == "" {
		return nil, ErrURLRequired
	}

	return r.execute(r.method, r.url)
}

// --------------------------------------------------------------------------------
// Private Methods

// execute performs the HTTP request with the given method and URL.
//
// It manages retries and logs debug information if enabled.
func (r *Request) execute(method, url string) (*Response, error) {
	r.method = method
	r.url = url
	r.time = time.Now()

	for r.attempt <= r.retryCount {
		resp, err := r.do()
		r.attempt++

		if err == nil && (resp == nil || resp.StatusCode() < http.StatusInternalServerError) {
			if r.client.debug {
				r.client.logger.Debug("request succeeded with status: %d", resp.StatusCode())
			}

			return resp, nil
		}

		if r.attempt > r.retryCount || r.ctx.Err() != nil {
			return resp, err
		}

		if err := util.Wait(r.ctx, r.attempt, r.retryWaitTime, r.retryMaxWaitTime, util.DefaultJitterFactor); err != nil {
			return nil, err
		}
	}

	return nil, fmt.Errorf("exceeded retry count after %d attempts", r.retryCount)
}

// do constructs and sends the HTTP request, returning the response.
//
// It logs request and response details in debug mode.
func (r *Request) do() (*Response, error) {
	req, err := r.newRequest()
	if err != nil {
		return nil, err
	}

	if r.client.debug {
		PrintRequest(req)
	}

	resp, err := r.client.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	response, err := NewResponse(r, resp)
	if err != nil {
		return nil, err
	}

	if r.client.debug {
		PrintResponse(response)
	}

	return response, nil
}

// newRequest constructs an HTTP request from the configured settings.
//
// It builds the URL, prepares the body, and sets headers.
func (r *Request) newRequest() (*http.Request, error) {
	fullURL := buildFullURL(r.client.baseURL, r.url, r.query)

	body, err := r.prepareBody()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(r.ctx, r.method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, values := range r.header {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	return req, nil
}

// prepareBody prepares the request body based on its content type.
//
// It supports JSON, form data, or multipart form data.
func (r *Request) prepareBody() (io.Reader, error) {
	switch {
	case r.isMultipart:
		return r.prepareMultipartBody()
	case r.body != nil:
		data, err := json.Marshal(r.body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal JSON body: %w", err)
		}

		r.header.Set("Content-Type", "application/json")

		return bytes.NewReader(data), nil
	case len(r.form) > 0:
		r.header.Set("Content-Type", "application/x-www-form-urlencoded")

		return strings.NewReader(r.form.Encode()), nil
	default:
		return nil, nil
	}
}

// prepareMultipartBody constructs a multipart/form-data body for file uploads.
//
// It streams data via a pipe and handles errors asynchronously.
func (r *Request) prepareMultipartBody() (io.Reader, error) {
	pr, pw := io.Pipe()
	writer := multipart.NewWriter(pw)
	errChan := make(chan error, 1)

	go func() {
		defer pw.Close()
		defer writer.Close()

		for key, values := range r.form {
			for _, value := range values {
				if err := writer.WriteField(key, value); err != nil {
					errChan <- fmt.Errorf("failed to write form field %s: %w", key, err)

					return
				}
			}
		}

		for i, f := range r.files {
			if err := r.writeMultipartFile(writer, f); err != nil {
				errChan <- fmt.Errorf("failed to write file %d: %w", i, err)

				return
			}
		}
		errChan <- nil
	}()

	r.header.Set("Content-Type", writer.FormDataContentType())

	go func() {
		if err := <-errChan; err != nil {
			pr.CloseWithError(err)
		}
	}()

	return pr, nil
}

// writeMultipartFile writes a single file to the multipart writer.
//
// It supports file paths or readers, ensuring proper cleanup.
func (r *Request) writeMultipartFile(w *multipart.Writer, f *MultipartField) error {
	part, err := w.CreateFormFile(f.Name, f.FileName)
	if err != nil {
		return fmt.Errorf("failed to create form file %s: %w", f.Name, err)
	}

	reader := f.Reader
	if reader == nil && f.FilePath != "" {
		file, err := os.Open(f.FilePath)
		if err != nil {
			return fmt.Errorf("failed to open file %s: %w", f.FilePath, err)
		}

		defer file.Close()
		reader = file
	}

	if reader == nil {
		return nil
	}

	if _, err := io.Copy(part, reader); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	return nil
}

// --------------------------------------------------------------------------------
// Configuration Options

// WithMethod sets the HTTP method for the request (e.g., "GET", "POST").
//
// It ensures the method is uppercase for consistency.
func WithMethod(method string) RequestOption {
	return func(r *Request) error {
		r.method = strings.ToUpper(method)

		return nil
	}
}

// WithURL sets the request URL path, relative to the client's base URL.
func WithURL(url string) RequestOption {
	return func(r *Request) error {
		r.url = url

		return nil
	}
}

// WithHeader adds a single header key-value pair to the request.
//
// It ensures the key is non-empty.
func WithHeader(key, value string) RequestOption {
	return func(r *Request) error {
		if key == "" {
			return ErrEmptyKey
		}

		r.header.Set(key, value)

		return nil
	}
}

// WithHeaders adds multiple headers to the request from a map.
//
// It ensures no empty keys are present.
func WithHeaders(headers map[string]string) RequestOption {
	return func(r *Request) error {
		for k, v := range headers {
			if k == "" {
				return ErrEmptyKey
			}

			r.header.Set(k, v)
		}

		return nil
	}
}

// WithQuery adds a single query parameter to the request.
//
// It ensures the key is non-empty.
func WithQuery(key, value string) RequestOption {
	return func(r *Request) error {
		if key == "" {
			return ErrEmptyKey
		}

		r.query.Set(key, value)

		return nil
	}
}

// WithQueries adds multiple query parameters from a map.
//
// It ensures no empty keys are present.
func WithQueries(params map[string]string) RequestOption {
	return func(r *Request) error {
		for k, v := range params {
			if k == "" {
				return ErrEmptyKey
			}

			r.query.Set(k, v)
		}

		return nil
	}
}

// WithBody sets the request body to be marshaled as JSON.
func WithBody(body any) RequestOption {
	return func(r *Request) error {
		r.body = body

		return nil
	}
}

// WithForm sets form data for the request, converting values to strings.
//
// It supports basic types and JSON-marshaled complex types, validating keys.
func WithForm(data map[string]any) RequestOption {
	return func(r *Request) error {
		for key, value := range data {
			if key == "" || strings.ContainsAny(key, "\r\n") {
				return fmt.Errorf("invalid form field key: %q", key)
			}

			var strVal string
			switch v := value.(type) {
			case string:
				strVal = v
			case int, int32, int64:
				strVal = fmt.Sprintf("%d", v)
			case bool:
				strVal = strconv.FormatBool(v)
			case nil:
				continue
			default:
				jsonData, err := json.Marshal(v)
				if err != nil {
					return fmt.Errorf("failed to marshal form field %s: %w", key, err)
				}

				strVal = string(jsonData)
			}

			r.form.Set(key, strVal)
		}

		return nil
	}
}

// WithFile adds a file to the request using a file path, enabling multipart form data.
//
// It ensures name and path are non-empty.
func WithFile(name, path string) RequestOption {
	return func(r *Request) error {
		if name == "" {
			return ErrEmptyKey
		}

		if path == "" {
			return ErrEmptyValue
		}

		r.isMultipart = true
		r.files = append(r.files, &MultipartField{
			Name:     name,
			FileName: filepath.Base(path),
			FilePath: path,
		})

		return nil
	}
}

// WithFiles adds multiple files to the request from a map of names to paths.
//
// It ensures no empty names or paths are present.
func WithFiles(files map[string]string) RequestOption {
	return func(r *Request) error {
		r.isMultipart = true

		for name, path := range files {
			if name == "" {
				return ErrEmptyKey
			}

			if path == "" {
				return fmt.Errorf("file path for %q cannot be empty", name)
			}

			r.files = append(r.files, &MultipartField{
				Name:     name,
				FileName: filepath.Base(path),
				FilePath: path,
			})
		}

		return nil
	}
}

// WithFileReader adds a file to the request using an io.Reader.
//
// It ensures name and reader are non-nil.
func WithFileReader(name, filename string, reader io.Reader) RequestOption {
	return func(r *Request) error {
		if name == "" {
			return ErrEmptyKey
		}

		if reader == nil {
			return ErrEmptyValue
		}

		r.isMultipart = true
		r.files = append(r.files, &MultipartField{
			Name:     name,
			FileName: filename,
			Reader:   reader,
		})

		return nil
	}
}

// WithResult sets the target for unmarshaling a successful response body.
func WithResult(result any) RequestOption {
	return func(r *Request) error {
		r.result = result

		return nil
	}
}

// WithError sets the target for unmarshaling an error response body.
func WithError(err any) RequestOption {
	return func(r *Request) error {
		r.errResult = err

		return nil
	}
}

// WithRequestRetries configures retry settings for the request.
//
// It ensures non-negative wait times.
//
// Parameters:
//   - count: Number of retries for failed requests.
//   - waitTime: Initial wait time between retries.
//   - maxWaitTime: Maximum wait time between retries.
func WithRequestRetries(count uint, waitTime, maxWaitTime time.Duration) RequestOption {
	return func(r *Request) error {
		if waitTime < 0 {
			return errors.New("retry wait time must be non-negative")
		}

		if maxWaitTime < 0 {
			return errors.New("retry max wait time must be non-negative")
		}

		r.retryCount = count
		r.retryWaitTime = waitTime
		r.retryMaxWaitTime = maxWaitTime

		return nil
	}
}

// WithContext sets the context for the request, controlling its lifecycle.
//
// It ensures the context is non-nil.
func WithContext(ctx context.Context) RequestOption {
	return func(r *Request) error {
		if ctx == nil {
			return ErrNilContext
		}

		r.ctx = ctx

		return nil
	}
}
