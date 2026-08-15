package webiris

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Connorig/go-blackbox/component/auth/token"
	"github.com/Connorig/go-blackbox/framework/database"
	"github.com/kataras/iris/v12"
)

// setupAuthEnv 注入 JWT 密钥(每个测试独立注入,覆盖即可)。
func setupAuthEnv(t *testing.T) {
	t.Helper()
	if err := apptoken.SetSecretKey("0123456789abcdef0123456789abcdef"); err != nil {
		t.Fatalf("set secret failed: %v", err)
	}
}

// TestDataScopeFromAuthToken 认证后组织身份可读。
func TestDataScopeFromAuthToken(t *testing.T) {
	setupAuthEnv(t)
	app := iris.New()
	app.Get("/api/v1/data", Auth(), func(ctx iris.Context) {
		scope := DataScope(ctx)
		_ = ctx.JSON(map[string]interface{}{
			"org_id":  scope.OrgID,
			"dept_id": scope.DeptID,
			"empty":   scope.IsEmpty(),
		})
	})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}

	access, _, err := apptoken.GenTokenFull(3, "dev@example.com", "data:read", 88, 99)
	if err != nil {
		t.Fatalf("gen token failed: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/data", nil)
	request.Header.Set("Authorization", "Bearer "+access)
	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	if !strings.Contains(body, `"org_id":88`) || !strings.Contains(body, `"dept_id":99`) {
		t.Fatalf("org identity not exposed: %s", body)
	}
	if strings.Contains(body, `"empty":true`) {
		t.Fatalf("scope must not be empty: %s", body)
	}
}

// TestDataScopeLegacyToken 老 token(无组织字段)解析为空范围。
func TestDataScopeLegacyToken(t *testing.T) {
	setupAuthEnv(t)
	app := iris.New()
	app.Get("/api/v1/legacy", Auth(), func(ctx iris.Context) {
		scope := DataScope(ctx)
		_ = ctx.JSON(map[string]interface{}{"org_id": scope.OrgID, "dept_id": scope.DeptID, "empty": scope.IsEmpty()})
	})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}

	access, _, err := apptoken.GenTokenWithScope(3, "dev@example.com", "")
	if err != nil {
		t.Fatalf("gen token failed: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/legacy", nil)
	request.Header.Set("Authorization", "Bearer "+access)
	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), `"empty":true`) {
		t.Fatalf("legacy token must produce empty scope: %s", recorder.Body.String())
	}
}

// TestDataScopeUnifiedWithDatabase 与 database.DataScope 类型一致(编译期验证)。
func TestDataScopeUnifiedWithDatabase(t *testing.T) {
	setupAuthEnv(t)
	app := iris.New()
	app.Get("/api/v1/type-check", Auth(), func(ctx iris.Context) {
		// DataScope 返回类型必须可直接赋给 database.DataScope
		var scope datasource.DataScope
		scope = DataScope(ctx)
		_ = ctx.JSON(map[string]interface{}{"org_id": scope.OrgID})
	})
	if err := app.Build(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	access, _, err := apptoken.GenTokenWithScope(3, "dev@example.com", "")
	if err != nil {
		t.Fatalf("gen token failed: %v", err)
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/type-check", nil)
	request.Header.Set("Authorization", "Bearer "+access)
	recorder := httptest.NewRecorder()
	app.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}
