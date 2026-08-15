package apidoc

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kataras/iris/v12"
)

// testOrder 测试模型。
type testOrder struct {
	ID        int64             `json:"id"`
	OrderNo   string            `json:"order_no"`
	Amount    float64           `json:"amount"`
	Status    int               `json:"status"`
	Items     []testOrderItem   `json:"items"`
	CreatedAt time.Time         `json:"created_at"`
	Note      *string           `json:"note,omitempty"`
	Extra     map[string]string `json:"extra,omitempty"`
}

type testOrderItem struct {
	SkuID  int64  `json:"sku_id"`
	Count  int    `json:"count"`
}

func noop(ctx iris.Context) {}

// resetStore 测试隔离。
func resetStore() {
	store = NewDocStore()
}

// TestInferSummary 函数名推断。
func TestInferSummary(t *testing.T) {
	cases := map[string]string{
		"GetOrder":    "获取orders",
		"CreateOrder": "创建orders",
		"UpdateOrder": "更新orders",
		"DeleteOrder": "删除orders",
		"ListOrders":  "查询列表orders",
	}
	for handler, want := range cases {
		if got := inferSummary(handler, "/api/v1/orders"); got != want {
			t.Errorf("%s → %q, want %q", handler, got, want)
		}
	}
	// 独立动词(不拼接实体)
	if got := inferSummary("Login", "/api/v1/auth"); got != "登录" {
		t.Errorf("Login → %q, want %q", got, "登录")
	}
}

// TestInferPathParams 路由模板解析。
func TestInferPathParams(t *testing.T) {
	params := inferPathParams("/api/v1/orders/{id:int64}")
	if len(params) != 1 || params[0].Name != "id" || params[0].Type != "int64" || !params[0].Required {
		t.Fatalf("params = %+v", params)
	}
	if got := inferPathParams("/api/v1/orders"); len(got) != 0 {
		t.Fatalf("no params expected: %+v", got)
	}
}

// TestInferTag 分组推断。
func TestInferTag(t *testing.T) {
	if got := inferTag("/api/v1/orders/{id}"); got != "orders" {
		t.Fatalf("tag = %q", got)
	}
}

// TestSchemaNested 反射 Schema:嵌套/数组/时间/必填。
func TestSchemaNested(t *testing.T) {
	store := NewDocStore()
	schema := schemaOf(&testOrder{}, store, false)
	if schema.Type != "object" {
		t.Fatalf("type = %q", schema.Type)
	}
	if schema.Properties["order_no"].Type != "string" {
		t.Fatalf("order_no type wrong: %+v", schema.Properties["order_no"])
	}
	if schema.Properties["amount"].Type != "number" {
		t.Fatalf("amount type wrong: %+v", schema.Properties["amount"])
	}
	if schema.Properties["created_at"].Format != "date-time" {
		t.Fatalf("created_at format wrong: %+v", schema.Properties["created_at"])
	}
	items := schema.Properties["items"]
	if items.Type != "array" || items.Items == nil {
		t.Fatalf("items schema wrong: %+v", items)
	}
	// 必填:id/order_no/amount/status/items/created_at 无 omitempty
	if !contains(schema.Required, "id") || !contains(schema.Required, "order_no") {
		t.Fatalf("required wrong: %v", schema.Required)
	}
	// note/extra 有 omitempty → 非必填
	if contains(schema.Required, "note") {
		t.Fatalf("note must not be required: %v", schema.Required)
	}
}

// TestBuildOpenAPI 文档收集 + OpenAPI 生成。
func TestBuildOpenAPI(t *testing.T) {
	resetStore()
	app := iris.New()
	GET(app, "/api/v1/orders/{id:int64}", noop, Summary("自定义摘要"))
	POST(app, "/api/v1/orders", noop, Body(&testOrder{}, true, "创建"))
	CRUD(app, "/api/v1/products", &testOrder{}, noop, noop, noop, noop, noop)

	spec := BuildOpenAPI(store, "测试服务", "1.0.0", "desc")
	data, err := spec.ToJSON()
	if err != nil {
		t.Fatalf("to json failed: %v", err)
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if raw["openapi"] != "3.0.3" {
		t.Fatalf("openapi version: %v", raw["openapi"])
	}
	paths, _ := raw["paths"].(map[string]interface{})
	if _, exists := paths["/api/v1/orders/{id:int64}"]; !exists {
		t.Fatalf("orders path missing: %v", paths)
	}
	if _, exists := paths["/api/v1/products"]; !exists {
		t.Fatalf("products path missing")
	}
	if _, exists := paths["/api/v1/products/{id:int64}"]; !exists {
		t.Fatalf("products detail path missing")
	}
	// 模型 schema 已注册
	components, _ := raw["components"].(map[string]interface{})
	schemas, _ := components["schemas"].(map[string]interface{})
	if _, exists := schemas["testOrder"]; !exists {
		t.Fatalf("testOrder schema missing: %v", schemas)
	}
}

// GetOrderHandler 命名 handler(函数名推断用)。
func GetOrderHandler(ctx iris.Context) {}

// TestDefaultSummary 自动推断摘要进入 OpenAPI。
func TestDefaultSummary(t *testing.T) {
	resetStore()
	app := iris.New()
	GET(app, "/api/v1/orders/{id:int64}", GetOrderHandler)
	spec := BuildOpenAPI(store, "t", "1", "")
	pathItem := spec.Paths["/api/v1/orders/{id:int64}"]
	op := pathItem["get"]
	if !strings.Contains(op.Summary, "获取") {
		t.Fatalf("summary = %q", op.Summary)
	}
	if len(op.Parameters) != 1 || op.Parameters[0].Name != "id" {
		t.Fatalf("params = %+v", op.Parameters)
	}
	if op.Responses["200"].Description == "" {
		t.Fatal("default response missing")
	}
}

// TestCRUDDocumentation CRUD 工厂生成 5 个接口文档。
func TestCRUDDocumentation(t *testing.T) {
	resetStore()
	app := iris.New()
	CRUD(app, "/api/v1/orders", &testOrder{}, noop, noop, noop, noop, noop)
	operations := store.Operations()
	if len(operations) != 5 {
		t.Fatalf("expected 5 operations, got %d", len(operations))
	}
	methods := map[string]bool{}
	for _, operation := range operations {
		methods[operation.Method] = true
	}
	for _, method := range []string{"GET", "POST", "PUT", "DELETE"} {
		if !methods[method] {
			t.Fatalf("method %s missing: %v", method, methods)
		}
	}
}

func contains(list []string, value string) bool {
	for _, item := range list {
		if item == value {
			return true
		}
	}
	return false
}
