package reqx

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"strings"
)

type bodySource struct {
	get         func() (io.Reader, error)
	contentType string
	replayable  bool
}

func (c *Client) NewRequestBuilder() *RequestBuilder {
	queryParams := make(map[string]string, len(c.queryParams))
	maps.Copy(queryParams, c.queryParams)

	return &RequestBuilder{
		client:      c,
		ctx:         c.ctx,
		method:      MethodGet,
		path:        "",
		queryParams: queryParams,
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

func (r *RequestBuilder) Context(ctx context.Context) *RequestBuilder {
	r.ctx = ctx
	return r
}

func (r *RequestBuilder) Method(method Method) *RequestBuilder {
	r.method = method
	return r
}

func (r *RequestBuilder) Path(path string) *RequestBuilder {
	r.path = path
	return r
}

func (r *RequestBuilder) QueryParam(key string, value string) *RequestBuilder {
	r.queryParams[key] = value
	return r
}

func (r *RequestBuilder) Header(key string, value string) *RequestBuilder {
	r.headers[key] = value
	return r
}

func (r *RequestBuilder) Body(body any) *RequestBuilder {
	r.body = body
	return r
}

func (r *RequestBuilder) BodyReader(reader io.Reader) *RequestBuilder {
	r.body = reader
	return r
}

func (r *RequestBuilder) JSONContentType() *RequestBuilder {
	r.contentType = ContentTypeJSON
	return r
}

func (r *RequestBuilder) FormURLEncodedContentType() *RequestBuilder {
	r.contentType = ContentTypeFormURLEncoded
	return r
}

func (r *RequestBuilder) MultipartFormContentType() *RequestBuilder {
	r.contentType = ContentTypeMultipartForm
	return r
}

func (r *RequestBuilder) MultipartFormBody() *MultipartFormBuilder {
	return &MultipartFormBuilder{
		requestBuilder: r,
		formData:       &MultipartFormData{},
	}
}

func (r *RequestBuilder) Do(successTarget any, errorTarget any) (*Response, error) {
	fullURL, body, err := r.prepare()
	if err != nil {
		return nil, err
	}

	return r.executeWithRetry(body.replayable, func() (*Response, error) {
		response, err := r.roundTrip(fullURL, body)
		if err != nil {
			return nil, err
		}

		target := errorTarget
		if response.IsSuccess() {
			target = successTarget
		}

		if target != nil {
			if err := json.Unmarshal(response.Body, target); err != nil {
				return response, errors.Join(ErrDecodeResponse, err)
			}
		}

		return response, nil
	})
}

func (r *RequestBuilder) DoRaw() (*Response, error) {
	fullURL, body, err := r.prepare()
	if err != nil {
		return nil, err
	}

	return r.executeWithRetry(body.replayable, func() (*Response, error) {
		return r.roundTrip(fullURL, body)
	})
}

func (r *RequestBuilder) DoStream() (*Response, error) {
	fullURL, body, err := r.prepare()
	if err != nil {
		return nil, err
	}

	return r.executeWithRetry(body.replayable, func() (*Response, error) {
		req, err := r.buildRequest(r.ctx, fullURL, body)
		if err != nil {
			return nil, err
		}

		resp, err := r.client.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		return &Response{
			Status:     resp.StatusCode,
			Headers:    resp.Header,
			BodyReader: resp.Body,
		}, nil
	})
}

func (r *RequestBuilder) prepare() (string, *bodySource, error) {
	fullURL, err := r.buildURL()
	if err != nil {
		return "", nil, err
	}

	body, err := r.resolveBody()
	if err != nil {
		return "", nil, err
	}

	return fullURL, body, nil
}

func (r *RequestBuilder) roundTrip(fullURL string, body *bodySource) (*Response, error) {
	req, err := r.buildRequest(r.ctx, fullURL, body)
	if err != nil {
		return nil, err
	}

	resp, err := r.client.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			slog.Error("failed to close response body",
				"package", "reqx",
				"error", err)
		}
	}()

	bodyBytes, err := r.client.readBody(resp.Body)
	if err != nil {
		return nil, err
	}

	return &Response{
		Status:  resp.StatusCode,
		Body:    bodyBytes,
		Headers: resp.Header,
	}, nil
}

func (c *Client) readBody(reader io.Reader) ([]byte, error) {
	if c.maxResponseBytes <= 0 {
		return io.ReadAll(reader)
	}

	data, err := io.ReadAll(io.LimitReader(reader, c.maxResponseBytes+1))
	if err != nil {
		return nil, err
	}

	if int64(len(data)) > c.maxResponseBytes {
		return nil, ErrResponseTooLarge
	}

	return data, nil
}

func (r *RequestBuilder) buildURL() (string, error) {
	base := r.client.baseURL

	ref, err := url.Parse(r.path)
	if err != nil {
		return "", errors.Join(ErrInvalidURL, err)
	}

	var u *url.URL

	if base == nil {
		if ref.Scheme == "" || ref.Host == "" {
			return "", ErrInvalidURL
		}
		u = ref
	} else {
		if ref.Scheme != "" || ref.Host != "" || ref.User != nil {
			return "", ErrURLHostMismatch
		}

		u = base.JoinPath(ref.EscapedPath())
		if !withinBasePath(base.EscapedPath(), u.EscapedPath()) {
			return "", ErrURLPathEscape
		}
		u.Fragment = ref.Fragment
	}

	query := u.Query()
	if base != nil {
		maps.Copy(query, ref.Query())
	}
	for key, value := range r.queryParams {
		query.Set(key, value)
	}
	u.RawQuery = query.Encode()

	return u.String(), nil
}

func withinBasePath(basePath, joinedPath string) bool {
	basePath = strings.TrimSuffix(basePath, "/")
	if basePath == "" {
		return true
	}

	return joinedPath == basePath || strings.HasPrefix(joinedPath, basePath+"/")
}

func (r *RequestBuilder) resolveBody() (*bodySource, error) {
	if r.body == nil {
		return &bodySource{replayable: true}, nil
	}

	switch body := r.body.(type) {
	case io.Reader:
		return singleUseBody(body, ""), nil
	case []byte:
		return replayableBody(body, ""), nil
	case string:
		return replayableBody([]byte(body), ""), nil
	}

	switch r.contentType {
	case ContentTypeJSON:
		encoded, err := json.Marshal(r.body)
		if err != nil {
			return nil, err
		}
		return replayableBody(encoded, ""), nil
	case ContentTypeFormURLEncoded:
		form, ok := r.body.(url.Values)
		if !ok {
			return nil, ErrInvalidBody
		}
		return replayableBody([]byte(form.Encode()), ""), nil
	case ContentTypeMultipartForm:
		formData, ok := r.body.(*MultipartFormData)
		if !ok {
			return nil, ErrInvalidBody
		}
		return multipartBody(formData)
	}

	return nil, ErrInvalidBody
}

func replayableBody(data []byte, contentType string) *bodySource {
	return &bodySource{
		get:         func() (io.Reader, error) { return bytes.NewReader(data), nil },
		contentType: contentType,
		replayable:  true,
	}
}

func singleUseBody(reader io.Reader, contentType string) *bodySource {
	used := false

	return &bodySource{
		get: func() (io.Reader, error) {
			if used {
				return nil, ErrInvalidBody
			}
			used = true
			return reader, nil
		},
		contentType: contentType,
	}
}

func (r *RequestBuilder) buildRequest(ctx context.Context, fullURL string, body *bodySource) (*http.Request, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	var reader io.Reader
	if body.get != nil {
		bodyReader, err := body.get()
		if err != nil {
			return nil, err
		}
		reader = bodyReader
	}

	names := make([]string, 0, len(r.client.headers)+len(r.headers))
	for name := range r.client.headers {
		names = append(names, name)
	}
	for name := range r.headers {
		names = append(names, name)
	}
	ctx = context.WithValue(ctx, callerHeadersKey{}, names)

	req, err := http.NewRequestWithContext(ctx, string(r.method), fullURL, reader)
	if err != nil {
		return nil, err
	}

	if body.get != nil && body.replayable {
		get := body.get
		req.GetBody = func() (io.ReadCloser, error) {
			bodyReader, err := get()
			if err != nil {
				return nil, err
			}
			return io.NopCloser(bodyReader), nil
		}
	}

	for key, value := range r.client.headers {
		req.Header.Set(key, value)
	}
	for key, value := range r.headers {
		req.Header.Set(key, value)
	}

	if body.get != nil {
		switch {
		case body.contentType != "":
			req.Header.Set("Content-Type", body.contentType)
		case r.contentType != "":
			req.Header.Set("Content-Type", string(r.contentType))
		case r.client.contentType != "":
			req.Header.Set("Content-Type", string(r.client.contentType))
		}
	}

	if r.client.oauth1 != nil {
		authHeader, err := r.generateOAuth1Header(string(r.method), fullURL)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", authHeader)
	}

	return req, nil
}
