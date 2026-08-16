package util

// FillTree 将扁平集合构建为树形结构,并把子节点直接填充进业务实体
// (业务 model 自带 Children 字段,前端直接拿到 model 数组)。
// 对标若依 SysMenu 的 children 模式;与 BuildTree(包装式)互补。
//
// 用法:
//
//	type Menu struct {
//	    ID       int64  `json:"id"`
//	    ParentID int64  `json:"parent_id"`
//	    Name     string `json:"name"`
//	    Children []Menu `json:"children"`
//	}
//	roots := util.FillTree(menus,
//	    func(m *Menu) int64 { return m.ID },
//	    func(m *Menu) int64 { return m.ParentID },
//	    func(parent *Menu, children []Menu) { parent.Children = children },
//	    0)
//	// roots 为 []Menu,每个 Menu.Children 已递归填充,直接 JSON 返回前端
//
// 特性:
//   - 支持任意层级(N 级)
//   - 孤儿节点挂根不丢数据;先序遍历顺序稳定
//   - idFn/parentFn/setChildren 均为函数式,任意实体结构适配
func FillTree[T any](list []T,
	idFn func(*T) int64,
	parentFn func(*T) int64,
	setChildren func(parent *T, children []T),
	rootParentID int64) []T {
	if len(list) == 0 {
		return nil
	}
	// ID → 列表下标
	index := make(map[int64]int, len(list))
	for i := range list {
		index[idFn(&list[i])] = i
	}
	// 父 ID → 子节点 ID 列表(保序)
	childrenIDs := make(map[int64][]int64)
	var rootIndexes []int
	for i := range list {
		parentID := parentFn(&list[i])
		if parentID == rootParentID {
			rootIndexes = append(rootIndexes, i)
			continue
		}
		if _, exists := index[parentID]; exists {
			childrenIDs[parentID] = append(childrenIDs[parentID], idFn(&list[i]))
		} else {
			// 孤儿节点:挂根,不丢数据
			rootIndexes = append(rootIndexes, i)
		}
	}
	// 递归填充:始终操作 list 原始元素(先填子,再挂到父)
	var fill func(id int64)
	fill = func(id int64) {
		childIDs := childrenIDs[id]
		if len(childIDs) == 0 {
			return
		}
		children := make([]T, 0, len(childIDs))
		for _, childID := range childIDs {
			childIdx := index[childID]
			fill(childID) // 先递归填充孙辈
			children = append(children, list[childIdx])
		}
		setChildren(&list[index[id]], children)
	}
	for _, idx := range rootIndexes {
		fill(idFn(&list[idx]))
	}
	roots := make([]T, 0, len(rootIndexes))
	for _, idx := range rootIndexes {
		roots = append(roots, list[idx])
	}
	return roots
}

// FillTreeString 字符串 ID 版本(UUID 主键)。
func FillTreeString[T any](list []T,
	idFn func(*T) string,
	parentFn func(*T) string,
	setChildren func(parent *T, children []T),
	rootParentID string) []T {
	if len(list) == 0 {
		return nil
	}
	index := make(map[string]int, len(list))
	for i := range list {
		index[idFn(&list[i])] = i
	}
	childrenIDs := make(map[string][]string)
	var rootIndexes []int
	for i := range list {
		parentID := parentFn(&list[i])
		if parentID == rootParentID {
			rootIndexes = append(rootIndexes, i)
			continue
		}
		if _, exists := index[parentID]; exists {
			childrenIDs[parentID] = append(childrenIDs[parentID], idFn(&list[i]))
		} else {
			rootIndexes = append(rootIndexes, i)
		}
	}
	var fill func(id string)
	fill = func(id string) {
		childIDs := childrenIDs[id]
		if len(childIDs) == 0 {
			return
		}
		children := make([]T, 0, len(childIDs))
		for _, childID := range childIDs {
			childIdx := index[childID]
			fill(childID)
			children = append(children, list[childIdx])
		}
		setChildren(&list[index[id]], children)
	}
	for _, idx := range rootIndexes {
		fill(idFn(&list[idx]))
	}
	roots := make([]T, 0, len(rootIndexes))
	for _, idx := range rootIndexes {
		roots = append(roots, list[idx])
	}
	return roots
}
