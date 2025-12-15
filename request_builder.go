package reqx

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
)

func (c *Client) NewRequestBuilder() *RequestBuilder {
	return &RequestBuilder{
		client:      c,
		method:      MethodGet,
		path:        "",
		queryParams: c.queryParams,
		headers:     make(map[string]string),
		contentType: ContentTypeJSON,
		body:        nil,
	}
}

func (c *Client) Get(path string) *RequestBuilder {
	return c.NewRequestBuilder().Method(MethodGet).Path(path)
}

func (c *Client) Post(path string) *RequestBuilder {
	return c.NewRequestBuilder().Method(MethodPost).Path(path)
}

func (c *Client) Put(path string) *RequestBuilder {
	return c.NewRequestBuilder().Method(MethodPut).Path(path)
}

func (c *Client) Delete(path string) *RequestBuilder {
	return c.NewRequestBuilder().Method(MethodDelete).Path(path)
}

func (c *Client) Patch(path string) *RequestBuilder {
	return c.NewRequestBuilder().Method(MethodPatch).Path(path)
}

func (c *RequestBuilder) Method(method Method) *RequestBuilder {
	c.method = method
	return c
}

func (c *RequestBuilder) Path(path string) *RequestBuilder {
	c.path = path
	return c
}

func (c *RequestBuilder) QueryParam(key string, value string) *RequestBuilder {
	c.queryParams[key] = value
	return c
}

func (c *RequestBuilder) Header(key string, value string) *RequestBuilder {
	c.headers[key] = value
	return c
}

func (c *RequestBuilder) Body(body any) *RequestBuilder {
	c.body = body
	return c
}

func (c *RequestBuilder) BodyReader(reader io.Reader) *RequestBuilder {
	c.body = reader
	return c
}

func (c *RequestBuilder) JsonContentType() *RequestBuilder {
	c.contentType = ContentTypeJSON
	return c
}

func (c *RequestBuilder) FormUrlencodedContentType() *RequestBuilder {
	c.contentType = ContentTypeFormUrlencoded
	return c
}

func (c *RequestBuilder) MultipartFormContentType() *RequestBuilder {
	c.contentType = ContentTypeMultipartForm
	return c
}

func (c *RequestBuilder) MultipartFormBody() *MultipartFormBuilder {
	return &MultipartFormBuilder{
		requestBuilder: c,
		formData:       &MultipartFormData{},
	}
}

func (c *RequestBuilder) Do(successTarget any, errorTarget any) (*Response, error) {
	return c.executeWithRetry(func() (*Response, error) {
		url := c.buildUrl()
		req, err := c.buildRequest(c.client.context, url)
		if err != nil {
			return nil, err
		}

		resp, err := c.client.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer func() {
			err := resp.Body.Close()
			if err != nil {
				slog.Error("Failed to close response body",
					"component", "RequestBuilder",
					"error", err)
			}
		}()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		response := &Response{
			Status:  resp.StatusCode,
			Body:    bodyBytes,
			Headers: resp.Header,
		}

		if response.IsSuccess() {
			if successTarget != nil {
				if err := json.Unmarshal(response.Body, successTarget); err != nil {
					slog.Error("failed to unmarshal success response",
						"package", "reqx",
						"error", err,
					)
				}
			}
		} else {
			if errorTarget != nil {
				if err := json.Unmarshal(response.Body, errorTarget); err != nil {
					slog.Error("failed to unmarshal error response",
						"package", "reqx",
						"error", err,
					)
				}
			}
		}

		return response, nil
	})
}

func (c *RequestBuilder) DoRaw() (*Response, error) {
	return c.executeWithRetry(func() (*Response, error) {
		url := c.buildUrl()
		req, err := c.buildRequest(c.client.context, url)
		if err != nil {
			return nil, err
		}

		resp, err := c.client.client.Do(req)
		if err != nil {
			return nil, err
		}
		defer func() {
			err := resp.Body.Close()
			if err != nil {
				slog.Error("Failed to close response body",
					"component", "RequestBuilder",
					"error", err)
			}
		}()

		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}

		response := &Response{
			Status:  resp.StatusCode,
			Body:    bodyBytes,
			Headers: resp.Header,
		}

		return response, nil
	})
}

func (c *RequestBuilder) DoStream() (*Response, error) {
	return c.executeWithRetry(func() (*Response, error) {
		url := c.buildUrl()
		req, err := c.buildRequest(c.client.context, url)
		if err != nil {
			return nil, err
		}

		resp, err := c.client.client.Do(req)
		if err != nil {
			return nil, err
		}

		response := &Response{
			Status:     resp.StatusCode,
			Headers:    resp.Header,
			BodyReader: resp.Body,
		}

		return response, nil
	})
}

func (c *RequestBuilder) buildUrl() string {
	var builder strings.Builder
	builder.WriteString(c.client.baseUrl)
	builder.WriteString(c.path)

	u, _ := url.Parse(builder.String())
	q := u.Query()
	for k, v := range c.queryParams {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func (b *RequestBuilder) buildRequest(ctx context.Context, fullURL string) (*http.Request, error) {
	var buf io.Reader
	if b.body != nil {
		switch body := b.body.(type) {
		case io.Reader:
			buf = body
		case []byte:
			buf = bytes.NewReader(body)
		case string:
			buf = strings.NewReader(body)
		default:
			switch b.contentType {
			case ContentTypeJSON:
				j, err := json.Marshal(body)
				if err != nil {
					return nil, err
				}
				buf = bytes.NewBuffer(j)
			case ContentTypeFormUrlencoded:
				form, ok := body.(url.Values)
				if !ok {
					return nil, ErrInvalidBody
				}
				buf = bytes.NewBufferString(form.Encode())
			case ContentTypeMultipartForm:
				formData, ok := body.(*MultipartFormData)
				if !ok {
					return nil, ErrInvalidBody
				}
				multipartBuf, contentType, err := b.buildMultipartForm(formData)
				if err != nil {
					return nil, err
				}
				buf = multipartBuf
				b.contentType = ContentType(contentType)
			}
		}
	}

	req, err := http.NewRequestWithContext(ctx, string(b.method), fullURL, buf)
	if err != nil {
		return nil, err
	}

	if b.client.oauth1 != nil {
		authHeader, err := b.generateOAuth1Header(string(b.method), fullURL)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", authHeader)
	}

	for k, v := range b.client.headers {
		req.Header.Set(k, v)
	}
	for k, v := range b.headers {
		req.Header.Set(k, v)
	}

	if b.body != nil {
		if b.client.contentType != "" {
			req.Header.Set("Content-Type", string(b.client.contentType))
		}

		if b.contentType != "" {
			req.Header.Set("Content-Type", string(b.contentType))
		}
	}

	return req, nil
}
