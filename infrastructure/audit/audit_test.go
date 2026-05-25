package audit

import (
	"testing"
	"time"

	c "github.com/archguard/project/shared/constants"
)

func TestRecordRateLimitAppendsEntry(t *testing.T) {
	log := NewLog()

	log.RecordRateLimit(c.RateLimitKind429)
	log.RecordRateLimit(c.RateLimitKindOverloaded)

	entries := log.Entries()
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	for i, want := range []string{c.RateLimitKind429, c.RateLimitKindOverloaded} {
		if entries[i].ToolName != c.EventRateLimit {
			t.Errorf("entries[%d].ToolName = %q, want %q", i, entries[i].ToolName, c.EventRateLimit)
		}
		if entries[i].Arguments != want {
			t.Errorf("entries[%d].Arguments = %q, want %q", i, entries[i].Arguments, want)
		}
	}
}

func TestRecordRateLimitConcurrent(t *testing.T) {
	log := NewLog()

	done := make(chan struct{}, 8)
	for i := 0; i < 8; i++ {
		go func() {
			for j := 0; j < 25; j++ {
				log.RecordRateLimit(c.RateLimitKind429)
			}
			done <- struct{}{}
		}()
	}
	for i := 0; i < 8; i++ {
		<-done
	}

	if got := len(log.Entries()); got != 200 {
		t.Errorf("entries = %d, want 200 (concurrent record must not lose entries)", got)
	}
}

func TestRecordPreemptiveSleep(t *testing.T) {
	log := NewLog()

	log.RecordPreemptiveSleep(2500 * time.Millisecond)

	entries := log.Entries()
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].ToolName != c.EventPreemptiveSleep {
		t.Errorf("ToolName = %q, want %q", entries[0].ToolName, c.EventPreemptiveSleep)
	}
	if entries[0].Arguments != "2.5s" {
		t.Errorf("Arguments = %q, want %q", entries[0].Arguments, "2.5s")
	}
}
