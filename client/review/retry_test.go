package review

import (
	"errors"
	"testing"
	"time"

	"github.com/archguard/project/api/llm"
	"github.com/archguard/project/shared/apperrors"
	c "github.com/archguard/project/shared/constants"
)

type errSeqProvider struct {
	calls int
	seq   []error
}

func (p *errSeqProvider) SendMessage(req llm.Request) (*llm.Response, error) {
	idx := p.calls
	p.calls++
	if idx >= len(p.seq) {
		return &llm.Response{StopReason: c.StopReasonEndTurn}, nil
	}
	if err := p.seq[idx]; err != nil {
		return nil, err
	}
	return &llm.Response{StopReason: c.StopReasonEndTurn}, nil
}

func (p *errSeqProvider) Model() string {
	return "fake-model"
}

func (p *errSeqProvider) Name() string {
	return "fake"
}

func newRetryEngine(p llm.Provider) *Engine {
	return &Engine{provider: p, observer: noopObserver{}}
}

func TestIsRetryableLLMErrorClassification(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain error", errors.New("boom"), false},
		{"rate_limited", apperrors.RateLimited("rate limit exceeded after 5 retries"), false},
		{"external_service", apperrors.ExternalService("transient upstream"), true},
		{"validation", apperrors.Validation("bad input"), false},
		{"internal with overloaded", apperrors.Internal("provider returned overloaded response"), true},
		{"internal with temporarily unavailable", apperrors.Internal("service temporarily unavailable"), true},
		{"internal with unrelated message", apperrors.Internal("nil pointer deref"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isRetryableLLMError(tc.err); got != tc.want {
				t.Errorf("isRetryableLLMError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

func TestSendMessageWithRetryDoesNotRetryRateLimited(t *testing.T) {
	p := &errSeqProvider{seq: []error{apperrors.RateLimited("rate limit exceeded after 5 retries")}}
	eng := newRetryEngine(p)
	zeroWaits := []time.Duration{0, 0, 0}

	_, err := eng.sendMessageWithRetry(llm.Request{}, 3, zeroWaits)
	if err == nil {
		t.Fatal("expected error to surface, got nil")
	}
	if !apperrors.IsKind(err, apperrors.KindRateLimited) {
		t.Errorf("expected KindRateLimited to propagate, got %v", err)
	}
	if p.calls != 1 {
		t.Errorf("RateLimited must not be retried at engine layer; got %d calls", p.calls)
	}
}

func TestSendMessageWithRetryRetriesExternalService(t *testing.T) {
	p := &errSeqProvider{seq: []error{
		apperrors.ExternalService("transient 1"),
		apperrors.ExternalService("transient 2"),
		nil,
	}}
	eng := newRetryEngine(p)
	zeroWaits := []time.Duration{0, 0, 0}

	resp, err := eng.sendMessageWithRetry(llm.Request{}, 3, zeroWaits)
	if err != nil {
		t.Fatalf("expected eventual success after retries, got %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response after retry success")
	}
	if p.calls != 3 {
		t.Errorf("expected 3 calls (2 retries + success), got %d", p.calls)
	}
}

func TestSendMessageWithRetryExhaustsBudgetForExternalService(t *testing.T) {
	p := &errSeqProvider{seq: []error{
		apperrors.ExternalService("transient"),
		apperrors.ExternalService("transient"),
		apperrors.ExternalService("transient"),
		apperrors.ExternalService("still transient"),
	}}
	eng := newRetryEngine(p)
	zeroWaits := []time.Duration{0, 0, 0}

	_, err := eng.sendMessageWithRetry(llm.Request{}, 3, zeroWaits)
	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}
	if !apperrors.IsKind(err, apperrors.KindExternalService) {
		t.Errorf("expected last KindExternalService to surface, got %v", err)
	}
	if p.calls != 4 {
		t.Errorf("expected 4 calls (1 initial + 3 retries), got %d", p.calls)
	}
}
