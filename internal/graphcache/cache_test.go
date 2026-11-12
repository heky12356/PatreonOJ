package graphcache

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestCoalescesConcurrentLoadsAndDoesNotCacheErrors(t *testing.T) {
	c := New()
	var calls atomic.Int32
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body, _, err := c.Get(context.Background(), "same", func() ([]byte, error) { calls.Add(1); time.Sleep(20 * time.Millisecond); return []byte("graph"), nil })
			if err != nil || string(body) != "graph" {
				t.Errorf("bad response: %s %v", body, err)
			}
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("built %d times", calls.Load())
	}
	for range 2 {
		_, _, err := c.Get(context.Background(), "error", func() ([]byte, error) { calls.Add(1); return nil, errors.New("temporary") })
		if err == nil {
			t.Fatal("error swallowed")
		}
	}
	if calls.Load() != 3 {
		t.Fatal("failed loads were cached")
	}
}

func TestCacheCapacityAndRevision(t *testing.T) {
	c := New()
	payload := make([]byte, 200<<10)
	for i := range 100 {
		if _, _, err := c.Get(context.Background(), fmt.Sprint(i), func() ([]byte, error) { return payload, nil }); err != nil {
			t.Fatal(err)
		}
	}
	if len(c.items) > 64 || c.bytes > 8<<20 {
		t.Fatal("unbounded cache")
	}
	old := Revision()
	Invalidate()
	if Revision() <= old {
		t.Fatal("revision did not advance")
	}
}
