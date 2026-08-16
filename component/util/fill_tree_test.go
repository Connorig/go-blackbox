package util

import (
	"encoding/json"
	"testing"
)

// menuVO 带 children 字段的业务实体(前端友好模式)。
type menuVO struct {
	ID       int64    `json:"id"`
	ParentID int64    `json:"parent_id"`
	Name     string   `json:"name"`
	Children []menuVO `json:"children"`
}

// TestFillTree 实体填充版树构建(三级)。
func TestFillTree(t *testing.T) {
	menus := []menuVO{
		{ID: 1, ParentID: 0, Name: "系统管理"},
		{ID: 2, ParentID: 0, Name: "订单管理"},
		{ID: 3, ParentID: 1, Name: "用户管理"},
		{ID: 4, ParentID: 1, Name: "角色管理"},
		{ID: 5, ParentID: 3, Name: "用户列表"},
		{ID: 6, ParentID: 3, Name: "用户编辑"},
	}
	roots := FillTree(menus,
		func(m *menuVO) int64 { return m.ID },
		func(m *menuVO) int64 { return m.ParentID },
		func(parent *menuVO, children []menuVO) { parent.Children = children },
		0)
	if len(roots) != 2 {
		t.Fatalf("roots = %d", len(roots))
	}
	if roots[0].Name != "系统管理" || len(roots[0].Children) != 2 {
		t.Fatalf("level1 wrong: %+v", roots[0])
	}
	if roots[0].Children[0].Name != "用户管理" || len(roots[0].Children[0].Children) != 2 {
		t.Fatalf("level2 wrong")
	}
	if roots[0].Children[0].Children[0].Name != "用户列表" {
		t.Fatalf("level3 wrong")
	}
}

// TestFillTreeJSON 前端场景:直接 JSON 输出带 children 的完整树。
func TestFillTreeJSON(t *testing.T) {
	menus := []menuVO{
		{ID: 1, ParentID: 0, Name: "中国"},
		{ID: 2, ParentID: 1, Name: "浙江"},
		{ID: 3, ParentID: 2, Name: "杭州"},
	}
	roots := FillTree(menus,
		func(m *menuVO) int64 { return m.ID },
		func(m *menuVO) int64 { return m.ParentID },
		func(parent *menuVO, children []menuVO) { parent.Children = children },
		0)
	data, err := json.Marshal(roots)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var parsed map[string]interface{}
	_ = json.Unmarshal(data, &parsed)
	// 直接反解验证 children 递归完整
	var decoded []menuVO
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(decoded) != 1 || decoded[0].Name != "中国" ||
		decoded[0].Children[0].Name != "浙江" ||
		decoded[0].Children[0].Children[0].Name != "杭州" {
		t.Fatalf("tree json wrong: %s", data)
	}
}

// TestFillTreeOrphan 孤儿挂根。
func TestFillTreeOrphan(t *testing.T) {
	menus := []menuVO{
		{ID: 1, ParentID: 0, Name: "root"},
		{ID: 9, ParentID: 99, Name: "orphan"},
	}
	roots := FillTree(menus,
		func(m *menuVO) int64 { return m.ID },
		func(m *menuVO) int64 { return m.ParentID },
		func(parent *menuVO, children []menuVO) { parent.Children = children },
		0)
	if len(roots) != 2 {
		t.Fatalf("orphan must be kept: %d", len(roots))
	}
}

// TestFillTreeString 字符串 ID。
func TestFillTreeString(t *testing.T) {
	type category struct {
		Code     string     `json:"code"`
		Parent   string     `json:"parent"`
		Name     string     `json:"name"`
		Children []category `json:"children"`
	}
	categories := []category{
		{Code: "cn", Parent: "", Name: "中国"},
		{Code: "cn-zj", Parent: "cn", Name: "浙江"},
		{Code: "cn-zj-hz", Parent: "cn-zj", Name: "杭州"},
	}
	roots := FillTreeString(categories,
		func(c *category) string { return c.Code },
		func(c *category) string { return c.Parent },
		func(parent *category, children []category) { parent.Children = children },
		"")
	if len(roots) != 1 || roots[0].Children[0].Children[0].Name != "杭州" {
		t.Fatalf("string tree wrong: %+v", roots)
	}
}
