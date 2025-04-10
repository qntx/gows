# REST Client

A lightweight Go HTTP client for REST API requests.

## Usage

### Common Request Types

- **GET**: `req.Get(url)`
- **POST**: `req.Post(url)`
- **PUT**: `req.Put(url)`
- **DELETE**: `req.Delete(url)`

### Parameter Combinations

1. **JSON**
   `WithBody(any)` → `application/json`

   ```go
   req, _ := client.R(rest.WithBody(map[string]string{"key": "value"}))
   resp, _ := req.Post("/endpoint")
   ```

2. **Form Data**
   `WithForm(map[string]any)` → `application/x-www-form-urlencoded`

   ```go
   req, _ := client.R(rest.WithForm(map[string]string{"key": "value"}))
   resp, _ := req.Post("/endpoint")
   ```

3. **File Upload**
   `WithFile(name, path)` → `multipart/form-data`
   Combines with `WithForm`

   ```go
   req, _ := client.R(rest.WithFile("file", "file.txt"), rest.WithForm(map[string]string{"key": "value"}))
   resp, _ := req.Post("/endpoint")
   ```

4. **Query Parameters**
   `WithQuery(key, value)` → Appended to URL
   Works with any request type

   ```go
   req, _ := client.R(rest.WithQuery("key", "value"), rest.WithBody(map[string]string{"data": "test"}))
   resp, _ := req.Get("/endpoint")
   ```

### Notes

- **Priority**: Files > JSON > Form
- **Response**: Use `resp.String()`, `resp.StatusCode()`, `resp.Result()` (with `WithResult`)
