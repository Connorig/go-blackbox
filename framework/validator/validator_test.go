package validator

import (
	"strings"
	"testing"

	apperr "github.com/Connorig/go-blackbox/component/error"
)

type createOrderReq struct {
	OrderNo string `validate:"required,min=6,max=32" label:"订单号"`
	Qty     int    `validate:"required,gt=0" label:"数量"`
	Email   string `validate:"omitempty,email" label:"邮箱"`
}

// TestStructValid 合法结构体通过。
func TestStructValid(t *testing.T) {
	req := createOrderReq{OrderNo: "SO-0001", Qty: 5, Email: "a@b.com"}
	if err := Struct(req); err != nil {
		t.Fatalf("valid struct must pass: %v", err)
	}
}

// TestStructInvalidLabel 非法结构体返回中文 label 错误 + A0400。
func TestStructInvalidLabel(t *testing.T) {
	req := createOrderReq{OrderNo: "abc", Qty: 0, Email: "not-an-email"}
	errs := StructAll(req)
	if len(errs) == 0 {
		t.Fatal("invalid struct must fail")
	}
	first := errs[0].(*apperr.Error)
	if first.Code != apperr.CodeRequestParamError {
		t.Fatalf("code = %v", first.Code)
	}
	joined := joinErrors(errs)
	if !strings.Contains(joined, "订单号") && !strings.Contains(joined, "数量") && !strings.Contains(joined, "邮箱") {
		t.Fatalf("errors missing label: %v", joined)
	}
}

// TestStructFirstOnly Struct 只返回第一条错误。
func TestStructFirstOnly(t *testing.T) {
	req := createOrderReq{}
	if err := Struct(req); err == nil {
		t.Fatal("must fail")
	}
}

// TestVar 单变量校验。
func TestVar(t *testing.T) {
	if err := Var("bad-email", "email"); err == nil {
		t.Fatal("bad email must fail")
	}
	if err := Var("good@example.com", "email"); err != nil {
		t.Fatalf("good email must pass: %v", err)
	}
}

// TestNilSafe nil 输入。
func TestNilSafe(t *testing.T) {
	if err := Struct(nil); err != nil {
		t.Fatalf("nil must pass: %v", err)
	}
}

// joinErrors 拼接错误信息。
func joinErrors(errs []error) string {
	parts := make([]string, 0, len(errs))
	for _, err := range errs {
		parts = append(parts, err.Error())
	}
	return strings.Join(parts, "; ")
}
