// Package validator 提供统一参数校验(对标 Spring Validation):
// 结构体 tag 校验 + 中文错误信息 + 业务错误码(A0400 参数错误)集成。
// 底层基于 go-playground/validator(Go 生态主流,iris 同款)。
//
// 用法:
//
//	type CreateOrderReq struct {
//	    OrderNo string `validate:"required,min=6,max=32" label:"订单号"`
//	    Qty     int    `validate:"required,gt=0" label:"数量"`
//	    Email   string `validate:"omitempty,email" label:"邮箱"`
//	}
//	if err := validator.Struct(&req); err != nil {
//	    // err 为 *apperr.Error(CodeA0400),错误信息含字段名与规则
//	}
package validator

import (
	"fmt"
	"reflect"
	"strings"
	"sync"

	playground "github.com/go-playground/validator/v10"
	apperr "github.com/Connorig/go-blackbox/component/error"
)

// instance 全局校验器(线程安全)。
var instance = playground.New()

// rules 中文规则文案(按需扩展)。
var rules = map[string]string{
	"required": "不能为空",
	"min":      "长度/数值过小",
	"max":      "长度/数值过大",
	"gt":       "必须大于",
	"gte":      "必须大于等于",
	"lt":       "必须小于",
	"lte":      "必须小于等于",
	"email":    "邮箱格式不正确",
	"url":      "URL 格式不正确",
	"len":      "长度不正确",
	"oneof":    "取值不合法",
}

// registerMu 保护自定义规则注册。
var registerMu sync.Mutex

// Struct 校验结构体:通过返回 nil;
// 失败返回 *apperr.Error(CodeA0400 参数错误),Message 为第一条错误(字段名+规则)。
// 校验多字段时只取首错(接口风格);业务需要全部错误用 StructAll。
func Struct(s interface{}) error {
	if errs := StructAll(s); len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// StructAll 校验结构体并返回全部错误(空列表 = 通过)。
// 错误信息格式:"订单号 长度/数值过小"(label 优先,其次字段名)。
func StructAll(s interface{}) []error {
	if s == nil {
		return nil
	}
	err := instance.Struct(s)
	if err == nil {
		return nil
	}
	validationErrors, ok := err.(playground.ValidationErrors)
	if !ok {
		return []error{apperr.New(apperr.CodeRequestParamError, err.Error())}
	}
	labelMap := buildLabelMap(s)
	errors := make([]error, 0, len(validationErrors))
	for _, fieldError := range validationErrors {
		errors = append(errors, apperr.New(apperr.CodeRequestParamError, formatError(fieldError, labelMap)))
	}
	return errors
}

// Var 校验单个变量(如 url 参数):tag 如 "required,email"。
func Var(value interface{}, tag string) error {
	if err := instance.Var(value, tag); err != nil {
		return apperr.New(apperr.CodeRequestParamError, err.Error())
	}
	return nil
}

// RegisterCustom 注册自定义校验规则(启动时调用一次)。
// 规则函数签名:func(fl playground.FieldLevel) bool。
func RegisterCustom(tag, message string, fn playground.Func) {
	registerMu.Lock()
	defer registerMu.Unlock()
	_ = instance.RegisterValidation(tag, fn)
	rules[tag] = message
}

// buildLabelMap 构建字段名 → label 映射(label tag 优先)。
func buildLabelMap(s interface{}) map[string]string {
	labelMap := make(map[string]string)
	typ := reflect.TypeOf(s)
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}
	if typ.Kind() != reflect.Struct {
		return labelMap
	}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if label := strings.TrimSpace(field.Tag.Get("label")); label != "" {
			labelMap[field.Name] = label
		}
	}
	return labelMap
}

// formatError 格式化单条校验错误:字段名(label 优先)+ 规则文案。
func formatError(fieldError playground.FieldError, labelMap map[string]string) string {
	field := fieldError.Field()
	if label, ok := labelMap[field]; ok && label != "" {
		field = label
	}
	message := rules[fieldError.Tag()]
	if message == "" {
		message = fieldError.Tag()
	}
	return fmt.Sprintf("%s %s", field, message)
}
