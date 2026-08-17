package counter

import (
	"sync"
	"testing"
)

// TestCounterBasic Inc/Dec/Value。
func TestCounterBasic(t *testing.T) {
	counter := New()
	if counter.Inc() != 1 || counter.Inc() != 2 {
		t.Fatal("inc failed")
	}
	if counter.Dec() != 1 {
		t.Fatal("dec failed")
	}
	if counter.Value() != 1 {
		t.Fatalf("value = %d", counter.Value())
	}
}

// TestCounterPeak 峰值追踪与重置。
func TestCounterPeak(t *testing.T) {
	counter := New()
	counter.Inc()
	counter.Inc()
	counter.Inc()
	if counter.Peak() != 3 {
		t.Fatalf("peak = %d", counter.Peak())
	}
	counter.Dec()
	if counter.Peak() != 3 {
		t.Fatal("peak must keep history")
	}
	if old := counter.ResetPeak(); old != 3 {
		t.Fatalf("reset peak = %d", old)
	}
	if counter.Peak() != 2 {
		t.Fatalf("peak after reset = %d", counter.Peak())
	}
}

// TestCounterNeverNegative Dec 下限 0。
func TestCounterNeverNegative(t *testing.T) {
	counter := New()
	counter.Dec()
	if counter.Value() != 0 {
		t.Fatal("must not go negative")
	}
}

// TestCounterConcurrent 并发 Inc/Dec 安全:
// 下限保护使 value=0 时的 Dec 被忽略,交错执行下有效 Dec 数不定,
// 断言值在合法范围且峰值不超过 Inc 总数。
func TestCounterConcurrent(t *testing.T) {
	counter := New()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() { defer wg.Done(); counter.Inc() }()
		go func() { defer wg.Done(); counter.Dec() }()
	}
	wg.Wait()
	value, peak := counter.Snapshot()
	if value < 0 || value > 50 {
		t.Fatalf("value=%d out of range [0,50]", value)
	}
	if peak < 1 || peak > 50 {
		t.Fatalf("peak=%d out of range [1,50]", peak)
	}
}

// TestCounterSequentialBalance 顺序执行 50 Inc + 50 Dec 精确归零。
func TestCounterSequentialBalance(t *testing.T) {
	counter := New()
	for i := 0; i < 50; i++ {
		counter.Inc()
	}
	for i := 0; i < 50; i++ {
		counter.Dec()
	}
	if counter.Value() != 0 {
		t.Fatalf("value = %d, want 0", counter.Value())
	}
	if counter.Peak() != 50 {
		t.Fatalf("peak = %d, want 50", counter.Peak())
	}
}

// TestRegistryRoomCounters 按房间计数。
func TestRegistryRoomCounters(t *testing.T) {
	registry := NewRegistry()
	if registry.Inc("live-1") != 1 || registry.Inc("live-1") != 2 {
		t.Fatal("room inc failed")
	}
	if registry.Inc("live-2") != 1 {
		t.Fatal("room2 inc failed")
	}
	if registry.Value("live-1") != 2 || registry.Value("live-2") != 1 {
		t.Fatalf("values = %d %d", registry.Value("live-1"), registry.Value("live-2"))
	}
	if registry.Value("missing") != 0 || registry.Peak("missing") != 0 {
		t.Fatal("missing room must be 0")
	}
	keys := registry.Keys()
	if len(keys) != 2 {
		t.Fatalf("keys = %v", keys)
	}
	registry.Remove("live-2")
	if registry.Value("live-2") != 0 || len(registry.Keys()) != 1 {
		t.Fatal("remove failed")
	}
}
