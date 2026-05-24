package httpclient

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/archguard/project/shared/constants"
)

func TestPostHonorsRetryAfter(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n == 1 {
			w.Header().Set(constants.HTTPHeaderRetryAfter, "3")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	var observed time.Duration
	client := New(5*time.Second, 2, 20*time.Second).WithRateLimitHook(func(d time.Duration) {
		if observed == 0 {
			observed = d
		}
	})
	resp, err := client.Post(RequestConfig{URL: srv.URL, Body: []byte("{}")})
	if err != nil {
		t.Fatalf("post failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if observed != 3*time.Second {
		t.Fatalf("hook received %s, want 3s from Retry-After header", observed)
	}
}

func TestPostFallsBackWithoutRetryAfter(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	const baseWait = 2 * time.Second
	var observed time.Duration
	client := New(5*time.Second, 2, baseWait).WithRateLimitHook(func(d time.Duration) {
		observed = d
	})
	if _, err := client.Post(RequestConfig{URL: srv.URL, Body: []byte("{}")}); err != nil {
		t.Fatalf("post failed: %v", err)
	}
	// attempt 0: exp = base << 0 = base; jitter ∈ [0, base) → observed ∈ [base, 2*base).
	if observed < baseWait || observed >= 2*baseWait {
		t.Fatalf("hook received %s, want fallback in [%s, %s) (exp + jitter on attempt 0)", observed, baseWait, 2*baseWait)
	}
}

func TestPostOverloadedAlsoTriggersRetry(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n == 1 {
			w.Header().Set(constants.HTTPHeaderRetryAfter, "1")
			w.WriteHeader(constants.HTTPStatusOverloaded)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	hookCalled := false
	client := New(5*time.Second, 2, 5*time.Second).WithRateLimitHook(func(d time.Duration) {
		hookCalled = true
	})
	resp, err := client.Post(RequestConfig{URL: srv.URL, Body: []byte("{}")})
	if err != nil {
		t.Fatalf("post failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 after overload retry", resp.StatusCode)
	}
	if !hookCalled {
		t.Fatalf("rate-limit hook should fire on 529 (overloaded)")
	}
}

func TestFallbackBackoffShapeExponentialPlusJitter(t *testing.T) {
	const baseWait = 1 * time.Second
	client := New(5*time.Second, 10, baseWait)

	maxWait := baseWait * retryAfterClampMaxScale
	upperBound := maxWait + baseWait // exp clamped + jitter < base

	for attempt := 0; attempt < 8; attempt++ {
		wait := client.fallbackBackoff(attempt)
		expected := baseWait << uint(attempt)
		if expected > maxWait {
			expected = maxWait
		}
		if wait < expected {
			t.Errorf("attempt %d: wait %s < expected floor %s", attempt, wait, expected)
		}
		if wait >= expected+baseWait {
			t.Errorf("attempt %d: wait %s >= expected ceiling %s", attempt, wait, expected+baseWait)
		}
		if wait >= upperBound {
			t.Errorf("attempt %d: wait %s breaks global upper bound %s", attempt, wait, upperBound)
		}
	}
}

func TestFallbackBackoffDeterministicWithSeed(t *testing.T) {
	t.Setenv(constants.EnvDeterministicBackoff, "1")

	const baseWait = 1 * time.Second
	clientA := New(5*time.Second, 3, baseWait)
	clientB := New(5*time.Second, 3, baseWait)

	for attempt := 0; attempt < 5; attempt++ {
		a := clientA.fallbackBackoff(attempt)
		b := clientB.fallbackBackoff(attempt)
		if a != b {
			t.Errorf("attempt %d: deterministic mode should produce identical backoffs, got %s vs %s", attempt, a, b)
		}
	}
}

func TestFallbackBackoffNonZeroJitter(t *testing.T) {
	const baseWait = 1 * time.Second
	client := New(5*time.Second, 3, baseWait)

	saw := make(map[time.Duration]bool)
	for i := 0; i < 32; i++ {
		saw[client.fallbackBackoff(0)] = true
	}
	if len(saw) < 2 {
		t.Errorf("expected jitter to vary across 32 calls, saw %d distinct values", len(saw))
	}
}

func anthropicHeaders() RateLimitHeaders {
	return RateLimitHeaders{
		RequestsRemaining: constants.HeaderAnthropicRequestsRemaining,
		TokensRemaining:   constants.HeaderAnthropicTokensRemaining,
		RequestsReset:     constants.HeaderAnthropicRequestsReset,
		TokensReset:       constants.HeaderAnthropicTokensReset,
		TokenThreshold:    constants.TailTokenThreshold,
	}
}

func TestPreemptiveSleepFiresWhenRequestsExhausted(t *testing.T) {
	resetAt := time.Now().Add(4 * time.Second).UTC().Format(time.RFC3339)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderAnthropicRequestsRemaining, "0")
		w.Header().Set(constants.HeaderAnthropicRequestsReset, resetAt)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))
	defer srv.Close()

	var observed time.Duration
	client := New(5*time.Second, 2, 20*time.Second).
		WithPreemptiveSleepObserver(func(d time.Duration) { observed = d }).
		EnablePreemptiveSleep(anthropicHeaders())

	resp, err := client.Post(RequestConfig{URL: srv.URL, Body: []byte("{}")})
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if observed <= 0 || observed > 5*time.Second {
		t.Fatalf("preemptive observer received %s, want ~4s", observed)
	}
}

func TestPreemptiveSleepDisabledByDefault(t *testing.T) {
	resetAt := time.Now().Add(4 * time.Second).UTC().Format(time.RFC3339)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderAnthropicRequestsRemaining, "0")
		w.Header().Set(constants.HeaderAnthropicRequestsReset, resetAt)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	preemptFired := false
	rateLimitFired := false
	client := New(5*time.Second, 2, 20*time.Second).
		WithRateLimitHook(func(time.Duration) { rateLimitFired = true }).
		WithPreemptiveSleepObserver(func(time.Duration) { preemptFired = true })

	if _, err := client.Post(RequestConfig{URL: srv.URL, Body: []byte("{}")}); err != nil {
		t.Fatalf("post: %v", err)
	}
	if preemptFired || rateLimitFired {
		t.Fatalf("preemptive sleep must stay off until EnablePreemptiveSleep is called (preempt=%v rateLimit=%v)", preemptFired, rateLimitFired)
	}
}

func TestPreemptiveSleepSkipsWhenBudgetHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderAnthropicRequestsRemaining, "50")
		w.Header().Set(constants.HeaderAnthropicTokensRemaining, "100000")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	hookFired := false
	client := New(5*time.Second, 2, 20*time.Second).
		WithPreemptiveSleepObserver(func(time.Duration) { hookFired = true }).
		EnablePreemptiveSleep(anthropicHeaders())

	if _, err := client.Post(RequestConfig{URL: srv.URL, Body: []byte("{}")}); err != nil {
		t.Fatalf("post: %v", err)
	}
	if hookFired {
		t.Fatalf("observer fired on healthy budget (50 req / 100k tok remaining)")
	}
}

func TestPreemptiveSleepFiresOnTokenThreshold(t *testing.T) {
	resetAt := time.Now().Add(3 * time.Second).UTC().Format(time.RFC3339)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderAnthropicRequestsRemaining, "50")
		w.Header().Set(constants.HeaderAnthropicTokensRemaining, "100") // < TailTokenThreshold
		w.Header().Set(constants.HeaderAnthropicTokensReset, resetAt)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var observed time.Duration
	client := New(5*time.Second, 2, 20*time.Second).
		WithPreemptiveSleepObserver(func(d time.Duration) { observed = d }).
		EnablePreemptiveSleep(anthropicHeaders())

	if _, err := client.Post(RequestConfig{URL: srv.URL, Body: []byte("{}")}); err != nil {
		t.Fatalf("post: %v", err)
	}
	if observed <= 0 {
		t.Fatalf("expected preemptive sleep on low token budget, got %s", observed)
	}
}

func TestPreemptiveSleepHandlesDurationResetFormat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderOpenAIRequestsRemaining, "0")
		w.Header().Set(constants.HeaderOpenAIRequestsReset, "2s500ms")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var observed time.Duration
	client := New(5*time.Second, 2, 20*time.Second).
		WithPreemptiveSleepObserver(func(d time.Duration) { observed = d }).
		EnablePreemptiveSleep(RateLimitHeaders{
			RequestsRemaining: constants.HeaderOpenAIRequestsRemaining,
			RequestsReset:     constants.HeaderOpenAIRequestsReset,
			TokenThreshold:    constants.TailTokenThreshold,
		})

	if _, err := client.Post(RequestConfig{URL: srv.URL, Body: []byte("{}")}); err != nil {
		t.Fatalf("post: %v", err)
	}
	if observed != 2500*time.Millisecond {
		t.Fatalf("expected 2.5s parsed from duration header, got %s", observed)
	}
}

func TestRateLimitObserverFiresOn429AndOverloaded(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		switch n {
		case 1:
			w.Header().Set(constants.HTTPHeaderRetryAfter, "0")
			w.WriteHeader(http.StatusTooManyRequests)
		case 2:
			w.Header().Set(constants.HTTPHeaderRetryAfter, "0")
			w.WriteHeader(constants.HTTPStatusOverloaded)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	var observed []string
	client := New(5*time.Second, 5, 1*time.Second).
		WithRateLimitHook(func(time.Duration) {}).
		WithRateLimitObserver(func(kind string) {
			observed = append(observed, kind)
		})

	if _, err := client.Post(RequestConfig{URL: srv.URL, Body: []byte("{}")}); err != nil {
		t.Fatalf("post: %v", err)
	}
	if len(observed) != 2 {
		t.Fatalf("observer fired %d times, want 2 (one per 429 and 529)", len(observed))
	}
	if observed[0] != constants.RateLimitKind429 {
		t.Errorf("first observation kind = %q, want %q", observed[0], constants.RateLimitKind429)
	}
	if observed[1] != constants.RateLimitKindOverloaded {
		t.Errorf("second observation kind = %q, want %q", observed[1], constants.RateLimitKindOverloaded)
	}
}

func TestRateLimitObserverNotFiredOnSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var fired bool
	client := New(5*time.Second, 2, 1*time.Second).
		WithRateLimitObserver(func(string) { fired = true })

	if _, err := client.Post(RequestConfig{URL: srv.URL, Body: []byte("{}")}); err != nil {
		t.Fatalf("post: %v", err)
	}
	if fired {
		t.Errorf("observer must not fire on 200 OK")
	}
}

func TestRateLimitCooldownSharedByHost(t *testing.T) {
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n == 1 {
			w.Header().Set(constants.HTTPHeaderRetryAfter, "2")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	first := New(5*time.Second, 0, 1*time.Second).
		WithRateLimitHook(func(time.Duration) {})
	if _, err := first.Post(RequestConfig{URL: srv.URL, Body: []byte("{}")}); err == nil {
		t.Fatalf("first post should return rate limit error")
	}

	var observed time.Duration
	second := New(5*time.Second, 0, 1*time.Second).
		WithRateLimitHook(func(d time.Duration) { observed = d })
	resp, err := second.Post(RequestConfig{URL: srv.URL, Body: []byte("{}")})
	if err != nil {
		t.Fatalf("second post: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if observed <= 0 || observed > 2*time.Second {
		t.Fatalf("shared host cooldown observed %s, want within Retry-After window", observed)
	}
}

func TestPreemptiveSleepClampedToMaxWait(t *testing.T) {
	resetAt := time.Now().Add(10 * time.Minute).UTC().Format(time.RFC3339)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderAnthropicRequestsRemaining, "0")
		w.Header().Set(constants.HeaderAnthropicRequestsReset, resetAt)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	const baseWait = 5 * time.Second
	var observed time.Duration
	client := New(5*time.Second, 2, baseWait).
		WithPreemptiveSleepObserver(func(d time.Duration) { observed = d }).
		EnablePreemptiveSleep(anthropicHeaders())

	if _, err := client.Post(RequestConfig{URL: srv.URL, Body: []byte("{}")}); err != nil {
		t.Fatalf("post: %v", err)
	}
	maxAllowed := baseWait * retryAfterClampMaxScale
	if observed != maxAllowed {
		t.Fatalf("expected clamp to %s, got %s (10-minute reset must not propagate)", maxAllowed, observed)
	}
}

func TestRateLimitObserverNotFiredOnPreemptive(t *testing.T) {
	resetAt := time.Now().Add(3 * time.Second).UTC().Format(time.RFC3339)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(constants.HeaderAnthropicRequestsRemaining, "0")
		w.Header().Set(constants.HeaderAnthropicRequestsReset, resetAt)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rateLimitFired := false
	preemptFired := false
	client := New(5*time.Second, 2, 20*time.Second).
		WithRateLimitHook(func(time.Duration) { rateLimitFired = true }).
		WithRateLimitObserver(func(string) { rateLimitFired = true }).
		WithPreemptiveSleepObserver(func(time.Duration) { preemptFired = true }).
		EnablePreemptiveSleep(anthropicHeaders())

	if _, err := client.Post(RequestConfig{URL: srv.URL, Body: []byte("{}")}); err != nil {
		t.Fatalf("post: %v", err)
	}
	if !preemptFired {
		t.Fatal("preemptive observer must fire on near-empty headers")
	}
	if rateLimitFired {
		t.Fatal("rate-limit hook/observer must NOT fire on preemptive 200 OK path")
	}
}

func TestHttpClientFiresExactlyOneUIHookPerWait(t *testing.T) {
	// 429 reactive path → only onRateLimit fires; onPreemptiveSleep stays silent.
	var attempts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&attempts, 1) == 1 {
			w.Header().Set(constants.HTTPHeaderRetryAfter, "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	rateLimitCalls := 0
	preemptCalls := 0
	client := New(5*time.Second, 2, 1*time.Second).
		WithRateLimitHook(func(time.Duration) { rateLimitCalls++ }).
		WithPreemptiveSleepObserver(func(time.Duration) { preemptCalls++ })

	if _, err := client.Post(RequestConfig{URL: srv.URL, Body: []byte("{}")}); err != nil {
		t.Fatalf("post: %v", err)
	}
	if rateLimitCalls == 0 {
		t.Errorf("429 path must fire onRateLimit at least once, got %d", rateLimitCalls)
	}
	if preemptCalls != 0 {
		t.Errorf("429 path must NOT fire onPreemptiveSleep, got %d", preemptCalls)
	}
}
