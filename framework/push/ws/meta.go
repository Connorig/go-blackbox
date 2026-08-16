package ws

// meta 相关方法:业务属性绑定(用户 ID/昵称/角色等),连接升级后即可注入。
// 业务在 onMessage/OnJoin 回调中读取,用于鉴权、统计与定向逻辑。

// SetMeta 设置客户端业务属性(线程安全)。
func (c *Client) SetMeta(key string, value interface{}) {
	if c == nil {
		return
	}
	c.metaMu.Lock()
	defer c.metaMu.Unlock()
	if c.meta == nil {
		c.meta = make(map[string]interface{})
	}
	c.meta[key] = value
}

// Meta 返回客户端业务属性快照(线程安全)。
func (c *Client) Meta() map[string]interface{} {
	if c == nil {
		return nil
	}
	c.metaMu.RLock()
	defer c.metaMu.RUnlock()
	snapshot := make(map[string]interface{}, len(c.meta))
	for key, value := range c.meta {
		snapshot[key] = value
	}
	return snapshot
}

// MetaValue 读取单个业务属性;不存在返回 nil。
func (c *Client) MetaValue(key string) interface{} {
	if c == nil {
		return nil
	}
	c.metaMu.RLock()
	defer c.metaMu.RUnlock()
	return c.meta[key]
}
