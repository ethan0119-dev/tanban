package cache

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrMiss = errors.New("cache miss")

type Cache interface {
	Get(context.Context, string) ([]byte, error)
	Set(context.Context, string, []byte, time.Duration) error
	Delete(context.Context, string) error
}

type entry struct {
	value     []byte
	expiresAt time.Time
}

type Memory struct {
	mu    sync.RWMutex
	items map[string]entry
}

func NewMemory() *Memory { return &Memory{items: make(map[string]entry)} }

func (m *Memory) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.RLock()
	item, ok := m.items[key]
	m.mu.RUnlock()
	if !ok || (!item.expiresAt.IsZero() && time.Now().After(item.expiresAt)) {
		if ok {
			_ = m.Delete(context.Background(), key)
		}
		return nil, ErrMiss
	}
	return append([]byte(nil), item.value...), nil
}

func (m *Memory) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	item := entry{value: append([]byte(nil), value...)}
	if ttl > 0 {
		item.expiresAt = time.Now().Add(ttl)
	}
	m.mu.Lock()
	m.items[key] = item
	m.mu.Unlock()
	return nil
}

func (m *Memory) Delete(_ context.Context, key string) error {
	m.mu.Lock()
	delete(m.items, key)
	m.mu.Unlock()
	return nil
}

// Redis is the extension point for a distributed cache. It intentionally does
// not open a network connection in v1; selecting it fails explicitly.
type Redis struct{ Addr string }

func (r Redis) Get(context.Context, string) ([]byte, error) { return nil, ErrMiss }
func (r Redis) Set(context.Context, string, []byte, time.Duration) error {
	return errors.New("redis cache adapter is not enabled in this build")
}
func (r Redis) Delete(context.Context, string) error {
	return errors.New("redis cache adapter is not enabled in this build")
}
