package reqx

import (
	"context"
	"encoding/base64"
	"errors"
	"maps"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type callerHeadersKey struct{}

type ClientBuilder struct {
	ctx              context.Context
	baseURL          string
	timeout          time.Duration
	queryParams      map[string]string
	headers          map[string]string
	contentType      ContentType
	oauth1           *OAuth1Config
	retryConfig      *RetryConfig
	maxResponseBytes int64
	maxRedirects     int
}

func NewClientBuilder() *ClientBuilder {
	return &ClientBuilder{
		ctx:         context.Background(),
		queryParams: make(map[string]string),
		headers:     make(map[string]string),
		retryConfig: &RetryConfig{
			MaxRetries: 3,
			BackoffMs:  1000,
		},
		maxResponseBytes: DefaultMaxResponseBytes,
		maxRedirects:     DefaultMaxRedirects,
	}
}

func (b *ClientBuilder) Context(ctx context.Context) *ClientBuilder {
	b.ctx = ctx
	return b
}

func (b *ClientBuilder) BaseURL(baseURL string) *ClientBuilder {
	b.baseURL = baseURL
	return b
}

func (b *ClientBuilder) JSONContentType() *ClientBuilder {
	b.contentType = ContentTypeJSON
	return b
}

func (b *ClientBuilder) FormURLEncodedContentType() *ClientBuilder {
	b.contentType = ContentTypeFormURLEncoded
	return b
}

func (b *ClientBuilder) MultipartFormContentType() *ClientBuilder {
	b.contentType = ContentTypeMultipartForm
	return b
}

func (b *ClientBuilder) BasicAuth(username string, password string) *ClientBuilder {
	var credBuilder strings.Builder
	credBuilder.WriteString(username)
	credBuilder.WriteString(":")
	credBuilder.WriteString(password)
	credentials := credBuilder.String()

	var authBuilder strings.Builder
	authBuilder.WriteString("Basic ")
	authBuilder.WriteString(base64.StdEncoding.EncodeToString([]byte(credentials)))

	b.headers["Authorization"] = authBuilder.String()

	return b
}

func (b *ClientBuilder) BearerAuth(token string) *ClientBuilder {
	var builder strings.Builder
	builder.WriteString("Bearer ")
	builder.WriteString(token)

	b.headers["Authorization"] = builder.String()

	return b
}

func (b *ClientBuilder) OAuth1(consumerKey, consumerSecret, accessToken, accessTokenSecret string) *ClientBuilder {
	b.oauth1 = &OAuth1Config{
		ConsumerKey:       consumerKey,
		ConsumerSecret:    consumerSecret,
		AccessToken:       accessToken,
		AccessTokenSecret: accessTokenSecret,
	}
	return b
}

func (b *ClientBuilder) Timeout(timeout time.Duration) *ClientBuilder {
	b.timeout = timeout
	return b
}

func (b *ClientBuilder) QueryParam(key string, value string) *ClientBuilder {
	b.queryParams[key] = value
	return b
}

func (b *ClientBuilder) Header(key, value string) *ClientBuilder {
	b.headers[key] = value
	return b
}

func (b *ClientBuilder) RetryConfig(maxRetries int, backoffMs int) *ClientBuilder {
	b.retryConfig = &RetryConfig{
		MaxRetries: maxRetries,
		BackoffMs:  backoffMs,
	}
	return b
}

func (b *ClientBuilder) MaxResponseBytes(limit int64) *ClientBuilder {
	b.maxResponseBytes = limit
	return b
}

func (b *ClientBuilder) MaxRedirects(maxRedirects int) *ClientBuilder {
	b.maxRedirects = maxRedirects
	return b
}

func (b *ClientBuilder) Build() (*Client, error) {
	var baseURL *url.URL

	if b.baseURL != "" {
		parsed, err := url.Parse(b.baseURL)
		if err != nil {
			return nil, errors.Join(ErrInvalidURL, err)
		}
		if parsed.Scheme == "" || parsed.Host == "" {
			return nil, ErrInvalidURL
		}
		baseURL = parsed
	}

	if b.oauth1 != nil && b.headers["Authorization"] != "" {
		return nil, ErrAuthConflict
	}

	queryParams := make(map[string]string, len(b.queryParams))
	maps.Copy(queryParams, b.queryParams)
	headers := make(map[string]string, len(b.headers))
	maps.Copy(headers, b.headers)

	return &Client{
		ctx:     b.ctx,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout:       b.timeout,
			CheckRedirect: redirectPolicy(b.maxRedirects),
		},
		queryParams:      queryParams,
		headers:          headers,
		contentType:      b.contentType,
		oauth1:           b.oauth1,
		retryConfig:      b.retryConfig,
		maxResponseBytes: b.maxResponseBytes,
	}, nil
}

func redirectPolicy(maxRedirects int) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if maxRedirects <= 0 {
			return http.ErrUseLastResponse
		}

		if len(via) >= maxRedirects {
			return ErrTooManyRedirects
		}

		initial := via[0].URL

		if initial.Scheme == "https" && req.URL.Scheme != "https" {
			return ErrRedirectDowngrade
		}

		if req.URL.Host != initial.Host {
			if names, ok := req.Context().Value(callerHeadersKey{}).([]string); ok {
				for _, name := range names {
					req.Header.Del(name)
				}
			}
			req.Header.Del("Authorization")
		}

		return nil
	}
}
