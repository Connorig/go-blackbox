package mongodb

import (
	"net/url"
	"strings"
	"time"

	"github.com/snowlyg/helper/str"
)

// MongoDBConfig 定义 MongoDB 连接配置。
// Password 为敏感字段，禁止写入日志或错误信息。
type MongoDBConfig struct {
	Timeout  time.Duration `mapstructure:"timeout" json:"timeout" yaml:"timeout"` // 超时秒数
	DB       string        `mapstructure:"db" json:"db" yaml:"db"`
	Addr     string        `mapstructure:"addr" json:"addr" yaml:"addr"` // host:port
	User     string        `mapstructure:"user" json:"user" yaml:"user"` // 可选用户名
	Password string        `mapstructure:"password" json:"password" yaml:"password"`
}

// GetApplyURI 构造 MongoDB 连接串。
// 提供用户名时使用 URL 编码的凭据；返回的 URI 禁止写入日志。
func (md *MongoDBConfig) GetApplyURI() string {
	if md == nil {
		return ""
	}
	if strings.TrimSpace(md.User) == "" {
		return str.Join("mongodb://", md.Addr, "?connect=direct")
	}
	user := url.QueryEscape(md.User)
	password := url.QueryEscape(md.Password)
	return str.Join("mongodb://", user, ":", password, "@", md.Addr, "?connect=direct")
}
