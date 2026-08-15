package email

// MailConnConf 定义 SMTP 发送配置。
// Pass 为敏感字段，禁止写入日志。
type MailConnConf struct {
	User  string // 发送人邮箱
	Pass  string // 发送人邮箱的授权码（由邮箱服务商签发）
	Host  string // 邮箱服务器（例如 smtp.qq.com）
	Alias string // 邮箱发送别名
	Port  int    // SMTP 端口；非正数时使用 465（465 使用 SSL，587 使用 STARTTLS）
}
