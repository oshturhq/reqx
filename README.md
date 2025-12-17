# reqx

A fluent, builder-pattern HTTP client for Go with built-in retry mechanism, OAuth1 support, and multipart form handling.

## Installation

```bash
go get github.com/oshturhq/reqx
```

## Features

- **Fluent Builder Pattern** - Chain methods for clean, readable code
- **Multiple Authentication Methods** - Basic Auth, Bearer Token, OAuth1 (HMAC-SHA1)
- **Automatic Retry** - Configurable retry with backoff for timeouts and 5xx errors
- **Multiple Content Types** - JSON, Form URL Encoded, Multipart Form
- **Multipart Form Builder** - Easy file uploads with streaming support
- **Context Support** - Full context.Context integration
- **Response Helpers** - Built-in status checking and JSON unmarshaling

## Quick Start

```go
package main

import (
    "fmt"
    "time"
    "github.com/oshturhq/reqx"
)

func main() {
    // Create a client
    client := reqx.NewClientBuilder().
        BaseUrl("https://api.example.com").
        Timeout(30 * time.Second).
        BearerAuth("your-token").
        Build()

    // Make a GET request
    var result map[string]any
    var errResp map[string]any

    resp, err := client.Get("/users").
        QueryParam("page", "1").
        Do(&result, &errResp)

    if err != nil {
        panic(err)
    }

    if resp.IsSuccess() {
        fmt.Println("Success:", result)
    }
}
```

## Usage

### Creating a Client

```go
client := reqx.NewClientBuilder().
    BaseUrl("https://api.example.com").
    Timeout(30 * time.Second).
    Header("X-Custom-Header", "value").
    QueryParam("api_key", "your-key").
    JsonContentType().
    RetryConfig(3, 1000). // 3 retries, 1000ms backoff
    Build()
```

### Authentication

**Basic Auth:**
```go
client := reqx.NewClientBuilder().
    BaseUrl("https://api.example.com").
    BasicAuth("username", "password").
    Build()
```

**Bearer Token:**
```go
client := reqx.NewClientBuilder().
    BaseUrl("https://api.example.com").
    BearerAuth("your-jwt-token").
    Build()
```

**OAuth1:**
```go
client := reqx.NewClientBuilder().
    BaseUrl("https://api.twitter.com").
    OAuth1(
        "consumer-key",
        "consumer-secret",
        "access-token",
        "access-token-secret",
    ).
    Build()
```

### Making Requests

**GET Request:**
```go
var users []User
var apiError ErrorResponse

resp, err := client.Get("/users").
    QueryParam("limit", "10").
    QueryParam("offset", "0").
    Do(&users, &apiError)
```

**POST Request with JSON Body:**
```go
type CreateUserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

var createdUser User
var apiError ErrorResponse

resp, err := client.Post("/users").
    JsonContentType().
    Body(CreateUserRequest{
        Name:  "John Doe",
        Email: "john@example.com",
    }).
    Do(&createdUser, &apiError)
```

**PUT Request:**
```go
resp, err := client.Put("/users/123").
    Body(updateData).
    Do(&updatedUser, &apiError)
```

**DELETE Request:**
```go
resp, err := client.Delete("/users/123").
    Do(nil, &apiError)
```

**PATCH Request:**
```go
resp, err := client.Patch("/users/123").
    Body(patchData).
    Do(&patchedUser, &apiError)
```

### Multipart Form / File Upload

```go
resp, err := client.Post("/upload").
    MultipartFormBody().
    AddField("description", "My file").
    AddFile("file", "document.pdf", fileBytes).
    Do(&uploadResult, &apiError)
```

**With io.Reader:**
```go
file, _ := os.Open("large-file.zip")
defer file.Close()

resp, err := client.Post("/upload").
    MultipartFormBody().
    AddField("name", "large-file").
    AddFileReader("file", "large-file.zip", file).
    Do(&result, &apiError)
```

### Form URL Encoded

```go
import "net/url"

formData := url.Values{}
formData.Set("username", "john")
formData.Set("password", "secret")

resp, err := client.Post("/login").
    FormUrlencodedContentType().
    Body(formData).
    Do(&loginResult, &apiError)
```

### Raw Response

If you don't want automatic JSON unmarshaling:

```go
resp, err := client.Get("/data").DoRaw()
if err != nil {
    panic(err)
}

fmt.Println("Status:", resp.Status)
fmt.Println("Body:", string(resp.Body))
fmt.Println("Headers:", resp.Headers)
```

### Streaming Response

For large responses or server-sent events:

```go
resp, err := client.Get("/stream").DoStream()
if err != nil {
    panic(err)
}
defer resp.BodyReader.Close()

// Read from resp.BodyReader as needed
```

### Response Handling

```go
resp, err := client.Get("/users").Do(&users, &apiError)

if resp.IsSuccess() {
    // 2xx status code
    fmt.Println("Success!")
}

if resp.IsError() {
    // 4xx status code
    fmt.Println("Client error:", apiError)
}

if resp.IsServerError() {
    // 5xx status code
    fmt.Println("Server error")
}

// Access raw response
fmt.Println("Status Code:", resp.Status)
fmt.Println("Headers:", resp.Headers)
fmt.Println("Raw Body:", string(resp.Body))
```

### Retry Configuration

The client automatically retries on:
- Network timeouts
- DNS errors
- HTTP 5xx errors
- HTTP 429 (Too Many Requests)

```go
client := reqx.NewClientBuilder().
    BaseUrl("https://api.example.com").
    RetryConfig(5, 2000). // 5 retries, 2000ms initial backoff
    Build()
```

The backoff increases linearly: `backoffMs * (attempt + 1)`

### Per-Request Customization

You can override client settings per request:

```go
resp, err := client.Get("/special-endpoint").
    Header("X-Request-ID", "abc123").
    QueryParam("special", "true").
    JsonContentType().
    Do(&result, &apiError)
```

## API Reference

### ClientBuilder Methods

| Method | Description |
|--------|-------------|
| `Context(ctx)` | Set context for all requests |
| `BaseUrl(url)` | Set base URL |
| `Timeout(duration)` | Set request timeout |
| `Header(key, value)` | Add default header |
| `QueryParam(key, value)` | Add default query parameter |
| `BasicAuth(user, pass)` | Set Basic authentication |
| `BearerAuth(token)` | Set Bearer token authentication |
| `OAuth1(...)` | Set OAuth1 authentication |
| `JsonContentType()` | Set default Content-Type to JSON |
| `FormUrlencodedContentType()` | Set default Content-Type to form-urlencoded |
| `MultipartFormContentType()` | Set default Content-Type to multipart/form-data |
| `RetryConfig(maxRetries, backoffMs)` | Configure retry behavior |
| `Build()` | Build the Client |

### RequestBuilder Methods

| Method | Description |
|--------|-------------|
| `Path(path)` | Set request path |
| `QueryParam(key, value)` | Add query parameter |
| `Header(key, value)` | Add header |
| `Body(data)` | Set request body (auto-serialized) |
| `BodyReader(reader)` | Set request body from io.Reader |
| `JsonContentType()` | Set Content-Type to JSON |
| `FormUrlencodedContentType()` | Set Content-Type to form-urlencoded |
| `MultipartFormBody()` | Start multipart form builder |
| `Do(success, error)` | Execute with JSON unmarshaling |
| `DoRaw()` | Execute and return raw response |
| `DoStream()` | Execute and return streaming response |

### Response Methods

| Method | Description |
|--------|-------------|
| `IsSuccess()` | Returns true for 2xx status codes |
| `IsError()` | Returns true for 4xx status codes |
| `IsServerError()` | Returns true for 5xx status codes |
