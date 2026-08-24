package apidoc

import (
	"reflect"
	"strings"
	"time"
)

// 反射生成 JSON Schema(OpenAPI 3.0 schema 对象子集)。

// jsonSchema 简化 JSON Schema。
type jsonSchema struct {
	Type       string                `json:"type,omitempty"`
	Format     string                `json:"format,omitempty"`
	Required   []string              `json:"required,omitempty"`
	Properties map[string]jsonSchema `json:"properties,omitempty"`
	Items      *jsonSchema           `json:"items,omitempty"`
	Ref        string                `json:"$ref,omitempty"`
	Example    interface{}           `json:"example,omitempty"`
}

// modelTypeName 模型类型名(去包前缀)。
func modelTypeName(model interface{}) string {
	if model == nil {
		return ""
	}
	t := reflect.TypeOf(model)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

// schemaOf 生成模型 Schema(注册引用 + 内联展开)。
func schemaOf(model interface{}, store *DocStore, ref bool) jsonSchema {
	if model == nil {
		return jsonSchema{Type: "object"}
	}
	value := reflect.ValueOf(model)
	t := value.Type()
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return jsonSchema{Type: goTypeName(t.Kind())}
	}
	name := t.Name()
	if ref && store != nil {
		store.RegisterModel(model)
		return jsonSchema{Ref: "#/components/schemas/" + name}
	}
	return structSchema(t, store)
}

// structSchema 结构体展开。
func structSchema(t reflect.Type, store *DocStore) jsonSchema {
	schema := jsonSchema{Type: "object", Properties: map[string]jsonSchema{}}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if field.PkgPath != "" {
			continue // 未导出
		}
		name := jsonName(field)
		if name == "-" || name == "" {
			continue
		}
		fieldSchema := fieldSchema(field.Type, store)
		// 必填推断:无 omitempty + 非指针
		if required(field) {
			schema.Required = append(schema.Required, name)
		}
		schema.Properties[name] = fieldSchema
	}
	return schema
}

// fieldSchema 单字段 Schema。
func fieldSchema(t reflect.Type, store *DocStore) jsonSchema {
	// 时间类型特判
	if t == reflect.TypeOf(time.Time{}) {
		return jsonSchema{Type: "string", Format: "date-time"}
	}
	switch t.Kind() {
	case reflect.Ptr:
		return fieldSchema(t.Elem(), store)
	case reflect.Struct:
		// 嵌套结构:引用
		if t.Name() != "" {
			name := t.Name()
			if store != nil {
				// 注册引用(避免无限递归:仅对非 time 结构体)
				if t != reflect.TypeOf(time.Time{}) {
					store.RegisterModel(reflect.New(t).Interface())
					return jsonSchema{Ref: "#/components/schemas/" + name}
				}
			}
		}
		return structSchema(t, store)
	case reflect.Slice, reflect.Array:
		return jsonSchema{Type: "array", Items: ptr(fieldSchema(t.Elem(), store))}
	case reflect.Map:
		return jsonSchema{Type: "object"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return jsonSchema{Type: "integer", Format: "int64"}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return jsonSchema{Type: "integer"}
	case reflect.Float32, reflect.Float64:
		return jsonSchema{Type: "number"}
	case reflect.Bool:
		return jsonSchema{Type: "boolean"}
	case reflect.String:
		return jsonSchema{Type: "string"}
	case reflect.Interface:
		return jsonSchema{Type: "object"}
	default:
		return jsonSchema{Type: "string"}
	}
}

// jsonName 提取 JSON 字段名(优先 json tag)。
func jsonName(field reflect.StructField) string {
	if tag := field.Tag.Get("json"); tag != "" {
		return strings.Split(tag, ",")[0]
	}
	if tag := field.Tag.Get("mapstructure"); tag != "" {
		return strings.Split(tag, ",")[0]
	}
	return field.Name
}

// required 必填推断:无 omitempty 且非指针。
func required(field reflect.StructField) bool {
	tag := field.Tag.Get("json")
	if tag == "" {
		return false
	}
	if strings.Contains(tag, "omitempty") {
		return false
	}
	if field.Type.Kind() == reflect.Ptr {
		return false
	}
	return true
}

// goTypeName 基本类型名。
func goTypeName(kind reflect.Kind) string {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Bool:
		return "boolean"
	default:
		return "string"
	}
}

// ptr 指针包装。
func ptr(schema jsonSchema) *jsonSchema {
	return &schema
}
