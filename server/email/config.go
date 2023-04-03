package email

type MailConnConf struct {
	User  string //发送人邮箱（邮箱以自己的为准）
	Pass  string //发送人邮箱的密码，现在可能会需要邮箱 开启授权密码后在pass填写授权码 jkgolslkqlnsdiid
	Host  string //邮箱服务器（此时用的是qq邮箱）
	Alias string // 邮箱发送别名
}
