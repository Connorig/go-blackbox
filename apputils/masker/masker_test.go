package masker

import (
	"strings"
	"testing"
)

// TestPhone 验证手机号脱敏。
func TestPhone(t *testing.T) {
	if got := Phone("13812341234"); got != "138****1234" {
		t.Fatalf("unexpected phone mask: %q", got)
	}
	// 非法长度走通用脱敏
	if got := Phone("123"); got != "***" {
		t.Fatalf("unexpected short phone mask: %q", got)
	}
}

// TestEmail 验证邮箱脱敏。
func TestEmail(t *testing.T) {
	if got := Email("zhangsan@example.com"); got != "z***@example.com" {
		t.Fatalf("unexpected email mask: %q", got)
	}
}

// TestName 验证姓名脱敏。
func TestName(t *testing.T) {
	if got := Name("张三丰"); got != "张**" {
		t.Fatalf("unexpected name mask: %q", got)
	}
	if got := Name("张"); got != "*" {
		t.Fatalf("unexpected single-char name mask: %q", got)
	}
}

// TestIDCardAndBankCard 验证证件号与银行卡脱敏。
func TestIDCardAndBankCard(t *testing.T) {
	idCardExpected := "1101" + strings.Repeat("*", 10) + "1234"
	if got := IDCard("110101199001011234"); got != idCardExpected {
		t.Fatalf("unexpected id card mask: %q", got)
	}
	bankCardExpected := "6222" + strings.Repeat("*", 8) + "7890"
	if got := BankCard("6222021234567890"); got != bankCardExpected {
		t.Fatalf("unexpected bank card mask: %q", got)
	}
}

// TestDefault 验证通用脱敏边界。
func TestDefault(t *testing.T) {
	if got := Default(""); got != "" {
		t.Fatalf("empty input must stay empty: %q", got)
	}
	if got := Default("abcd"); got != "****" {
		t.Fatalf("short input must be fully masked: %q", got)
	}
	if got := Default("abcdefghij"); got != "abcd**ghij" {
		t.Fatalf("unexpected default mask: %q", got)
	}
}
