package apidoc

import (
	"strings"
	"testing"
)

// TestRenderPagePrefix 文档页 fetch 路径注入前缀,无尾斜杠 URL 下也能加载 api.json。
func TestRenderPagePrefix(t *testing.T) {
	page := renderPage("/docs")
	for _, want := range []string{
		`fetch("/docs/api.json")`,
		`<!DOCTYPE html>`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("page missing %q", want)
		}
	}
	if strings.Contains(page, `fetch("api.json")`) || strings.Contains(page, `fetch("/api.json")`) {
		t.Error("page must fetch api.json with prefix")
	}
}

// TestRenderPageMethodCase OpenAPI PathItem 方法 key 为小写,
// JS 必须用小写 key 取值(大写取值导致接口列表永远为空)。
func TestRenderPageMethodCase(t *testing.T) {
	page := renderPage("/docs")
	if !strings.Contains(page, "var op = item[key];") {
		t.Error("JS must read item with lowercase method key")
	}
	if strings.Contains(page, "item[methods[key]]") {
		t.Error("JS must not read item with uppercase method key")
	}
}

// TestRenderPageVersionDisplay 版本号显示不重复 v 前缀(v1 vs 1)。
func TestRenderPageVersionDisplay(t *testing.T) {
	page := renderPage("/docs")
	if !strings.Contains(page, "shownVersion") {
		t.Error("page must handle version prefix")
	}
	if strings.Contains(page, `" · v" + spec.info.version`) {
		t.Error("page must not blindly prepend v to version")
	}
}

// TestRenderPageFallback 空接口时的提示文案保留。
func TestRenderPageFallback(t *testing.T) {
	page := renderPage("/docs")
	if !strings.Contains(page, "暂无接口文档") {
		t.Error("page must keep empty-state hint")
	}
}
