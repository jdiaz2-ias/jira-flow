// Package cache implements bounded, process-local storage. No issue data is written to disk.
package cache

import (
	"sync"
	"time"
)

type entry struct {
	Value []byte
	At    time.Time
	TTL   time.Duration
}
type Memory struct {
	mu      sync.Mutex
	entries map[string]entry
	bytes   int
	Now     func() time.Time
}

func New() *Memory { return &Memory{entries: map[string]entry{}, Now: time.Now} }
func (m *Memory) Get(key string, allowStale bool) ([]byte, time.Time, bool, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, ok := m.entries[key]
	if !ok {
		return nil, time.Time{}, false, false
	}
	stale := m.Now().Sub(e.At) >= e.TTL
	if stale && !allowStale {
		return nil, time.Time{}, false, false
	}
	return append([]byte(nil), e.Value...), e.At, stale, true
}
func (m *Memory) Put(key string, value []byte, ttl time.Duration) {
	if len(value) > 4*1024*1024 {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if old, ok := m.entries[key]; ok {
		m.bytes -= len(old.Value)
		delete(m.entries, key)
	}
	for len(m.entries) >= 128 || m.bytes+len(value) > 16*1024*1024 {
		var oldest string
		var at time.Time
		for k, e := range m.entries {
			if oldest == "" || e.At.Before(at) {
				oldest = k
				at = e.At
			}
		}
		m.bytes -= len(m.entries[oldest].Value)
		delete(m.entries, oldest)
	}
	m.entries[key] = entry{Value: append([]byte(nil), value...), At: m.Now(), TTL: ttl}
	m.bytes += len(value)
}
func (m *Memory) DeletePrefix(prefix string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, e := range m.entries {
		if len(k) >= len(prefix) && k[:len(prefix)] == prefix {
			m.bytes -= len(e.Value)
			delete(m.entries, k)
		}
	}
}
