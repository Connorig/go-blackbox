package sensitive

import (
	"strings"
	"testing"
)

// TestFilterContains 基础命中。
func TestFilterContains(t *testing.T) {
	filter := NewFilter([]string{"赌博", "色情", "广告"})
	cases := map[string]bool{
		"这条弹幕包含赌博内容": true,
		"正常聊天":       false,
		"广告词":        true,
		"":           false,
	}
	for text, want := range cases {
		if got := filter.Contains(text); got != want {
			t.Errorf("Contains(%q) = %v, want %v", text, got, want)
		}
	}
}

// TestFilterFind 命中词列表(多词、嵌套词)。
func TestFilterFind(t *testing.T) {
	filter := NewFilter([]string{"赌博", "赌博网站", "色情"})
	hits := filter.Find("这个赌博网站有色情内容")
	if len(hits) != 2 {
		t.Fatalf("hits = %v", hits)
	}
	if hits[0] != "赌博网站" || hits[1] != "色情" {
		t.Fatalf("hits = %v", hits)
	}
}

// TestFilterReplace 敏感词打码。
func TestFilterReplace(t *testing.T) {
	filter := NewFilter([]string{"赌博", "色情"})
	masked := filter.Replace("赌博和色情都不行", '*')
	if masked != "**和**都不行" {
		t.Fatalf("masked = %q", masked)
	}
}

// TestFilterValidate 校验通过/不通过。
func TestFilterValidate(t *testing.T) {
	filter := NewFilter([]string{"诈骗"})
	ok, hits := filter.Validate("正常内容")
	if !ok || hits != nil {
		t.Fatalf("ok=%v hits=%v", ok, hits)
	}
	ok, hits = filter.Validate("这是诈骗信息")
	if ok || len(hits) != 1 || hits[0] != "诈骗" {
		t.Fatalf("ok=%v hits=%v", ok, hits)
	}
}

// TestFilterDynamicAdd 动态添加词条。
func TestFilterDynamicAdd(t *testing.T) {
	filter := NewFilter([]string{"旧词"})
	if !filter.Contains("旧词") {
		t.Fatal("initial word missing")
	}
	filter.Add("新词")
	if !filter.Contains("新词") {
		t.Fatal("added word missing")
	}
	if filter.Size() != 2 {
		t.Fatalf("size = %d", filter.Size())
	}
	filter.Add("旧词") // 重复添加不重复计数
	if filter.Size() != 2 {
		t.Fatalf("size after dup = %d", filter.Size())
	}
}

// TestFilterNilSafe nil 安全。
func TestFilterNilSafe(t *testing.T) {
	var filter *Filter
	if filter.Contains("x") || filter.Size() != 0 || len(filter.Find("x")) != 0 {
		t.Fatal("nil filter must be safe")
	}
	if filter.Replace("x", '*') != "x" {
		t.Fatal("nil replace must return original")
	}
	if ok, _ := filter.Validate("x"); !ok {
		t.Fatal("nil validate must pass")
	}
}

// TestFilterPrefixNested 前缀嵌套词(赌博/赌博网站)最长匹配。
func TestFilterPrefixNested(t *testing.T) {
	filter := NewFilter([]string{"赌博", "赌博网站"})
	hits := filter.Find("赌博网站")
	if len(hits) != 1 || hits[0] != "赌博网站" {
		t.Fatalf("hits = %v", hits)
	}
}

// TestFilterUnicodeAndSpaces 中文与空格干扰场景。
func TestFilterUnicodeAndSpaces(t *testing.T) {
	filter := NewFilter([]string{"加微信"})
	if !filter.Contains("加 微 信") {
		t.Fatal("space-separated must match")
	}
	if strings.Contains(filter.Replace("加微信加微信", '*'), "加微信") {
		t.Fatal("replace failed")
	}
}
