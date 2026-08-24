package apidoc

import (
	"encoding/json"
	"reflect"
	"runtime"
	"strings"

	"github.com/kataras/iris/v12"
)

// Option 文档描述选项(约定之外的自定义)。
type Option func(*Operation)

// Summary 自定义摘要。
func Summary(text string) Option {
	return func(operation *Operation) { operation.Summary = text }
}

// Description 详细描述。
func Description(text string) Option {
	return func(operation *Operation) { operation.Description = text }
}

// Tag 分组标签(可多次)。
func Tag(tag string) Option {
	return func(operation *Operation) { operation.Tags = append(operation.Tags, tag) }
}

// PathParam 路径参数(路由模板已自动解析,自定义描述用)。
func PathParam(name, paramType string, required bool, desc string) Option {
	return func(operation *Operation) {
		operation.Params = append(operation.Params, Param{Name: name, In: "path", Type: paramType, Required: required, Desc: desc})
	}
}

// QueryParam 查询参数。
func QueryParam(name, paramType string, required bool, desc string) Option {
	return func(operation *Operation) {
		operation.Params = append(operation.Params, Param{Name: name, In: "query", Type: paramType, Required: required, Desc: desc})
	}
}

// Body 请求体模型。
func Body(model interface{}, required bool, desc string) Option {
	return func(operation *Operation) {
		operation.RequestBody = &RequestBody{Model: model, Required: required, Desc: desc}
	}
}

// Responds 成功响应模型(默认 200 通用响应)。
func Responds(model interface{}) Option {
	return func(operation *Operation) {
		operation.Responses = append(operation.Responses, Response{Code: 200, Desc: "成功", Model: model})
	}
}

// Respond 指定状态码响应。
func Respond(code int, desc string, model interface{}) Option {
	return func(operation *Operation) {
		operation.Responses = append(operation.Responses, Response{Code: code, Desc: desc, Model: model})
	}
}

// Deprecated 标记弃用。
func Deprecated() Option {
	return func(operation *Operation) { operation.Deprecated = true }
}

// store 全局文档收集器(Register 时自动创建)。
var store = NewDocStore()

// Store 返回全局文档收集器。
func Store() *DocStore { return store }

// GET 注册 GET 路由并收集文档(自动推断:函数名→摘要、路径模板→参数)。
func GET(app *iris.Application, path string, handler iris.Handler, options ...Option) {
	register(app, "GET", path, handler, options...)
}

// POST 注册 POST 路由并收集文档。
func POST(app *iris.Application, path string, handler iris.Handler, options ...Option) {
	register(app, "POST", path, handler, options...)
}

// PUT 注册 PUT 路由并收集文档。
func PUT(app *iris.Application, path string, handler iris.Handler, options ...Option) {
	register(app, "PUT", path, handler, options...)
}

// DELETE 注册 DELETE 路由并收集文档。
func DELETE(app *iris.Application, path string, handler iris.Handler, options ...Option) {
	register(app, "DELETE", path, handler, options...)
}

// register 统一注册 + 收集。
func register(app *iris.Application, method, path string, handler iris.Handler, options ...Option) {
	operation := Operation{
		Method:      method,
		Path:        path,
		HandlerName: handlerName(handler),
	}
	// 自动推断
	operation.Summary = inferSummary(operation.HandlerName, path)
	operation.Tags = append(operation.Tags, inferTag(path))
	operation.Params = append(operation.Params, inferPathParams(path)...)
	// 默认响应(未指定时)
	operation.Responses = append(operation.Responses, Response{Code: 200, Desc: "成功"})

	// 自定义覆盖
	for _, option := range options {
		if option != nil {
			option(&operation)
		}
	}
	store.Add(operation)
	app.Handle(method, path, handler)
}

// handlerName 获取函数名(反射)。
func handlerName(handler iris.Handler) string {
	if handler == nil {
		return ""
	}
	full := runtime.FuncForPC(reflect.ValueOf(handler).Pointer()).Name()
	parts := strings.Split(full, ".")
	return parts[len(parts)-1]
}

// ---- OpenAPI 3.0 生成 ----

// OpenAPISpec OpenAPI 3.0 文档结构。
type OpenAPISpec struct {
	OpenAPI    string              `json:"openapi"`
	Info       Info                `json:"info"`
	Paths      map[string]PathItem `json:"paths"`
	Components Components          `json:"components"`
}

// Info 文档信息。
type Info struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Version     string `json:"version"`
}

// PathItem 路径项。
type PathItem map[string]OperationDoc

// OperationDoc 操作文档(OpenAPI 对齐)。
type OperationDoc struct {
	Summary     string                 `json:"summary,omitempty"`
	Description string                 `json:"description,omitempty"`
	Tags        []string               `json:"tags,omitempty"`
	Deprecated  bool                   `json:"deprecated,omitempty"`
	Parameters  []ParameterDoc         `json:"parameters,omitempty"`
	RequestBody *RequestBodyDoc        `json:"requestBody,omitempty"`
	Responses   map[string]ResponseDoc `json:"responses"`
	Security    []map[string][]string  `json:"security,omitempty"`
}

// ParameterDoc 参数文档。
type ParameterDoc struct {
	Name        string     `json:"name"`
	In          string     `json:"in"`
	Required    bool       `json:"required"`
	Description string     `json:"description,omitempty"`
	Schema      jsonSchema `json:"schema"`
}

// RequestBodyDoc 请求体文档。
type RequestBodyDoc struct {
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Content     map[string]struct {
		Schema jsonSchema `json:"schema"`
	} `json:"content"`
}

// ResponseDoc 响应文档。
type ResponseDoc struct {
	Description string `json:"description"`
	Content     map[string]struct {
		Schema jsonSchema `json:"schema"`
	} `json:"content,omitempty"`
}

// Components 组件(模型 Schema)。
type Components struct {
	Schemas map[string]jsonSchema `json:"schemas"`
}

// BuildOpenAPI 从 DocStore 生成 OpenAPI 3.0 定义。
func BuildOpenAPI(store *DocStore, title, version, description string) *OpenAPISpec {
	if store == nil {
		store = NewDocStore()
	}
	spec := &OpenAPISpec{
		OpenAPI:    "3.0.3",
		Info:       Info{Title: title, Description: description, Version: version},
		Paths:      map[string]PathItem{},
		Components: Components{Schemas: map[string]jsonSchema{}},
	}
	for _, operation := range store.Operations() {
		pathItem, exists := spec.Paths[operation.Path]
		if !exists {
			pathItem = PathItem{}
			spec.Paths[operation.Path] = pathItem
		}
		doc := OperationDoc{
			Summary:     operation.Summary,
			Description: operation.Description,
			Tags:        operation.Tags,
			Deprecated:  operation.Deprecated,
			Responses:   map[string]ResponseDoc{},
		}
		// 参数
		for _, param := range operation.Params {
			doc.Parameters = append(doc.Parameters, ParameterDoc{
				Name: param.Name, In: param.In, Required: param.Required,
				Description: param.Desc, Schema: jsonSchema{Type: schemaType(param.Type)},
			})
		}
		// 请求体
		if operation.RequestBody != nil && operation.RequestBody.Model != nil {
			doc.RequestBody = &RequestBodyDoc{
				Description: operation.RequestBody.Desc,
				Required:    operation.RequestBody.Required,
				Content: map[string]struct {
					Schema jsonSchema `json:"schema"`
				}{"application/json": {Schema: schemaOf(operation.RequestBody.Model, store, true)}},
			}
		}
		// 响应
		responseCodes := map[int]bool{}
		for _, response := range operation.Responses {
			key := itoa(response.Code)
			responseDoc := ResponseDoc{Description: response.Desc}
			if response.Model != nil {
				responseDoc.Content = map[string]struct {
					Schema jsonSchema `json:"schema"`
				}{"application/json": {Schema: responseWrapperSchema(response.Model, store)}}
			}
			doc.Responses[key] = responseDoc
			responseCodes[response.Code] = true
		}
		if !responseCodes[200] {
			// 自动推断失败兜底:通用响应
			if _, exists := doc.Responses["200"]; !exists {
				doc.Responses["200"] = ResponseDoc{Description: "成功"}
			}
		}
		// 安全(默认 Bearer,可后续配置)
		doc.Security = []map[string][]string{{"BearerAuth": {}}}
		pathItem[strings.ToLower(operation.Method)] = doc
		spec.Paths[operation.Path] = pathItem
	}
	// 模型 Schema
	for name, model := range store.Models() {
		spec.Components.Schemas[name] = schemaOf(model, store, false)
	}
	return spec
}

// responseWrapperSchema 统一响应包装 {code, message, data}。
func responseWrapperSchema(model interface{}, store *DocStore) jsonSchema {
	wrapper := struct {
		Code    string      `json:"code"`
		Message string      `json:"message"`
		Data    interface{} `json:"data,omitempty"`
	}{Code: "00000", Message: "ok", Data: model}
	return schemaOf(wrapper, store, false)
}

// schemaType 参数类型字符串 → schema 类型。
func schemaType(paramType string) string {
	switch strings.ToLower(paramType) {
	case "int", "int64", "int32", "integer":
		return "integer"
	case "float", "float64", "number":
		return "number"
	case "bool", "boolean":
		return "boolean"
	default:
		return "string"
	}
}

// itoa 数字转字符串。
func itoa(value int) string {
	if value == 0 {
		return "200"
	}
	digits := "0123456789"
	if value == 0 {
		return "0"
	}
	var result []byte
	for value > 0 {
		result = append([]byte{digits[value%10]}, result...)
		value /= 10
	}
	return string(result)
}

// ToJSON 输出 OpenAPI JSON。
func (s *OpenAPISpec) ToJSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}
