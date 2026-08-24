// Package apidoc 提供接口文档自动生成(对标 Springdoc/Swagger):
// 注册路由时自动推断文档(函数名/路径模板/动词),支持可选描述覆盖;
// 启动后输出 OpenAPI 3.0 定义 + 内嵌 API 浏览页面。
//
// 用法(约定优先,只写差异):
//
//	// ① 一行注册,全自动推断
//	apidoc.GET(app, "/api/v1/orders/{id}", GetOrder)
//
//	// ② 只写自定义部分
//	apidoc.POST(app, "/api/v1/orders", CreateOrder,
//	    apidoc.Summary("创建订单(自定义)"),
//	    apidoc.Responds(&Order{}),
//	)
//
//	// ③ CRUD 工厂:一次注册 5 个标准接口 + 完整文档
//	apidoc.CRUD(app, "/api/v1/orders", &Order{})
//
//	// ④ 页面与定义
//	//   GET /docs           API 浏览页面(内嵌)
//	//   GET /docs/api.json  OpenAPI 3.0 定义
package apidoc

import (
	"strings"
)

// Operation 接口操作元数据(OpenAPI operation 对齐)。
type Operation struct {
	Method      string       // HTTP 方法(GET/POST/PUT/DELETE)
	Path        string       // 路由路径(如 /api/v1/orders/{id})
	Summary     string       // 摘要
	Description string       // 详细描述
	Tags        []string     // 标签
	Params      []Param      // 参数
	Responses   []Response   // 响应
	Deprecated  bool         // 弃用标记
	RequestBody *RequestBody // 请求体(有 body 参数时)
	HandlerName string       // 处理函数名(调试)
}

// Param 参数定义。
type Param struct {
	Name     string
	In       string // path / query / header
	Type     string // string/int/int64/float64/bool
	Required bool
	Desc     string
}

// Response 响应定义。
type Response struct {
	Code  int
	Desc  string
	Model interface{} // 响应 data 模型(可选;nil=通用响应)
}

// RequestBody 请求体定义。
type RequestBody struct {
	Model    interface{}
	Required bool
	Desc     string
}

// DocStore 文档收集器。
type DocStore struct {
	operations []Operation
	models     map[string]interface{} // 已注册模型(供 schema 复用)
}

// NewDocStore 创建文档收集器。
func NewDocStore() *DocStore {
	return &DocStore{models: make(map[string]interface{})}
}

// Add 注册一条操作。
func (s *DocStore) Add(operation Operation) {
	if s == nil {
		return
	}
	s.operations = append(s.operations, operation)
}

// Operations 返回全部操作。
func (s *DocStore) Operations() []Operation {
	if s == nil {
		return nil
	}
	return s.operations
}

// RegisterModel 注册模型(可选,schema 引用用)。
func (s *DocStore) RegisterModel(model interface{}) {
	if s == nil || model == nil {
		return
	}
	name := modelTypeName(model)
	if _, exists := s.models[name]; !exists {
		s.models[name] = model
	}
}

// Models 返回已注册模型。
func (s *DocStore) Models() map[string]interface{} {
	if s == nil {
		return nil
	}
	return s.models
}

// ---- 推断规则 ----

// inferSummary 从函数名推断摘要(GetOrder → 获取订单)。
func inferSummary(handlerName string, path string) string {
	verb := ""
	// 无实体动词(不拼接实体名)
	standalone := map[string]bool{"Login": true, "Register": true, "Logout": true,
		"Upload": true, "Download": true, "Export": true, "Import": true}
	for _, pair := range [][2]string{
		{"Create", "创建"}, {"Add", "新增"}, {"Update", "更新"}, {"Edit", "编辑"},
		{"Delete", "删除"}, {"Remove", "删除"}, {"List", "查询列表"}, {"Get", "获取"},
		{"Query", "查询"}, {"Find", "查询"}, {"Save", "保存"}, {"Export", "导出"},
		{"Import", "导入"}, {"Upload", "上传"}, {"Download", "下载"}, {"Login", "登录"},
		{"Register", "注册"}, {"Logout", "退出"},
	} {
		if strings.HasPrefix(handlerName, pair[0]) {
			verb = pair[1]
			break
		}
	}
	if verb == "" {
		return handlerName
	}
	if standalone[handlerName] {
		return verb
	}
	entity := inferEntity(path)
	if entity == "" {
		return verb
	}
	return verb + entity
}

// inferEntity 从路径推断实体名(orders → 订单?无字典时取路径段原文)。
// 约定:路径最后一段(去 {id})为实体,如 /api/v1/orders → orders。
func inferEntity(path string) string {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	for i := len(segments) - 1; i >= 0; i-- {
		segment := segments[i]
		if segment == "" || strings.Contains(segment, "{") {
			continue
		}
		return segment
	}
	return ""
}

// inferTag 从路径推断分组(第一段业务段,如 orders)。
func inferTag(path string) string {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	for _, segment := range segments {
		if segment == "" || segment == "api" || segment == "v1" || strings.Contains(segment, "{") {
			continue
		}
		return segment
	}
	return "default"
}

// inferPathParams 从路由模板解析路径参数(/orders/{id:int64} → id int64)。
func inferPathParams(path string) []Param {
	var params []Param
	for _, segment := range strings.Split(path, "/") {
		if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") {
			continue
		}
		inner := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
		parts := strings.SplitN(inner, ":", 2)
		name := parts[0]
		paramType := "string"
		if len(parts) > 1 {
			paramType = parts[1]
		}
		params = append(params, Param{Name: name, In: "path", Type: paramType, Required: true})
	}
	return params
}

// ResetStoreForTest 重置全局文档收集器(测试隔离用)。
func ResetStoreForTest() {
	store = NewDocStore()
}
