package live

import (
	"strings"
	"sync"
)

// 客户端注册表:默认实例(空名)+ 命名实例,对齐 datasource 的 NewNamed 模式。
// 多 SRS 集群/多 vhost 场景:SetNamed 注册集群实例,GetNamed 按名获取。
var (
	clientsMu sync.RWMutex
	clients   = map[string]*Client{}
)

// SetGlobal 设置默认客户端(空名注册表项)。
func SetGlobal(client *Client) {
	SetNamed("", client)
}

// SetNamed 注册命名客户端;name 为空等价默认实例。
// client 为 nil 时注销该实例(幂等)。
func SetNamed(name string, client *Client) {
	name = strings.TrimSpace(name)
	clientsMu.Lock()
	defer clientsMu.Unlock()
	if client == nil {
		delete(clients, name)
		return
	}
	clients[name] = client
}

// Get 获取默认客户端;未初始化返回 nil(兼容旧语义,业务判 nil 即可)。
func Get() *Client {
	return getClient("")
}

// GetNamed 获取命名客户端;未注册返回 nil。
func GetNamed(name string) *Client {
	return getClient(strings.TrimSpace(name))
}

// Clients 返回全部已注册客户端快照(name -> client),默认实例名为空字符串。
func Clients() map[string]*Client {
	clientsMu.RLock()
	defer clientsMu.RUnlock()
	snapshot := make(map[string]*Client, len(clients))
	for name, client := range clients {
		snapshot[name] = client
	}
	return snapshot
}

// getClient 内部读取:RLock 保护并发访问。
func getClient(name string) *Client {
	clientsMu.RLock()
	defer clientsMu.RUnlock()
	return clients[name]
}
