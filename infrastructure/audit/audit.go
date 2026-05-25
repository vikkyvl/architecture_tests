package audit

import (
	"sync"
	"time"

	c "github.com/archguard/project/shared/constants"
	"github.com/archguard/project/shared/models"
)

type Log struct {
	mu      sync.Mutex
	entries []models.AuditEntry
}

func NewLog() *Log {
	return new(Log)
}

func (l *Log) Record(toolName, args, decision string, size int, err error, dedup bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := models.AuditEntry{
		Timestamp: time.Now(), ToolName: toolName,
		Arguments: args, Decision: decision, ResultSize: size,
		Dedup: dedup,
	}
	if err != nil {
		entry.Error = err.Error()
	}
	l.entries = append(l.entries, entry)
}

func (l *Log) RecordLLMTurn(input, cached, cacheWrite, pruned int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = append(l.entries, models.AuditEntry{
		Timestamp:             time.Now(),
		ToolName:              c.EventLLMTurn,
		InputTokens:           input,
		CachedInputTokens:     cached,
		CacheWriteInputTokens: cacheWrite,
		PrunedTurns:           pruned,
	})
}

func (l *Log) RecordRateLimit(kind string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = append(l.entries, models.AuditEntry{
		Timestamp: time.Now(),
		ToolName:  c.EventRateLimit,
		Arguments: kind,
	})
}

func (l *Log) RecordPreemptiveSleep(wait time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.entries = append(l.entries, models.AuditEntry{
		Timestamp: time.Now(),
		ToolName:  c.EventPreemptiveSleep,
		Arguments: wait.String(),
	})
}

func (l *Log) Entries() []models.AuditEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]models.AuditEntry, len(l.entries))
	copy(out, l.entries)
	return out
}
