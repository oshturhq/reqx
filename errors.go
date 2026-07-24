package reqx

import (
	"errors"
)

var (
	ErrInvalidBody        = errors.New("reqx: invalid request body")
	ErrMaxRetriesExceeded = errors.New("reqx: maximum retries exceeded")
	ErrInvalidURL         = errors.New("reqx: invalid url")
	ErrURLHostMismatch    = errors.New("reqx: path would leave the base url host")
	ErrURLPathEscape      = errors.New("reqx: path escapes the base url path")
	ErrAuthConflict       = errors.New("reqx: conflicting authentication mechanisms")
	ErrResponseTooLarge   = errors.New("reqx: response too large")
	ErrTooManyRedirects   = errors.New("reqx: too many redirects")
	ErrRedirectDowngrade  = errors.New("reqx: redirect downgrades https to plaintext")
	ErrDecodeResponse     = errors.New("reqx: failed to decode response")
)
