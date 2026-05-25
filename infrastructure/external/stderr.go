package external

import (
	"strings"
	"sync"
)

type stderrTail struct {
	mu  sync.Mutex
	buf []byte
	cap int
}

func newStderrTail(cap int) *stderrTail {
	return &stderrTail{
		cap: cap,
		buf: make([]byte, 0, cap*2),
	}
}

func (s *stderrTail) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.buf = append(s.buf, p...)
	if len(s.buf) > s.cap*2 {
		s.buf = append(s.buf[:0], s.buf[len(s.buf)-s.cap:]...)
	}
	return len(p), nil
}

func (s *stderrTail) String() string {
	if s == nil {
		return ""
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.buf) <= s.cap {
		return strings.TrimSpace(string(s.buf))
	}
	tail := s.buf[len(s.buf)-s.cap:]
	if nl := strings.IndexByte(string(tail), '\n'); nl >= 0 && nl < len(tail)-1 {
		tail = tail[nl+1:]
	}
	return strings.TrimSpace(string(tail))
}
