package httpclient

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/archguard/project/shared/apperrors"
	"github.com/archguard/project/shared/constants"
)

const (
	errCreateRequest        = "failed to create request"
	errRequestFailed        = "request failed"
	errReadResponseBody     = "failed to read response body"
	errRateLimitExceeded    = "rate limit exceeded after %d retries"
	operationHTTPRequest    = "http request"
	operationHTTPResp       = "http response"
	retryAfterClampMinSec   = 1
	retryAfterClampMaxScale = 5
)

func sleepRateLimitHook(wait time.Duration) {
	time.Sleep(wait)
}

type Client struct {
	httpClient          *http.Client
	maxRetries          int
	retryWait           time.Duration
	onRateLimit         func(wait time.Duration)
	onRateLimitObserved func(kind string)
	onPreemptiveSleep   func(wait time.Duration)
	backoffRand         *rand.Rand
	randMu              sync.Mutex
	preemptiveSleep     bool
	rateLimitHeaders    RateLimitHeaders
}

type cooldownCoordinator struct {
	mu    sync.Mutex
	until map[string]time.Time
}

var providerCooldowns = &cooldownCoordinator{until: make(map[string]time.Time)}

type RateLimitHeaders struct {
	RequestsRemaining string
	TokensRemaining   string
	RequestsReset     string
	TokensReset       string
	TokenThreshold    int
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
		httpClient:        &http.Client{Timeout: timeout},
		maxRetries:        maxRetries,
		retryWait:         retryWait,
		onRateLimit:       hook,
		onPreemptiveSleep: sleepRateLimitHook,
		backoffRand:       newBackoffRand(),
	}
}

func newBackoffRand() *rand.Rand {
	if constants.IsEnvTrue(constants.EnvDeterministicBackoff) {
		return rand.New(rand.NewSource(constants.DeterministicBackoffSeed))
	}
	return rand.New(rand.NewSource(time.Now().UnixNano()))
}

func (c *Client) WithRateLimitHook(fn func(time.Duration)) *Client {
	if fn == nil {
		fn = sleepRateLimitHook
	}
	c.onRateLimit = fn
	return c
}

func (c *Client) EnablePreemptiveSleep(h RateLimitHeaders) *Client {
	c.preemptiveSleep = true
	c.rateLimitHeaders = h
	return c
}

func (c *Client) WithRateLimitObserver(fn func(kind string)) *Client {
	c.onRateLimitObserved = fn
	return c
}

func (c *Client) WithPreemptiveSleepObserver(fn func(wait time.Duration)) *Client {
	if fn == nil {
		fn = sleepRateLimitHook
	}
	c.onPreemptiveSleep = fn
	return c
}

func rateLimitKindFromStatus(status int) string {
	switch status {
	case constants.HTTPStatusOverloaded:
		return constants.RateLimitKindOverloaded
	case constants.HTTPStatusUnavailable:
		return constants.RateLimitKindUnavailable
	default:
		return constants.RateLimitKind429
	}
}

func (c *Client) Post(cfg RequestConfig) (*Response, error) {
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		if wait := providerCooldowns.wait(cfg.URL); wait > 0 {
			c.onRateLimit(wait)
		}
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
		_ = resp.Body.Close()
		if readErr != nil {
			return nil, apperrors.Wrap(apperrors.KindExternalService, operationHTTPResp, errReadResponseBody, readErr)
		}

		if !isRateLimited(resp.StatusCode) {
			if c.preemptiveSleep && resp.StatusCode == http.StatusOK {
				if wait := c.preemptiveWait(resp.Header); wait > 0 {
					providerCooldowns.set(cfg.URL, wait)
					c.onPreemptiveSleep(wait)
				}
			}
			return &Response{
				StatusCode: resp.StatusCode,
				Body:       body,
			}, nil
		}
		if c.onRateLimitObserved != nil {
			c.onRateLimitObserved(rateLimitKindFromStatus(resp.StatusCode))
		}
		wait := c.pickRetryWait(resp.Header, attempt)
		providerCooldowns.set(cfg.URL, wait)
		if attempt < c.maxRetries {
			c.onRateLimit(wait)
		}
	}
	return nil, apperrors.RateLimited(fmt.Sprintf(errRateLimitExceeded, c.maxRetries))
}

func (c *cooldownCoordinator) wait(rawURL string) time.Duration {
	key := cooldownKey(rawURL)
	if key == "" {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	until, ok := c.until[key]
	if !ok {
		return 0
	}
	wait := time.Until(until)
	if wait <= 0 {
		delete(c.until, key)
		return 0
	}
	return wait
}

func (c *cooldownCoordinator) set(rawURL string, wait time.Duration) {
	if wait <= 0 {
		return
	}
	key := cooldownKey(rawURL)
	if key == "" {
		return
	}
	until := time.Now().Add(wait)
	c.mu.Lock()
	defer c.mu.Unlock()
	if current, ok := c.until[key]; ok && current.After(until) {
		return
	}
	c.until[key] = until
}

func cooldownKey(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func (c *Client) preemptiveWait(h http.Header) time.Duration {
	cfg := c.rateLimitHeaders
	var wait time.Duration

	if cfg.RequestsRemaining != "" && cfg.RequestsReset != "" {
		if remaining, ok := parseIntHeader(h, cfg.RequestsRemaining); ok && remaining == 0 {
			if w, ok := parseResetHeader(h.Get(cfg.RequestsReset)); ok && w > wait {
				wait = w
			}
		}
	}
	if cfg.TokensRemaining != "" && cfg.TokensReset != "" {
		threshold := cfg.TokenThreshold
		if remaining, ok := parseIntHeader(h, cfg.TokensRemaining); ok && remaining < threshold {
			if w, ok := parseResetHeader(h.Get(cfg.TokensReset)); ok && w > wait {
				wait = w
			}
		}
	}
	return clampPreemptive(wait, c.retryWait)
}

func parseIntHeader(h http.Header, name string) (int, bool) {
	raw := h.Get(name)
	if raw == "" {
		return 0, false
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		return 0, false
	}
	return n, true
}

func parseResetHeader(raw string) (time.Duration, bool) {
	if raw == "" {
		return 0, false
	}
	if d, err := time.ParseDuration(raw); err == nil {
		return d, true
	}
	if when, err := time.Parse(time.RFC3339, raw); err == nil {
		return time.Until(when), true
	}
	if secs, err := strconv.Atoi(raw); err == nil {
		return time.Duration(secs) * time.Second, true
	}
	return 0, false
}

func clampPreemptive(wait, retryWait time.Duration) time.Duration {
	if wait <= 0 {
		return 0
	}
	maxWait := retryWait * time.Duration(retryAfterClampMaxScale)
	if wait > maxWait {
		return maxWait
	}
	return wait
}

func isRateLimited(status int) bool {
	return status == http.StatusTooManyRequests ||
		status == constants.HTTPStatusOverloaded ||
		status == constants.HTTPStatusUnavailable
}

func (c *Client) pickRetryWait(h http.Header, attempt int) time.Duration {
	fallback := c.fallbackBackoff(attempt)
	if h == nil {
		return fallback
	}
	raw := h.Get(constants.HTTPHeaderRetryAfter)
	if raw == "" {
		return fallback
	}
	if secs, err := strconv.Atoi(raw); err == nil {
		return clampRetryAfter(time.Duration(secs)*time.Second, c.retryWait)
	}
	if when, err := http.ParseTime(raw); err == nil {
		return clampRetryAfter(time.Until(when), c.retryWait)
	}
	return fallback
}

func (c *Client) fallbackBackoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	exp := c.retryWait << uint(attempt)
	if exp < c.retryWait {
		exp = c.retryWait
	}
	maxWait := c.retryWait * retryAfterClampMaxScale
	if exp > maxWait {
		exp = maxWait
	}
	return exp + c.jitter(c.retryWait)
}

func (c *Client) jitter(maxJitter time.Duration) time.Duration {
	if maxJitter <= 0 || c.backoffRand == nil {
		return 0
	}
	c.randMu.Lock()
	defer c.randMu.Unlock()
	return time.Duration(c.backoffRand.Int63n(int64(maxJitter)))
}

func clampRetryAfter(d, retryWait time.Duration) time.Duration {
	minWait := time.Duration(retryAfterClampMinSec) * time.Second
	maxWait := retryWait * time.Duration(retryAfterClampMaxScale)
	if d < minWait {
		return minWait
	}
	if d > maxWait {
		return maxWait
	}
	return d
}
