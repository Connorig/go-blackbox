package apperr

import (
	"errors"
	"net/http"
	"testing"
)

// TestNewAndMessage 验证构造器与错误消息格式。
func TestNewAndMessage(t *testing.T) {
	appError := New(CodeRequestParamError, "invalid input")
	if appError.HTTPStatus != 400 || appError.Code != CodeRequestParamError {
		t.Fatalf("unexpected error fields: %+v", appError)
	}
	if appError.Error() != "invalid input (code=A0400)" {
		t.Fatalf("unexpected error message: %s", appError.Error())
	}
}

// TestNewWithStatus 验证显式 HTTP 状态。
func TestNewWithStatus(t *testing.T) {
	appError := NewWithStatus(http.StatusTeapot, CodeSystemError, "teapot")
	if appError.HTTPStatus != http.StatusTeapot {
		t.Fatalf("unexpected status: %d", appError.HTTPStatus)
	}
}

// TestWrapPreservesCause 验证 Wrap 保留原始错误链。
func TestWrapPreservesCause(t *testing.T) {
	cause := errors.New("database down")
	appError := Wrap(cause, CodeDatabaseError, "query failed")
	if !errors.Is(appError, cause) {
		t.Fatal("errors.Is must reach the wrapped cause")
	}
	if appError.Cause != cause {
		t.Fatal("cause must be preserved")
	}
	if appError.HTTPStatus != http.StatusInternalServerError {
		t.Fatalf("unexpected mapped status: %d", appError.HTTPStatus)
	}
}

// TestFromExtractsAppError 验证 From 对已有业务错误原样返回。
func TestFromExtractsAppError(t *testing.T) {
	expected := New(CodeTableNotExists, "not found")
	got := From(expected)
	if got != expected {
		t.Fatal("From must return the original app error")
	}
}

// TestFromConvertsUnknownError 验证未知错误转换为 B0001 且保留原因。
func TestFromConvertsUnknownError(t *testing.T) {
	cause := errors.New("panic-ish failure")
	got := From(cause)
	if got.Code != CodeSystemError || got.HTTPStatus != http.StatusInternalServerError {
		t.Fatalf("unexpected converted error: %+v", got)
	}
	if !errors.Is(got, cause) {
		t.Fatal("converted error must keep the cause in its chain")
	}
}

// TestIsChecksBusinessCode 验证 Is 按业务码判断错误链。
func TestIsChecksBusinessCode(t *testing.T) {
	appError := Wrap(errors.New("raw"), CodeAPINoPermission, "no permission")
	if !Is(appError, CodeAPINoPermission) {
		t.Fatal("Is must match the business code")
	}
	if Is(appError, CodeAccessUnauthorized) {
		t.Fatal("Is must not match a different code")
	}
}

// TestHTTPStatusMapping 验证错误码 HTTP 状态映射。
func TestHTTPStatusMapping(t *testing.T) {
	cases := map[Code]int{
		CodeOK:                 200,
		CodeRequestParamError:  400,
		CodeAccessUnauthorized: 401,
		CodeAPINoPermission:    403,
		CodeRequestRateLimited: 429,
		CodeSystemError:        500,
		CodeSystemTimeout:      504,
		CodeDatabaseError:      500,
	}
	for code, expected := range cases {
		if got := HTTPStatus(code); got != expected {
			t.Fatalf("unexpected status for %s: got %d, want %d", code, got, expected)
		}
	}
	// 未注册码默认 500
	if got := HTTPStatus(Code("ZZ9999")); got != 500 {
		t.Fatalf("unknown code must map to 500, got %d", got)
	}
}
