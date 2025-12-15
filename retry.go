package httpclient

import (
	"errors"
	"net"
	"net/http"
	"time"
)

func (r *RequestBuilder) shouldRetry(err error, statusCode int) bool {
	if err != nil {
		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return true
		}

		var dnsErr *net.DNSError
		return errors.As(err, &dnsErr)
	}

	return statusCode >= http.StatusInternalServerError || statusCode == http.StatusTooManyRequests
}

func (r *RequestBuilder) executeWithRetry(fn func() (*Response, error)) (*Response, error) {
	maxRetries := r.client.retryConfig.MaxRetries
	backoffMs := r.client.retryConfig.BackoffMs

	var lastErr error
	var lastResp *Response

	for attempt := 0; attempt <= maxRetries; attempt++ {
		resp, err := fn()

		if err == nil && !r.shouldRetry(nil, resp.Status) {
			return resp, nil
		}

		lastErr = err
		lastResp = resp

		if attempt == maxRetries {
			break
		}

		shouldRetry := r.shouldRetry(err, 0)
		if resp != nil {
			shouldRetry = shouldRetry || r.shouldRetry(nil, resp.Status)
		}

		if !shouldRetry {
			break
		}

		backoffDuration := time.Duration(backoffMs*(attempt+1)) * time.Millisecond

		time.Sleep(backoffDuration)
	}

	if lastErr != nil {
		return lastResp, lastErr
	}

	return lastResp, ErrMaxRetriesExceeded
}
