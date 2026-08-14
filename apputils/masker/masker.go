// Package masker 提供敏感字段脱敏工具。
// 用于日志、审计和前端回显前对手机号、邮箱、姓名、证件号等字段打码。
package masker

import "strings"

// Phone 手机号脱敏：保留前 3 后 4，如 138****1234。
func Phone(input string) string {
	if len(input) != 11 {
		return Default(input)
	}
	return input[:3] + "****" + input[len(input)-4:]
}

// Email 邮箱脱敏：本地部分保留首字符，如 a***@example.com。
func Email(input string) string {
	at := strings.Index(input, "@")
	if at <= 1 {
		return Default(input)
	}
	return input[:1] + "***" + input[at:]
}

// Name 姓名脱敏：保留姓氏，如 张*。
func Name(input string) string {
	runes := []rune(input)
	if len(runes) == 0 {
		return ""
	}
	if len(runes) == 1 {
		return "*"
	}
	return string(runes[0]) + strings.Repeat("*", len(runes)-1)
}

// IDCard 身份证脱敏：保留前 4 后 4，如 1101**********1234。
func IDCard(input string) string {
	return Default(input)
}

// BankCard 银行卡脱敏：保留前 4 后 4。
func BankCard(input string) string {
	return Default(input)
}

// Default 通用脱敏：长度不足 8 时全部打码，否则保留前 4 后 4。
func Default(input string) string {
	runes := []rune(input)
	if len(runes) == 0 {
		return ""
	}
	if len(runes) < 8 {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[:4]) + strings.Repeat("*", len(runes)-8) + string(runes[len(runes)-4:])
}
