package email

// 全局便捷入口:NewSender 成功后自动设置,业务直接 mail.Get() 获取。

var global *Client

// SetGlobal 设置全局客户端(NewSender 成功后自动调用)。
func SetGlobal(client *Client) { global = client }

// Get 获取全局邮件客户端;未初始化时返回 nil。
func Get() *Client { return global }
