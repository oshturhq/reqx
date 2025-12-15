package reqx

import (
	"context"
	"io"
	"net/http"
	"time"
)

type Method string

const (
	MethodGet    Method = "GET"
	MethodPost   Method = "POST"
	MethodPut    Method = "PUT"
	MethodDelete Method = "DELETE"
	MethodPatch  Method = "PATCH"
)

type ContentType string

const (
	ContentTypeFormUrlencoded ContentType = "application/x-www-form-urlencoded"
	ContentTypeJSON           ContentType = "application/json; charset=UTF-8"
	ContentTypeMultipartForm  ContentType = "multipart/form-data"
)

type MultipartFormData struct {
	Fields []FormField
	Files  []FormFile
}

type FormField struct {
	Name  string
	Value string
}

type FormFile struct {
	FieldName string
	FileName  string
	Data      []byte
	Reader    io.Reader
}

type OAuth1Config struct {
	ConsumerKey       string
	ConsumerSecret    string
	AccessToken       string
	AccessTokenSecret string
}

type RetryConfig struct {
	MaxRetries int
	BackoffMs  int
}

type Client struct {
	context     context.Context
	client      *http.Client
	baseUrl     string
	timeout     time.Duration
	queryParams map[string]string
	headers     map[string]string
	contentType ContentType
	oauth1      *OAuth1Config
	retryConfig *RetryConfig
}

type RequestBuilder struct {
	client      *Client
	method      Method
	path        string
	queryParams map[string]string
	headers     map[string]string
	contentType ContentType
	body        any
}

type Response struct {
	Status     int
	Body       []byte
	Headers    http.Header
	BodyReader io.ReadCloser
}

func (r *Response) IsSuccess() bool {
	return checkStatus(r.Status, 200, 300)
}

func (r *Response) IsError() bool {
	return checkStatus(r.Status, 400, 500)
}

func (r *Response) IsServerError() bool {
	return checkStatus(r.Status, 500, 0)
}

func checkStatus(status int, left int, right int) bool {
	if right == 0 {
		return status >= left
	}

	return status >= left && status < right
}
