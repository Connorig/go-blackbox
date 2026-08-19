package notify

import (
	"strings"
	"testing"
)

// TestRegisterAndRender 验证注册与渲染。
func TestRegisterAndRender(t *testing.T) {
	RegisterTemplate("order-paid", "亲爱的{{ user.name }},您的订单 {{order_no}} 已支付,金额 {{ amount }} 元")
	content, ok := Template("order-paid")
	if !ok {
		t.Fatal("template must be registered")
	}
	if !strings.Contains(content, "{{ user.name }}") {
		t.Fatalf("unexpected template content: %s", content)
	}

	rendered, err := RenderTemplate("order-paid", map[string]interface{}{
		"user":     map[string]interface{}{"name": "张三"},
		"order_no": "ORD-20260819-001",
		"amount":   99.5,
	})
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	expected := "亲爱的张三,您的订单 ORD-20260819-001 已支付,金额 99.5 元"
	if rendered != expected {
		t.Fatalf("unexpected render: %q, want %q", rendered, expected)
	}
}

// TestRenderMissingParams 验证缺失参数报错并列出键。
func TestRenderMissingParams(t *testing.T) {
	RegisterTemplate("missing-test", "您好 {{ name }},验证码 {{ code }},有效期 {{ expire }} 分钟")
	_, err := RenderTemplate("missing-test", map[string]interface{}{"name": "李四"})
	if err == nil {
		t.Fatal("missing params must return error")
	}
	if !strings.Contains(err.Error(), "code") || !strings.Contains(err.Error(), "expire") {
		t.Fatalf("error must list missing keys, got: %v", err)
	}
	if strings.Contains(err.Error(), "name") {
		t.Fatalf("provided key must not be listed as missing, got: %v", err)
	}
}

// TestRenderNotRegistered 验证未注册模板报错。
func TestRenderNotRegistered(t *testing.T) {
	if _, err := RenderTemplate("no-such-template", nil); err == nil {
		t.Fatal("unregistered template must return error")
	}
	if _, ok := Template("no-such-template"); ok {
		t.Fatal("unregistered template must not be found")
	}
}

// TestRenderInline 验证直接渲染(无需注册)。
func TestRenderInline(t *testing.T) {
	rendered, err := Render("订单 {{ id }} 已发货,{{ blank }}", map[string]interface{}{"id": 10086, "blank": ""})
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if rendered != "订单 10086 已发货," {
		t.Fatalf("unexpected inline render: %q", rendered)
	}
}

// TestRenderEdgeCases 验证边界:空模板/空格占位符/嵌套缺失。
func TestRenderEdgeCases(t *testing.T) {
	rendered, err := Render("", nil)
	if err != nil || rendered != "" {
		t.Fatalf("empty template must render empty, got %q err %v", rendered, err)
	}

	// 嵌套键缺失:父级不是 map
	_, err = Render("{{a.b}}", map[string]interface{}{"a": "not-a-map"})
	if err == nil {
		t.Fatal("nested key over scalar must fail")
	}

	// 占位符空格形式
	rendered, err = Render("值:{{ key }}", map[string]interface{}{"key": "v"})
	if err != nil || rendered != "值:v" {
		t.Fatalf("spaced placeholder render failed: %q err %v", rendered, err)
	}
}

// TestRegisterOverride 验证同名覆盖。
func TestRegisterOverride(t *testing.T) {
	RegisterTemplate("override-me", "v1 内容")
	RegisterTemplate("override-me", "v2 内容")
	content, _ := Template("override-me")
	if content != "v2 内容" {
		t.Fatalf("template must be overridden, got %q", content)
	}
	RegisterTemplate("", "ignored") // 空名忽略,不 panic
}
