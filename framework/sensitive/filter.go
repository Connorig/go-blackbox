// Package sensitive 提供敏感词过滤(DFA 确定性有限自动机实现)。
// 特性:单次遍历 O(n) 匹配全部词条;内存占用与词条数线性;并发安全(构建后只读)。
// 场景:弹幕风控、昵称审核、评论过滤、内容安全。
package sensitive

import (
	"strings"
	"sync"
	"unicode"
)

// Filter 敏感词过滤器(DFA)。
// 构建后并发安全:Add 与查询不可并发(典型用法:启动时构建完成再服务)。
type Filter struct {
	mu   sync.RWMutex
	root *node
	size int
}

// node DFA 节点。
type node struct {
	children map[rune]*node
	isEnd    bool
}

// newTrieNode 创建节点。
func newTrieNode() *node {
	return &node{children: make(map[rune]*node)}
}

// NewFilter 构建敏感词过滤器。
func NewFilter(words []string) *Filter {
	filter := &Filter{root: newTrieNode()}
	for _, word := range words {
		filter.add(strings.TrimSpace(word))
	}
	return filter
}

// Add 动态添加词条(空词忽略)。
func (f *Filter) Add(word string) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.add(strings.TrimSpace(word))
}

// add 内部添加(调用方持锁)。
func (f *Filter) add(word string) {
	if word == "" {
		return
	}
	current := f.root
	for _, char := range word {
		next, exists := current.children[char]
		if !exists {
			next = newTrieNode()
			current.children[char] = next
		}
		current = next
	}
	if !current.isEnd {
		current.isEnd = true
		f.size++
	}
}

// Size 返回词条数量。
func (f *Filter) Size() int {
	if f == nil {
		return 0
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	return f.size
}

// Contains 判断文本是否包含敏感词。
func (f *Filter) Contains(text string) bool {
	if f == nil {
		return false
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	_, matched := f.scan(text)
	return matched
}

// Find 返回文本中命中的全部敏感词(去重,顺序按出现位置)。
func (f *Filter) Find(text string) []string {
	if f == nil {
		return nil
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	hits, _ := f.scan(text)
	return hits
}

// Replace 将命中的敏感词替换为 mask 字符(默认 '*' 取首字符)。
func (f *Filter) Replace(text string, mask rune) string {
	if f == nil {
		return text
	}
	if mask == 0 {
		mask = '*'
	}
	f.mu.RLock()
	defer f.mu.RUnlock()
	// 标记命中区间
	positions := make([]bool, len([]rune(text)))
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		end := f.matchEnd(runes, i)
		if end > i {
			for j := i; j < end; j++ {
				if !unicode.IsSpace(runes[j]) {
					positions[j] = true
				}
			}
			i = end - 1
		}
	}
	for i := range runes {
		if positions[i] {
			runes[i] = mask
		}
	}
	return string(runes)
}

// Validate 校验文本:通过返回 (true, nil);不通过返回 (false, 命中词列表)。
func (f *Filter) Validate(text string) (bool, []string) {
	hits := f.Find(text)
	if len(hits) == 0 {
		return true, nil
	}
	return false, hits
}

// matchEnd 从位置 start 开始,返回最长匹配的结束位置(未命中返回 start)。
// 空白字符跳过但匹配状态保持,防“加 微 信”类空格绕过。
func (f *Filter) matchEnd(runes []rune, start int) int {
	current := f.root
	end := start
	for i := start; i < len(runes); i++ {
		if unicode.IsSpace(runes[i]) {
			continue
		}
		next, exists := current.children[runes[i]]
		if !exists {
			break
		}
		current = next
		if current.isEnd {
			end = i + 1
		}
	}
	return end
}

// scan 全文本扫描:返回命中词列表与是否命中。
func (f *Filter) scan(text string) ([]string, bool) {
	runes := []rune(text)
	var hits []string
	matched := false
	for i := 0; i < len(runes); i++ {
		end := f.matchEnd(runes, i)
		if end > i {
			hits = append(hits, string(runes[i:end]))
			matched = true
			i = end - 1
		}
	}
	return hits, matched
}
