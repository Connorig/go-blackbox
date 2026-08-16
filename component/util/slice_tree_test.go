package util

import (
	"testing"
)

// TestSliceContains 包含判断。
func TestSliceContains(t *testing.T) {
	list := []string{"a", "b", "c"}
	if !SliceContains(list, "b") || SliceContains(list, "z") {
		t.Fatal("contains wrong")
	}
	if !SliceContains([]int{1, 2, 3}, 2) {
		t.Fatal("int contains wrong")
	}
}

// TestSliceFilterMap 过滤与映射。
func TestSliceFilterMap(t *testing.T) {
	nums := []int{1, 2, 3, 4, 5}
	even := SliceFilter(nums, func(n int) bool { return n%2 == 0 })
	if len(even) != 2 || even[0] != 2 || even[1] != 4 {
		t.Fatalf("filter = %v", even)
	}
	doubled := SliceMap(nums, func(n int) int { return n * 2 })
	if len(doubled) != 5 || doubled[4] != 10 {
		t.Fatalf("map = %v", doubled)
	}
}

// TestSliceRemove 移除。
func TestSliceRemove(t *testing.T) {
	list := []string{"a", "b", "c"}
	result := SliceRemove(list, "b")
	if len(result) != 2 || result[0] != "a" || result[1] != "c" {
		t.Fatalf("remove = %v", result)
	}
	// 原切片不变
	if len(list) != 3 {
		t.Fatal("original must not change")
	}
}

// TestSliceFirst 首个匹配。
func TestSliceFirst(t *testing.T) {
	first, ok := SliceFirst([]int{1, 2, 3}, func(n int) bool { return n > 1 })
	if !ok || first != 2 {
		t.Fatalf("first = %d %v", first, ok)
	}
	if _, ok := SliceFirst([]int{1}, func(n int) bool { return n > 9 }); ok {
		t.Fatal("must not found")
	}
}

// TestSliceGroupBy 分组。
func TestSliceGroupBy(t *testing.T) {
	type row struct {
		Group string
		Value int
	}
	rows := []row{{"a", 1}, {"b", 2}, {"a", 3}}
	grouped := SliceGroupBy(rows, func(r row) string { return r.Group })
	if len(grouped["a"]) != 2 || len(grouped["b"]) != 1 {
		t.Fatalf("grouped = %v", grouped)
	}
}

// TestMapKeysValues 键值提取。
func TestMapKeysValues(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2, "c": 3}
	keys := MapKeys(m)
	if len(keys) != 3 || !SliceContains(keys, "b") {
		t.Fatalf("keys = %v", keys)
	}
	values := MapValues(m)
	if len(values) != 3 || !SliceContains(values, 2) {
		t.Fatalf("values = %v", values)
	}
}

// menu 菜单实体(测试用)。
type menu struct {
	ID       int64
	ParentID int64
	Name     string
}

// TestBuildTree 三级树构建。
func TestBuildTree(t *testing.T) {
	menus := []menu{
		{ID: 1, ParentID: 0, Name: "系统管理"},
		{ID: 2, ParentID: 0, Name: "订单管理"},
		{ID: 3, ParentID: 1, Name: "用户管理"},
		{ID: 4, ParentID: 1, Name: "角色管理"},
		{ID: 5, ParentID: 3, Name: "用户列表"},
		{ID: 6, ParentID: 3, Name: "用户编辑"},
	}
	trees := BuildTree(menus,
		func(m menu) int64 { return m.ID },
		func(m menu) int64 { return m.ParentID },
		0)
	if len(trees) != 2 {
		t.Fatalf("roots = %d", len(trees))
	}
	if trees[0].Node.Name != "系统管理" || len(trees[0].Children) != 2 {
		t.Fatalf("level1 wrong: %s children=%d", trees[0].Node.Name, len(trees[0].Children))
	}
	userMgmt := trees[0].Children[0]
	if userMgmt.Node.Name != "用户管理" || len(userMgmt.Children) != 2 {
		t.Fatalf("level2 wrong: %+v", userMgmt.Node)
	}
	if userMgmt.Children[0].Node.Name != "用户列表" {
		t.Fatalf("level3 wrong: %+v", userMgmt.Children[0].Node)
	}
}

// TestBuildTreeOrphan 孤儿节点不丢失(挂根)。
func TestBuildTreeOrphan(t *testing.T) {
	menus := []menu{
		{ID: 1, ParentID: 0, Name: "root"},
		{ID: 9, ParentID: 99, Name: "orphan"},
	}
	trees := BuildTree(menus,
		func(m menu) int64 { return m.ID },
		func(m menu) int64 { return m.ParentID },
		0)
	if len(trees) != 2 {
		t.Fatalf("orphan must be kept: %d roots", len(trees))
	}
}

// TestBuildTreeString 字符串 ID 树。
func TestBuildTreeString(t *testing.T) {
	type category struct {
		Code     string
		Parent   string
		Name     string
	}
	categories := []category{
		{Code: "cn", Parent: "", Name: "中国"},
		{Code: "cn-zj", Parent: "cn", Name: "浙江"},
		{Code: "cn-zj-hz", Parent: "cn-zj", Name: "杭州"},
	}
	trees := BuildTreeString(categories,
		func(c category) string { return c.Code },
		func(c category) string { return c.Parent },
		"")
	if len(trees) != 1 || trees[0].Node.Name != "中国" {
		t.Fatalf("string tree roots wrong")
	}
	if trees[0].Children[0].Node.Name != "浙江" ||
		trees[0].Children[0].Children[0].Node.Name != "杭州" {
		t.Fatalf("string tree nested wrong")
	}
}

// TestFlattenWalk 拍平与遍历。
func TestFlattenWalk(t *testing.T) {
	menus := []menu{
		{ID: 1, ParentID: 0, Name: "a"},
		{ID: 2, ParentID: 1, Name: "b"},
		{ID: 3, ParentID: 2, Name: "c"},
	}
	trees := BuildTree(menus,
		func(m menu) int64 { return m.ID },
		func(m menu) int64 { return m.ParentID },
		0)
	flattened := FlattenTree(trees)
	if len(flattened) != 3 || flattened[0].Name != "a" || flattened[2].Name != "c" {
		t.Fatalf("flatten = %v", flattened)
	}
	var names []string
	WalkTree(trees, func(node menu, depth int) {
		names = append(names, node.Name)
	})
	if len(names) != 3 || names[2] != "c" {
		t.Fatalf("walk = %v", names)
	}
}
