package audit

import (
	"sync"
	"time"

	"github.com/archguard/project/shared/models"
)

type Log struct {
	mu      sync.Mutex
	entries []models.AuditEntry
}

func NewLog() *Log { return &Log{} }

func (l *Log) Record(toolName, args, decision string, size int, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := models.AuditEntry{
		Timestamp: time.Now(), ToolName: toolName,
		Arguments: args, Decision: decision, ResultSize: size,
	}
	if err != nil {
		entry.Error = err.Error()
	}
	l.entries = append(l.entries, entry)
}

func (l *Log) Entries() []models.AuditEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]models.AuditEntry, len(l.entries))
	copy(out, l.entries)
	return out
}
