package reqx

import (
	"context"
	"encoding/base64"
	"net/http"
	"strings"
	"time"
)

type ClientBuilder struct {
	context     context.Context
	baseUrl     string
	timeout     time.Duration
	queryParams map[string]string
	headers     map[string]string
	contentType ContentType
	oauth1      *OAuth1Config
	retryConfig *RetryConfig
}

func NewClientBuilder() *ClientBuilder {
	return &ClientBuilder{
		context:     context.Background(),
		queryParams: make(map[string]string),
		headers:     make(map[string]string),
		retryConfig: &RetryConfig{
			MaxRetries: 3,
			BackoffMs:  1000,
		},
	}
}

func (h *ClientBuilder) Context(ctx context.Context) *ClientBuilder {
	h.context = ctx
	return h
}

func (h *ClientBuilder) BaseUrl(baseUrl string) *ClientBuilder {
	h.baseUrl = baseUrl
	return h
}

func (c *ClientBuilder) JsonContentType() *ClientBuilder {
	c.contentType = ContentTypeJSON
	return c
}

func (c *ClientBuilder) FormUrlencodedContentType() *ClientBuilder {
	c.contentType = ContentTypeFormUrlencoded
	return c
}

func (c *ClientBuilder) MultipartFormContentType() *ClientBuilder {
	c.contentType = ContentTypeMultipartForm
	return c
}

func (h *ClientBuilder) BasicAuth(username string, password string) *ClientBuilder {
	var credBuilder strings.Builder
	credBuilder.WriteString(username)
	credBuilder.WriteString(":")
	credBuilder.WriteString(password)
	credentials := credBuilder.String()

	var authBuilder strings.Builder
	authBuilder.WriteString("Basic ")
	authBuilder.WriteString(base64.StdEncoding.EncodeToString([]byte(credentials)))

	h.headers["Authorization"] = authBuilder.String()

	return h
}

func (h *ClientBuilder) BearerAuth(token string) *ClientBuilder {
	var builder strings.Builder
	builder.WriteString("Bearer ")
	builder.WriteString(token)

	h.headers["Authorization"] = builder.String()

	return h
}

func (h *ClientBuilder) OAuth1(consumerKey, consumerSecret, accessToken, accessTokenSecret string) *ClientBuilder {
	h.oauth1 = &OAuth1Config{
		ConsumerKey:       consumerKey,
		ConsumerSecret:    consumerSecret,
		AccessToken:       accessToken,
		AccessTokenSecret: accessTokenSecret,
	}
	return h
}

func (h *ClientBuilder) Timeout(timeout time.Duration) *ClientBuilder {
	h.timeout = timeout
	return h
}

func (c *ClientBuilder) QueryParam(key string, value string) *ClientBuilder {
	c.queryParams[key] = value
	return c
}

func (h *ClientBuilder) Header(key, value string) *ClientBuilder {
	h.headers[key] = value
	return h
}

func (h *ClientBuilder) RetryConfig(maxRetries int, backoffMs int) *ClientBuilder {
	h.retryConfig = &RetryConfig{
		MaxRetries: maxRetries,
		BackoffMs:  backoffMs,
	}
	return h
}

func (h *ClientBuilder) Build() *Client {
	return &Client{
		context:     h.context,
		client:      &http.Client{Timeout: h.timeout},
		baseUrl:     h.baseUrl,
		timeout:     h.timeout,
		queryParams: h.queryParams,
		headers:     h.headers,
		contentType: h.contentType,
		oauth1:      h.oauth1,
		retryConfig: h.retryConfig,
	}
}
