// Package security 提供接口前置安全防护:
//   - SQL 注入特征检测(Get/Post 参数、请求体通用检测)
//   - DoS 防护配套:请求体大小限制、请求超时(配合 webiris.Limit 限流)
//   - 统一校验入口,供中间件或业务代码直接调用
package security

import (
	"regexp"
	"strings"
)

// SQL 注入特征模式(覆盖联合查询、布尔盲注、注释符、堆叠语句、危险关键字)。
// 命中任一模式即判定为可疑输入,建议在接口前置拦截并记录审计日志。
var sqlPatterns = []*regexp.Regexp{
	// 联合查询:union ... select
	regexp.MustCompile(`(?i)\bunion\b[\s\S]{0,80}?\bselect\b`),
	// 布尔盲注:' OR '1'='1 / " AND "1"="1(成对引号夹关键字)
	regexp.MustCompile(`(?i)('|")\s*\b(or|and)\b\s*('|")`),
	// 数值恒真:or 1=1 / and 1=1(带空格或注释变体)
	regexp.MustCompile(`(?i)\b(or|and)\s+\d+\s*=\s*\d+`),
	// 内联注释(MySQL 版本注释 / 注释符注入)
	regexp.MustCompile(`(?i)/\*.*?\*/`),
	// 行注释注入:--(需空格或行尾)与 #(需空格)
	regexp.MustCompile(`(--\s|--$|#\s)`),
	// 堆叠语句:; drop / ; select / ; delete ...
	regexp.MustCompile(`(?i);\s*\b(drop|select|delete|update|insert|alter|truncate|exec|execute)\b`),
	// 危险关键字 + 表/库操作组合
	regexp.MustCompile(`(?i)\b(drop|truncate)\s+table\b`),
	regexp.MustCompile(`(?i)\b(drop|create|alter)\s+database\b`),
	// information_schema 探测
	regexp.MustCompile(`(?i)\binformation_schema\b`),
	// 延时盲注:sleep( / benchmark(
	regexp.MustCompile(`(?i)\b(sleep|benchmark)\s*\(`),
	// 报错注入:updatexml / extractvalue
	regexp.MustCompile(`(?i)\b(updatexml|extractvalue|gtid_subset|xmltype)\s*\(`),
	// load_file / into outfile 文件读写
	regexp.MustCompile(`(?i)\bload_file\s*\(|\binto\s+(outfile|dumpfile)\b`),
	// 注释包裹的经典变体:1' or '1'='1' -- -
	regexp.MustCompile(`(?i)\d\s*'\s*\b(or|and)\b`),
}

// IsSQLInjection 判断单个输入值是否包含 SQL 注入特征。
// 适用于查询参数、表单字段、JSON 字符串字段等用户可控输入。
// 注意:只能作为前置拦截,不能替代参数化查询(GORM/PreparedStatement 仍是底线)。
func IsSQLInjection(value string) bool {
	if value == "" {
		return false
	}
	for _, pattern := range sqlPatterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	return false
}

// FindInjection 返回命中的第一个特征描述(审计日志用);未命中返回空串。
func FindInjection(value string) string {
	if value == "" {
		return ""
	}
	for _, pattern := range sqlPatterns {
		if pattern.MatchString(value) {
			return pattern.String()
		}
	}
	return ""
}

// CheckValues 批量检测一组输入值;任一命中返回该值(用于定位)。
// 检测 query 参数时建议把 key 一并传入以便定位。
func CheckValues(values ...string) (hits []string) {
	for _, value := range values {
		if IsSQLInjection(value) {
			hits = append(hits, value)
		}
	}
	return hits
}

// ContainsControlChars 检测不可见控制字符(除 \t\n\r 外),
// 用于拦截协议注入/日志注入(CRLF 注入)。
func ContainsControlChars(value string) bool {
	for _, r := range value {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			return true
		}
	}
	return false
}

// SanitizeLog 去除字符串中的控制字符与换行(日志/审计落库前清洗,防日志注入)。
func SanitizeLog(value string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return '?'
		}
		return r
	}, value)
}
