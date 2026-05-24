package llm

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	c "github.com/archguard/project/shared/constants"
	"github.com/archguard/project/shared/httpclient"
)

func TestRuntimeOptionsRetriesFallback(t *testing.T) {
	opts := RuntimeOptions{}
	if got := opts.retries(5); got != 5 {
		t.Errorf("zero MaxRetries should use fallback, got %d", got)
	}
	opts.MaxRetries = 9
	if got := opts.retries(5); got != 9 {
		t.Errorf("non-zero MaxRetries should override fallback, got %d", got)
	}
}

func TestRuntimeOptionsRetryWaitFallback(t *testing.T) {
	opts := RuntimeOptions{}
	if got := opts.retryWait(20 * time.Second); got != 20*time.Second {
		t.Errorf("zero RetryWait should use fallback, got %v", got)
	}
	opts.RetryWait = 7 * time.Second
	if got := opts.retryWait(20 * time.Second); got != 7*time.Second {
		t.Errorf("non-zero RetryWait should override fallback, got %v", got)
	}
}

func TestNewAnthropicClientWithOptionsStoresFlags(t *testing.T) {
	opts := RuntimeOptions{
		MaxRetries:              3,
		RetryWait:               5 * time.Second,
		AnthropicCacheTail:      true,
		RespectRateLimitHeaders: true,
	}
	client, err := NewAnthropicClientWithOptions("test-key", "claude-test", opts, nil)
	if err != nil {
		t.Fatalf("NewAnthropicClientWithOptions: %v", err)
	}
	if !client.cacheTail {
		t.Errorf("cacheTail should be true")
	}
	if !client.respectRateLimitHeaders {
		t.Errorf("respectRateLimitHeaders should be true")
	}
}

func TestNewAnthropicClientWithRateLimitHookKeepsDefaults(t *testing.T) {
	client, err := NewAnthropicClientWithRateLimitHook("test-key", "claude-test", nil)
	if err != nil {
		t.Fatalf("NewAnthropicClientWithRateLimitHook: %v", err)
	}
	if client.cacheTail {
		t.Errorf("legacy constructor must not enable cacheTail")
	}
	if client.respectRateLimitHeaders {
		t.Errorf("legacy constructor must not enable respectRateLimitHeaders")
	}
}

func TestNewOpenAIClientWithOptionsStoresHeaderFlag(t *testing.T) {
	opts := RuntimeOptions{RespectRateLimitHeaders: true}
	client, err := NewOpenAIClientWithOptions("test-key", "gpt-test", opts, nil)
	if err != nil {
		t.Fatalf("NewOpenAIClientWithOptions: %v", err)
	}
	if !client.respectRateLimitHeaders {
		t.Errorf("respectRateLimitHeaders should be true")
	}
}

func TestAnthropicRespectRateLimitHeadersActivatesPreemptiveSleep(t *testing.T) {
	resetAt := time.Now().Add(3 * time.Second).UTC().Format(time.RFC3339)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(c.HeaderAnthropicRequestsRemaining, "0")
		w.Header().Set(c.HeaderAnthropicRequestsReset, resetAt)
		w.Header().Set(c.HTTPHeaderContentType, c.HTTPContentTypeJSON)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"x","role":"assistant","content":[],"stop_reason":"end_turn","usage":{}}`))
	}))
	defer srv.Close()

	var observed time.Duration
	hook := func(d time.Duration) { observed = d }

	client, err := NewAnthropicClientWithOptions("k", "m", RuntimeOptions{RespectRateLimitHeaders: true}, hook)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	client.http = httpclient.New(2*time.Second, 0, 5*time.Second).
		WithRateLimitHook(hook).
		WithPreemptiveSleepObserver(hook).
		EnablePreemptiveSleep(anthropicRateLimitHeaders)

	resp, err := postViaTestServer(client.http, srv.URL)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if observed <= 0 {
		t.Errorf("expected preemptive sleep fired, got %s", observed)
	}
}

func postViaTestServer(client *httpclient.Client, url string) (*httpclient.Response, error) {
	return client.Post(httpclient.RequestConfig{URL: url, Body: []byte("{}")})
}
