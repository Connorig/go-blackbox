package apperr

import (
	"errors"
	"net/http"
	"testing"
)

// TestNewAndMessage 验证构造器与错误消息格式。
func TestNewAndMessage(t *testing.T) {
	appError := New(http.StatusBadRequest, CodeBadRequest, "invalid input")
	if appError.HTTPStatus != 400 || appError.Code != 400 {
		t.Fatalf("unexpected error fields: %+v", appError)
	}
	if appError.Error() != "invalid input (code=400)" {
		t.Fatalf("unexpected error message: %s", appError.Error())
	}
}

// TestWrapPreservesCause 验证 Wrap 保留原始错误链。
func TestWrapPreservesCause(t *testing.T) {
	cause := errors.New("database down")
	appError := Wrap(cause, http.StatusInternalServerError, CodeInternal, "query failed")
	if !errors.Is(appError, cause) {
		t.Fatal("errors.Is must reach the wrapped cause")
	}
	if appError.Cause != cause {
		t.Fatal("cause must be preserved")
	}
}

// TestFromExtractsAppError 验证 From 对已有业务错误原样返回。
func TestFromExtractsAppError(t *testing.T) {
	expected := New(http.StatusNotFound, CodeNotFound, "not found")
	got := From(expected)
	if got != expected {
		t.Fatal("From must return the original app error")
	}
}

// TestFromConvertsUnknownError 验证未知错误转换为 500 且保留原因。
func TestFromConvertsUnknownError(t *testing.T) {
	cause := errors.New("panic-ish failure")
	got := From(cause)
	if got.HTTPStatus != http.StatusInternalServerError || got.Code != CodeInternal {
		t.Fatalf("unexpected converted error: %+v", got)
	}
	if !errors.Is(got, cause) {
		t.Fatal("converted error must keep the cause in its chain")
	}
}

// TestIsChecksBusinessCode 验证 Is 按业务码判断错误链。
func TestIsChecksBusinessCode(t *testing.T) {
	appError := Wrap(errors.New("raw"), http.StatusForbidden, CodeForbidden, "no permission")
	if !Is(appError, CodeForbidden) {
		t.Fatal("Is must match the business code")
	}
	if Is(appError, CodeNotFound) {
		t.Fatal("Is must not match a different code")
	}
}
