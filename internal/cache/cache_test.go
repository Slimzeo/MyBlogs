package cache

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSetNXIsAtomic(t *testing.T) {
	cache := New()
	defer cache.Close()

	var successes atomic.Int64
	var waitGroup sync.WaitGroup
	for range 100 {
		waitGroup.Add(1)
		go func() {
			defer waitGroup.Done()
			if cache.SetNX("frequency", 1, 60) {
				successes.Add(1)
			}
		}()
	}
	waitGroup.Wait()

	if successes.Load() != 1 {
		t.Fatalf("SetNX successes = %d, want 1", successes.Load())
	}
}

func TestExpiredValueCanBeReplaced(t *testing.T) {
	cache := New()
	defer cache.Close()

	cache.Set("short", 1, 1)
	time.Sleep(1100 * time.Millisecond)
	if !cache.SetNX("short", 2, 1) {
		t.Fatal("SetNX rejected an expired value")
	}
	value, exists := cache.GetInt("short")
	if !exists || value != 2 {
		t.Fatalf("value = %d, exists = %v; want 2, true", value, exists)
	}
}
