package reqx

import (
	"errors"
)

var (
	ErrInvalidBody        = errors.New("reqx.invalid_body")
	ErrMaxRetriesExceeded = errors.New("reqx.max_retries_exceeded")
)
