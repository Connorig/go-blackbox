// Package apperr 定义统一业务错误：HTTP 状态 + 业务码 + 消息 + 原始错误链。
// 业务码遵循阿里巴巴《Java开发手册(泰山版)》A/B/C 三级错误码体系（见 codes.go）。
// 与 webiris.OK/Fail 配合，业务代码返回 apperr.Error，由中间件统一转响应。
package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Code 是业务错误码（手册 A/B/C 分级，如 "A0400" 参数错误、"B0001" 系统错误）。
type Code string

// Error 是统一业务错误。
// Message 会直接暴露给调用方，不得包含敏感信息；原始错误通过 Cause 保留在服务端日志。
type Error struct {
	HTTPStatus int
	Code       Code
	Message    string
	Cause      error
}

// New 创建业务错误（无原始错误）；HTTP 状态按错误码默认映射。
func New(code Code, message string) *Error {
	return &Error{HTTPStatus: HTTPStatus(code), Code: code, Message: message}
}

// NewWithStatus 创建业务错误并显式指定 HTTP 状态。
func NewWithStatus(httpStatus int, code Code, message string) *Error {
	return &Error{HTTPStatus: httpStatus, Code: code, Message: message}
}

// Newf 创建带格式化消息的业务错误。
func Newf(code Code, format string, args ...interface{}) *Error {
	return &Error{HTTPStatus: HTTPStatus(code), Code: code, Message: fmt.Sprintf(format, args...)}
}

// Wrap 包装原始错误为业务错误；原始错误不出现在 Message 中。
func Wrap(err error, code Code, message string) *Error {
	return &Error{HTTPStatus: HTTPStatus(code), Code: code, Message: message, Cause: err}
}

// WrapWithStatus 包装原始错误并显式指定 HTTP 状态。
func WrapWithStatus(err error, httpStatus int, code Code, message string) *Error {
	return &Error{HTTPStatus: httpStatus, Code: code, Message: message, Cause: err}
}

// Error 实现 error 接口。
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s (code=%s): %v", e.Message, e.Code, e.Cause)
	}
	return fmt.Sprintf("%s (code=%s)", e.Message, e.Code)
}

// Unwrap 支持 errors.Is/As 定位原始错误。
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// From 把任意 error 转换为 *Error：
// 已是 *Error 时原样返回；其他错误转换为 B0001 系统错误（服务端应记录 Cause）。
func From(err error) *Error {
	if err == nil {
		return nil
	}
	var appError *Error
	if errors.As(err, &appError) {
		return appError
	}
	return &Error{HTTPStatus: http.StatusInternalServerError, Code: CodeSystemError, Message: "internal server error", Cause: err}
}

// Is 判断错误链中是否存在指定业务码的错误。
func Is(err error, code Code) bool {
	var appError *Error
	if errors.As(err, &appError) {
		return appError.Code == code
	}
	return false
}
