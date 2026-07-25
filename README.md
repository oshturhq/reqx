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
- **Context Support** - Set a context on the client, on a single request, or both
- **Response Helpers** - Built-in status checking and JSON unmarshaling
- **Zero Dependencies** - Standard library only

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
    client, err := reqx.NewClientBuilder().
        BaseURL("https://api.example.com").
        Timeout(30 * time.Second).
        BearerAuth("your-token").
        Build()

    if err != nil {
        panic(err)
    }

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
client, err := reqx.NewClientBuilder().
    BaseURL("https://api.example.com").
    Timeout(30 * time.Second).
    Header("X-Custom-Header", "value").
    QueryParam("api_key", "your-key").
    JSONContentType().
    RetryConfig(3, 1000). // 3 retries, 1000ms backoff
    Build()
```

### Authentication

**Basic Auth:**
```go
client, err := reqx.NewClientBuilder().
    BaseURL("https://api.example.com").
    BasicAuth("username", "password").
    Build()
```

**Bearer Token:**
```go
client, err := reqx.NewClientBuilder().
    BaseURL("https://api.example.com").
    BearerAuth("your-jwt-token").
    Build()
```

**OAuth1:**
```go
client, err := reqx.NewClientBuilder().
    BaseURL("https://api.twitter.com").
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
    JSONContentType().
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
    FormURLEncodedContentType().
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
client, err := reqx.NewClientBuilder().
    BaseURL("https://api.example.com").
    RetryConfig(5, 2000). // 5 retries, 2000ms initial backoff
    Build()
```

The backoff increases linearly with jitter applied: `backoffMs * (attempt + 1)`, randomised
within the upper half of that window. A `Retry-After` response header takes precedence (capped
at 30s). Backoff observes the request's context, so cancelling stops the retry schedule
immediately.

Retries only happen when the request body can be produced again. Bodies set via `Body()` — JSON,
form, string, `[]byte` — and multipart forms built from in-memory files are buffered and replayed
in full. A body set via `BodyReader()`, or a multipart form containing `AddFileReader()`, cannot
be rewound, so the request is **not** retried and the original response is returned instead of
being resent with an empty body.

### Context

A context can be set on the client, on a single request, or both:

```go
// Applies to every request made with this client
client, err := reqx.NewClientBuilder().
    BaseURL("https://api.example.com").
    Context(ctx).
    Build()

// Applies to this request only
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

resp, err := client.Get("/slow-endpoint").
    Context(ctx).
    Do(&result, &apiError)
```

Each request inherits the client's context, and `Context()` on the request overrides it for that
request alone. Cancelling either one aborts the in-flight request and stops any pending retry
backoff.

### Per-Request Customization

You can override client settings per request:

```go
resp, err := client.Get("/special-endpoint").
    Context(ctx).
    Header("X-Request-ID", "abc123").
    QueryParam("special", "true").
    JSONContentType().
    Do(&result, &apiError)
```

## API Reference

### ClientBuilder Methods

| Method | Description |
|--------|-------------|
| `Context(ctx)` | Set the default context inherited by every request |
| `BaseURL(url)` | Set base URL |
| `Timeout(duration)` | Set request timeout |
| `Header(key, value)` | Add default header |
| `QueryParam(key, value)` | Add default query parameter |
| `BasicAuth(user, pass)` | Set Basic authentication |
| `BearerAuth(token)` | Set Bearer token authentication |
| `OAuth1(...)` | Set OAuth1 authentication |
| `JSONContentType()` | Set default Content-Type to JSON |
| `FormURLEncodedContentType()` | Set default Content-Type to form-urlencoded |
| `MultipartFormContentType()` | Set default Content-Type to multipart/form-data |
| `RetryConfig(maxRetries, backoffMs)` | Configure retry behavior |
| `MaxResponseBytes(limit)` | Cap buffered response size (default 32 MiB; `0` disables) |
| `MaxRedirects(n)` | Redirect limit (default 10; `0` returns the redirect response) |
| `Build()` | Validate the configuration and build the Client; returns `(*Client, error)` |

### RequestBuilder Methods

| Method | Description |
|--------|-------------|
| `Context(ctx)` | Set the context for this request, overriding the client's |
| `Path(path)` | Set request path |
| `QueryParam(key, value)` | Add query parameter |
| `Header(key, value)` | Add header |
| `Body(data)` | Set request body (auto-serialized) |
| `BodyReader(reader)` | Set request body from io.Reader |
| `JSONContentType()` | Set Content-Type to JSON |
| `FormURLEncodedContentType()` | Set Content-Type to form-urlencoded |
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

## Security Behaviour

The client enforces a few rules that are worth knowing about, because they turn what used to be
silent misbehaviour into explicit errors.

**Requests stay on the configured host.** `Path()` is resolved against `BaseUrl` rather than
concatenated with it. A path that carries a scheme, a host, or userinfo — `//other.host/x`,
`https://other.host/x`, `@other.host/x` — is rejected with `ErrURLHostMismatch`, and dot segments
that would escape the base URL's path prefix are rejected with `ErrURLPathEscape`. Without this,
a path assembled from untrusted input could relocate the request, taking the client's credentials
with it.

**Credentials do not follow cross-host redirects.** Every header you set is stripped when a
redirect leaves the original host, and redirects that downgrade HTTPS to a plaintext scheme are
refused with `ErrRedirectDowngrade`. Same-host redirects keep your headers.

**Configuration errors surface at `Build()`.** `Build()` returns `(*Client, error)`, so a
malformed or relative `BaseURL` fails with `ErrInvalidURL` and configuring `OAuth1()` alongside
`BasicAuth()`, `BearerAuth()`, or an explicit `Authorization` header fails with `ErrAuthConflict`
— at construction, not on the first request. Within a single request, the OAuth1 signature is
applied last and always wins.

**Clients are safe for concurrent use.** A `Client` may be shared across goroutines; each request
gets its own copy of the client's query parameters, so per-request values never leak into later
requests.

**Response bodies are bounded.** `Do` and `DoRaw` buffer at most `MaxResponseBytes` (32 MiB by
default) and return `ErrResponseTooLarge` beyond that. `DoStream` is unaffected — the caller owns
the reader.

**Decode failures are returned.** If the response body cannot be unmarshaled into your target,
`Do` returns an error wrapping `ErrDecodeResponse` along with the response, instead of handing
back a zero-valued target and a nil error.

### Errors

| Error | Meaning |
|-------|---------|
| `ErrInvalidBody` | Body type does not match the configured content type, or a stream was reused |
| `ErrMaxRetriesExceeded` | Retry budget exhausted |
| `ErrInvalidURL` | Base URL or path could not be parsed, or a relative path was used with no base URL |
| `ErrURLHostMismatch` | Path would move the request off the base URL's host |
| `ErrURLPathEscape` | Path escapes the base URL's path prefix |
| `ErrAuthConflict` | More than one authentication mechanism configured |
| `ErrResponseTooLarge` | Response exceeded `MaxResponseBytes` |
| `ErrTooManyRedirects` | Redirect limit reached |
| `ErrRedirectDowngrade` | Redirect would move an HTTPS request to plaintext |
| `ErrDecodeResponse` | Response body could not be unmarshaled into the target |
