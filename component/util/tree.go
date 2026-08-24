package util

// 树形结构构建(评论回复、国家地区、角色菜单等 N 级树)。

// Tree 树节点包装:保留原始实体 + 子节点列表。
type Tree[T any] struct {
	Node     T          `json:"node"`     // 原始实体
	Children []*Tree[T] `json:"children"` // 子节点(N 级递归)
}

// BuildTree 将扁平集合构建为树形结构(泛型,任意实体)。
// idFn/parentFn 分别提取节点 ID 与父 ID;rootParentID 为根节点的父 ID
// (通常传 0 或 -1,按业务约定)。
//
// 用法:
//
//	type Menu struct { ID int64; ParentID int64; Name string }
//	trees := util.BuildTree(menus,
//	    func(m Menu) int64 { return m.ID },
//	    func(m Menu) int64 { return m.ParentID },
//	    0)
//	// trees[i].Node.Name 为根菜单,trees[i].Children 递归子菜单
//
// 特性:
//   - 支持任意层级(N 级)
//   - 父节点不存在/孤儿节点挂在根下(不丢失数据)
//   - 保持输入顺序稳定(先序遍历)
func BuildTree[T any](list []T, idFn func(T) int64, parentFn func(T) int64, rootParentID int64) []*Tree[T] {
	if len(list) == 0 {
		return nil
	}
	// 一次遍历建索引:ID → 节点
	nodeMap := make(map[int64]*Tree[T], len(list))
	for _, item := range list {
		nodeMap[idFn(item)] = &Tree[T]{Node: item}
	}
	// 第二次遍历挂父子关系
	var roots []*Tree[T]
	for _, item := range list {
		node := nodeMap[idFn(item)]
		parent := nodeMap[parentFn(item)]
		if parent == nil || parentFn(item) == rootParentID {
			roots = append(roots, node)
			continue
		}
		parent.Children = append(parent.Children, node)
	}
	return roots
}

// BuildTreeString 与 BuildTree 同,ID 为字符串类型(如 UUID 主键)。
func BuildTreeString[T any](list []T, idFn func(T) string, parentFn func(T) string, rootParentID string) []*Tree[T] {
	if len(list) == 0 {
		return nil
	}
	nodeMap := make(map[string]*Tree[T], len(list))
	for _, item := range list {
		nodeMap[idFn(item)] = &Tree[T]{Node: item}
	}
	var roots []*Tree[T]
	for _, item := range list {
		node := nodeMap[idFn(item)]
		parent := nodeMap[parentFn(item)]
		if parent == nil || parentFn(item) == rootParentID {
			roots = append(roots, node)
			continue
		}
		parent.Children = append(parent.Children, node)
	}
	return roots
}

// FlattenTree 树形结构拍平为列表(先序遍历,顺序稳定)。
func FlattenTree[T any](trees []*Tree[T]) []T {
	var result []T
	var walk func(nodes []*Tree[T])
	walk = func(nodes []*Tree[T]) {
		for _, node := range nodes {
			if node == nil {
				continue
			}
			result = append(result, node.Node)
			walk(node.Children)
		}
	}
	walk(trees)
	return result
}

// WalkTree 深度优先遍历树节点(先序)。
func WalkTree[T any](trees []*Tree[T], fn func(node T, depth int)) {
	var walk func(nodes []*Tree[T], depth int)
	walk = func(nodes []*Tree[T], depth int) {
		for _, node := range nodes {
			if node == nil {
				continue
			}
			fn(node.Node, depth)
			walk(node.Children, depth+1)
		}
	}
	walk(trees, 0)
}
