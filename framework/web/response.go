package webiris

import "github.com/kataras/iris/v12"

// Response 是统一业务响应结构。
// Code 为业务码（0 表示成功），Message 为可读信息，Data 为负载。
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// OK 返回成功响应。
func OK(ctx iris.Context, data interface{}) {
	if err := ctx.JSON(Response{Code: 0, Message: "ok", Data: data}); err != nil {
		ctx.Application().Logger().Errorf("write success response failed: %v", err)
	}
}

// Fail 返回失败响应并设置 HTTP 状态码。
func Fail(ctx iris.Context, httpStatus, code int, message string) {
	ctx.StatusCode(httpStatus)
	if err := ctx.JSON(Response{Code: code, Message: message}); err != nil {
		ctx.Application().Logger().Errorf("write failure response failed: %v", err)
	}
}
