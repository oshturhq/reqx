package httpclient

import (
	"errors"
)

var (
	ErrInvalidBody        = errors.New("httpclient.invalid_body")
	ErrMaxRetriesExceeded = errors.New("httpclient.max_retries_exceeded")
)
