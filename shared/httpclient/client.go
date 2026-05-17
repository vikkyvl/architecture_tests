package httpclient

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/archguard/project/shared/apperrors"
	"github.com/archguard/project/shared/constants"
)

const (
	errCreateRequest     = "failed to create request"
	errRequestFailed     = "request failed"
	errReadResponseBody  = "failed to read response body"
	errRateLimitExceeded = "rate limit exceeded after %d retries"
	operationHTTPRequest = "http request"
	operationHTTPResp    = "http response"
)

func sleepRateLimitHook(wait time.Duration) {
	time.Sleep(wait)
}

type Client struct {
	httpClient  *http.Client
	maxRetries  int
	retryWait   time.Duration
	onRateLimit func(wait time.Duration)
}

type RequestConfig struct {
	URL     string
	Body    []byte
	Headers map[string]string
}

type Response struct {
	StatusCode int
	Body       []byte
}

func New(timeout time.Duration, maxRetries int, retryWait time.Duration) *Client {
	return NewWithRateLimitHook(timeout, maxRetries, retryWait, nil)
}

func NewWithRateLimitHook(timeout time.Duration, maxRetries int, retryWait time.Duration, hook func(time.Duration)) *Client {
	if hook == nil {
		hook = sleepRateLimitHook
	}
	return &Client{
		httpClient:  &http.Client{Timeout: timeout},
		maxRetries:  maxRetries,
		retryWait:   retryWait,
		onRateLimit: hook,
	}
}

func (c *Client) WithRateLimitHook(fn func(time.Duration)) *Client {
	if fn == nil {
		fn = sleepRateLimitHook
	}
	c.onRateLimit = fn
	return c
}

func (c *Client) Post(cfg RequestConfig) (*Response, error) {
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		req, err := http.NewRequest(constants.HTTPMethodPost, cfg.URL, bytes.NewReader(cfg.Body))
		if err != nil {
			return nil, apperrors.Wrap(apperrors.KindInternal, operationHTTPRequest, errCreateRequest, err)
		}
		for key, val := range cfg.Headers {
			req.Header.Set(key, val)
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, apperrors.Wrap(apperrors.KindExternalService, operationHTTPRequest, errRequestFailed, err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, apperrors.Wrap(apperrors.KindExternalService, operationHTTPResp, errReadResponseBody, readErr)
		}

		if resp.StatusCode != http.StatusTooManyRequests {
			return &Response{
				StatusCode: resp.StatusCode,
				Body:       body,
			}, nil
		}
		if attempt < c.maxRetries {
			wait := c.retryWait * time.Duration(attempt+1)
			c.onRateLimit(wait)
		}
	}
	return nil, apperrors.RateLimited(fmt.Sprintf(errRateLimitExceeded, c.maxRetries))
}
