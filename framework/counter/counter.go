// Package counter 提供高并发在线计数:直播间人数/峰值/时段统计。
// 内存计数(Inc/Dec 高频安全)+ 峰值追踪;按房间注册表。
// 场景:直播在线人数、房间热度、峰值记录;持久化由业务定时快照(如每 30s 落库)。
package counter

import "sync"

// Counter 单实例计数器(线程安全)。
type Counter struct {
	mu    sync.Mutex
	value int64
	peak  int64
}

// New 创建计数器。
func New() *Counter {
	return &Counter{}
}

// Inc 自增,返回当前值。
func (c *Counter) Inc() int64 {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value++
	if c.value > c.peak {
		c.peak = c.value
	}
	return c.value
}

// Dec 自减(下限 0),返回当前值。
func (c *Counter) Dec() int64 {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.value > 0 {
		c.value--
	}
	return c.value
}

// Value 返回当前值。
func (c *Counter) Value() int64 {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value
}

// Peak 返回历史峰值。
func (c *Counter) Peak() int64 {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.peak
}

// ResetPeak 重置峰值(新的统计周期),返回重置前的峰值。
func (c *Counter) ResetPeak() int64 {
	if c == nil {
		return 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	old := c.peak
	c.peak = c.value
	return old
}

// Snapshot 快照(当前值 + 峰值)。
func (c *Counter) Snapshot() (value, peak int64) {
	if c == nil {
		return 0, 0
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.value, c.peak
}

// Registry 按房间(键)管理的计数器注册表。
// 典型用法:直播间在线人数,key = 房间号。
type Registry struct {
	mu       sync.RWMutex
	counters map[string]*Counter
}

// NewRegistry 创建注册表。
func NewRegistry() *Registry {
	return &Registry{counters: make(map[string]*Counter)}
}

// Inc 自增指定键的计数器,返回当前值。
func (r *Registry) Inc(key string) int64 {
	return r.counter(key).Inc()
}

// Dec 自减指定键的计数器,返回当前值。
func (r *Registry) Dec(key string) int64 {
	return r.counter(key).Dec()
}

// Value 返回指定键的当前值(不存在为 0)。
func (r *Registry) Value(key string) int64 {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	counter := r.counters[key]
	r.mu.RUnlock()
	if counter == nil {
		return 0
	}
	return counter.Value()
}

// Peak 返回指定键的峰值(不存在为 0)。
func (r *Registry) Peak(key string) int64 {
	if r == nil {
		return 0
	}
	r.mu.RLock()
	counter := r.counters[key]
	r.mu.RUnlock()
	if counter == nil {
		return 0
	}
	return counter.Peak()
}

// Keys 返回全部已注册键。
func (r *Registry) Keys() []string {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	keys := make([]string, 0, len(r.counters))
	for key := range r.counters {
		keys = append(keys, key)
	}
	return keys
}

// Remove 移除指定键的计数器。
func (r *Registry) Remove(key string) {
	if r == nil {
		return
	}
	r.mu.Lock()
	delete(r.counters, key)
	r.mu.Unlock()
}

// counter 获取或创建计数器。
func (r *Registry) counter(key string) *Counter {
	r.mu.RLock()
	counter := r.counters[key]
	r.mu.RUnlock()
	if counter != nil {
		return counter
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if counter = r.counters[key]; counter == nil {
		counter = New()
		r.counters[key] = counter
	}
	return counter
}
