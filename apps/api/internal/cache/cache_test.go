package cache

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestMemoryCacheCopiesAndExpires(t *testing.T) {
	t.Parallel()
	c := NewMemory()
	value := []byte("value")
	if err := c.Set(context.Background(), "key", value, 5*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	value[0] = 'X'
	got, err := c.Get(context.Background(), "key")
	if err != nil || string(got) != "value" {
		t.Fatalf("got %q, err=%v", got, err)
	}
	time.Sleep(10 * time.Millisecond)
	if _, err := c.Get(context.Background(), "key"); !errors.Is(err, ErrMiss) {
		t.Fatalf("expected ErrMiss, got %v", err)
	}
}
