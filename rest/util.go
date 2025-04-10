package rest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/fatih/color"
)

// buildFullURL constructs the full URL with query parameters.
//
// It combines the base URL, path, and encoded query string.
func buildFullURL(baseURL, path string, query url.Values) string {
	var fullURL strings.Builder

	fullURL.WriteString(baseURL)
	fullURL.WriteString(path)

	if len(query) > 0 {
		fullURL.WriteByte('?')
		fullURL.WriteString(query.Encode())
	}

	return fullURL.String()
}

// PrintRequest prints HTTP request details with colorized formatting.
func PrintRequest(req *http.Request) {
	cyanBold := color.New(color.FgHiCyan, color.Bold)
	_, _ = cyanBold.Println("╔══════════════════ HTTP Request ══════════════════╗")

	green := color.New(color.FgGreen)
	_, _ = green.Print("  📜 Method: ")

	fmt.Println(req.Method)

	_, _ = green.Print("  🌐 URL: ")

	fmt.Println(req.URL.String())

	_, _ = green.Print("  📋 Headers: ")

	printIndentedHeaders(req.Header)

	if req.Body != nil {
		_, _ = color.New(color.FgHiMagenta, color.Bold).Println("  📦 Request Body:")

		printBody(req.Header.Get("Content-Type"), req.Body, func(r io.Reader) {
			if br, ok := r.(*bytes.Reader); ok {
				req.Body = io.NopCloser(br)
			} else {
				fmt.Print(color.YellowString("  ⚠ Warning: Failed to restore request body\n"))
			}
		})
	}

	_, _ = cyanBold.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println()
}

// PrintResponse prints HTTP response details with colorized formatting.
func PrintResponse(resp *Response) {
	cyanBold := color.New(color.FgHiCyan, color.Bold)
	_, _ = cyanBold.Println("╔══════════════════ HTTP Response ═════════════════╗")

	green := color.New(color.FgGreen)
	_, _ = green.Print("  ✅ Status: ")

	fmt.Println(resp.Status())

	_, _ = green.Print("  📋 Headers: ")

	printIndentedHeaders(resp.Header())

	if body := resp.Bytes(); len(body) > 0 {
		_, _ = color.New(color.FgHiMagenta, color.Bold).Println("  📦 Response Body:")

		printBody(resp.Header().Get("Content-Type"), bytes.NewReader(body), nil)
	}

	yellow := color.New(color.FgHiYellow)
	_, _ = yellow.Print("  ⏱ Duration:")

	fmt.Println(resp.Duration())

	_, _ = cyanBold.Println("╚══════════════════════════════════════════════════╝")
	fmt.Println()
}

// printIndentedHeaders prints HTTP headers with consistent indentation.
func printIndentedHeaders(headers http.Header) {
	if len(headers) == 0 {
		fmt.Println("<empty>")

		return
	}

	fmt.Println()

	for key, values := range headers {
		fmt.Printf("    %-20s : %s\n", key, strings.Join(values, ", "))
	}
}

// printBody prints the HTTP body, formatted by Content-Type, with optional restoration.
func printBody(contentType string, body io.Reader, restore func(io.Reader)) {
	bodyBytes, err := io.ReadAll(body)
	if err != nil {
		fmt.Print(color.RedString("    ⛔ Error: "))
		fmt.Printf("Failed to read body: %v\n", err)

		return
	}

	if restore != nil {
		restore(bytes.NewReader(bodyBytes))
	}

	if len(bodyBytes) == 0 {
		fmt.Println("<empty>")

		return
	}

	if strings.Contains(contentType, "application/json") {
		var prettyJSON bytes.Buffer
		if json.Indent(&prettyJSON, bodyBytes, "    ", "  ") != nil {
			fmt.Printf("<unformattable JSON>: %s\n", bodyBytes)

			return
		}

		fmt.Println("    " + prettyJSON.String())
	} else {
		lines := strings.Split(string(bodyBytes), "\n")

		fmt.Println()

		for _, line := range lines {
			if line != "" {
				fmt.Printf("    %s\n", line)
			}
		}
	}
}
