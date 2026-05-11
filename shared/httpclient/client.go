package httpclient

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/archguard/project/shared/apperrors"
	"github.com/archguard/project/shared/constants"
)

const (
	errCreateRequest     = "failed to create request"
	errRequestFailed     = "request failed"
	errRateLimitExceeded = "rate limit exceeded after %d retries"
	rateLimitWaitFormat  = "\n[rate limit] waiting %s...\n"
)

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
	return &Client{
		httpClient:  &http.Client{Timeout: timeout},
		maxRetries:  maxRetries,
		retryWait:   retryWait,
		onRateLimit: func(wait time.Duration) { fmt.Fprintf(os.Stderr, rateLimitWaitFormat, wait) },
	}
}

func (c *Client) WithRateLimitHook(fn func(time.Duration)) *Client {
	c.onRateLimit = fn
	return c
}

func (c *Client) Post(cfg RequestConfig) (*Response, error) {
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		req, err := http.NewRequest(constants.HTTPMethodPost, cfg.URL, bytes.NewReader(cfg.Body))
		if err != nil {
			return nil, apperrors.Wrap(apperrors.KindInternal, "http request", errCreateRequest, err)
		}
		for key, val := range cfg.Headers {
			req.Header.Set(key, val)
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, apperrors.Wrap(apperrors.KindExternalService, "http request", errRequestFailed, err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusTooManyRequests {
			return &Response{StatusCode: resp.StatusCode, Body: body}, nil
		}
		if attempt < c.maxRetries {
			wait := c.retryWait * time.Duration(attempt+1)
			c.onRateLimit(wait)
			time.Sleep(wait)
		}
	}
	return nil, apperrors.RateLimited(fmt.Sprintf(errRateLimitExceeded, c.maxRetries))
}
