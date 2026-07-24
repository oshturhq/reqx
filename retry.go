package reqx

import (
	"context"
	"errors"
	"math/rand/v2"
	"net"
	"net/http"
	"strconv"
	"time"
)

const maxRetryAfter = 30 * time.Second

func (r *RequestBuilder) shouldRetry(err error, statusCode int) bool {
	if err != nil {
		if netErr, ok := errors.AsType[net.Error](err); ok && netErr.Timeout() {
			return true
		}

		var dnsErr *net.DNSError
		return errors.As(err, &dnsErr)
	}

	return statusCode >= http.StatusInternalServerError || statusCode == http.StatusTooManyRequests
}

func (r *RequestBuilder) executeWithRetry(replayable bool, fn func() (*Response, error)) (*Response, error) {
	maxRetries := r.client.retryConfig.MaxRetries
	backoffMs := r.client.retryConfig.BackoffMs

	ctx := r.client.ctx
	if ctx == nil {
		ctx = context.Background()
	}

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

		if !shouldRetry || !replayable {
			break
		}

		closeStream(lastResp)

		if err := sleep(ctx, backoffFor(lastResp, attempt, backoffMs)); err != nil {
			return lastResp, err
		}
	}

	if lastErr != nil {
		return lastResp, lastErr
	}

	if lastResp != nil && !r.shouldRetry(nil, lastResp.Status) {
		return lastResp, nil
	}

	return lastResp, ErrMaxRetriesExceeded
}

func backoffFor(resp *Response, attempt int, backoffMs int) time.Duration {
	if delay, ok := retryAfter(resp); ok {
		return delay
	}

	base := time.Duration(backoffMs*(attempt+1)) * time.Millisecond
	if base <= 0 {
		return 0
	}

	half := base / 2

	return half + time.Duration(rand.Int64N(int64(half)+1))
}

func retryAfter(resp *Response) (time.Duration, bool) {
	if resp == nil || resp.Headers == nil {
		return 0, false
	}

	value := resp.Headers.Get("Retry-After")
	if value == "" {
		return 0, false
	}

	if seconds, err := strconv.Atoi(value); err == nil {
		return min(max(time.Duration(seconds)*time.Second, 0), maxRetryAfter), true
	}

	if deadline, err := http.ParseTime(value); err == nil {
		return min(max(time.Until(deadline), 0), maxRetryAfter), true
	}

	return 0, false
}

func sleep(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func closeStream(resp *Response) {
	if resp == nil || resp.BodyReader == nil {
		return
	}

	_ = resp.BodyReader.Close()
	resp.BodyReader = nil
}
