package id

import (
	"errors"
	"regexp"
	"sync"
	"testing"
	"time"
)

var errUnexpectedNonPositive = errors.New("unexpected non-positive id")

// TestSnowflakeUniqueAndPositive 验证并发唯一性与正数。
func TestSnowflakeUniqueAndPositive(t *testing.T) {
	const workers = 8
	const perWorker = 2000

	var wg sync.WaitGroup
	results := make(chan int64, workers*perWorker)
	errs := make(chan error, workers)

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(node int64) {
			defer wg.Done()
			// 每个 goroutine 独立生成器,模拟多实例
			gen := &snowflake{node: node}
			for i := 0; i < perWorker; i++ {
				value, err := gen.nextID()
				if err != nil {
					errs <- err
					return
				}
				if value <= 0 {
					errs <- errUnexpectedNonPositive
					return
				}
				results <- value
			}
		}(int64(w % 1024))
	}
	wg.Wait()
	close(results)
	close(errs)

	for err := range errs {
		t.Fatalf("generate failed: %v", err)
	}

	seen := make(map[int64]bool, workers*perWorker)
	for value := range results {
		if seen[value] {
			t.Fatalf("duplicate snowflake id: %d", value)
		}
		seen[value] = true
	}
}

// TestSetNodeOutOfRange 节点越界返回错误。
func TestSetNodeOutOfRange(t *testing.T) {
	if err := SetNode(-1); err == nil {
		t.Fatal("SetNode(-1) must fail")
	}
	if err := SetNode(1024); err == nil {
		t.Fatal("SetNode(1024) must fail")
	}
}

// TestParseSnowflakeRoundTrip 验证解析回读。
func TestParseSnowflakeRoundTrip(t *testing.T) {
	if err := SetNode(7); err != nil {
		t.Fatalf("SetNode failed: %v", err)
	}
	value, err := NextID()
	if err != nil {
		t.Fatalf("NextID failed: %v", err)
	}
	timestamp, node, sequence, err := ParseSnowflake(value)
	if err != nil {
		t.Fatalf("ParseSnowflake failed: %v", err)
	}
	if node != 7 {
		t.Fatalf("node = %d, want 7", node)
	}
	if sequence < 0 || sequence > snowflakeMaxSeq {
		t.Fatalf("sequence out of range: %d", sequence)
	}
	if timestamp.After(time.Now()) {
		t.Fatal("parsed timestamp must not be in the future")
	}
}

// TestUUIDFormat 验证 UUID v4 格式。
func TestUUIDFormat(t *testing.T) {
	pattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	for i := 0; i < 500; i++ {
		value, err := UUID()
		if err != nil {
			t.Fatalf("UUID failed: %v", err)
		}
		if !pattern.MatchString(value) {
			t.Fatalf("uuid %q does not match v4 pattern", value)
		}
	}
}

// TestUUIDUnique 验证批量唯一性。
func TestUUIDUnique(t *testing.T) {
	seen := make(map[string]bool, 2000)
	for i := 0; i < 2000; i++ {
		value, err := UUID()
		if err != nil {
			t.Fatalf("UUID failed: %v", err)
		}
		if seen[value] {
			t.Fatalf("duplicate uuid: %s", value)
		}
		seen[value] = true
	}
}
