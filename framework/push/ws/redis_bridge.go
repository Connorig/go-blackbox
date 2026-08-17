package ws

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// 跨节点广播:多实例部署时,房间消息通过 Redis pub/sub 路由到其他节点。
// 单实例(未配置 WithRedis)行为不变;配置后:
//   - Broadcast/BroadcastRoom 本地直发 + Publish 到 Redis channel
//   - 订阅端收到消息后,仅对非本节点的消息做本地转发(避免双发)
//
// 消息协议(JSON):
//
//	{"node_id":"实例标识","room":"房间(空=全局)","data":"<base64>"}

const (
	// defaultRedisChannel Redis channel 默认名。
	defaultRedisChannel = "gbx-ws-broadcast"
	// defaultNodeID 环境变量读取节点标识;未设置时用主机名+随机后缀。
)

// redisBridge Redis 桥接配置状态。
type redisBridge struct {
	client  *redis.Client
	channel string
	nodeID  string
}

// WithRedis 启用跨节点广播:注册 Redis 客户端与 channel(默认 "gbx-ws-broadcast")。
// 必须在 Run 之前调用。
func (h *Hub) WithRedis(client *redis.Client, channel ...string) *Hub {
	if h == nil || client == nil {
		return h
	}
	name := defaultRedisChannel
	if len(channel) > 0 && channel[0] != "" {
		name = channel[0]
	}
	h.bridgeMu.Lock()
	h.bridge = &redisBridge{client: client, channel: name, nodeID: nodeIdentifier()}
	h.bridgeMu.Unlock()
	return h
}

// publishToRedis 将广播消息发布到 Redis channel(本地直发由调用方完成)。
func (h *Hub) publishToRedis(room string, data []byte) {
	h.bridgeMu.RLock()
	bridge := h.bridge
	h.bridgeMu.RUnlock()
	if bridge == nil || bridge.client == nil {
		return
	}
	message := redisMessage{NodeID: bridge.nodeID, Room: room, Data: data}
	payload, err := json.Marshal(message)
	if err != nil {
		zap.L().Warn("ws redis bridge marshal failed", zap.Error(err))
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := bridge.client.Publish(ctx, bridge.channel, payload).Err(); err != nil {
		zap.L().Warn("ws redis bridge publish failed", zap.Error(err))
	}
}

// startRedisSubscription 订阅 Redis channel 并转发他节点消息(goroutine)。
// Run 启动时调用;订阅断线由 go-redis 自动重连。
func (h *Hub) startRedisSubscription(ctx context.Context) {
	h.bridgeMu.RLock()
	bridge := h.bridge
	h.bridgeMu.RUnlock()
	if bridge == nil || bridge.client == nil {
		return
	}
	sub := bridge.client.Subscribe(ctx, bridge.channel)
	go func() {
		defer sub.Close()
		ch := sub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				h.handleRedisMessage(msg.Payload)
			}
		}
	}()
}

// handleRedisMessage 处理订阅消息:非本节点消息转发到本地房间/全局。
func (h *Hub) handleRedisMessage(payload string) {
	var message redisMessage
	if err := json.Unmarshal([]byte(payload), &message); err != nil {
		zap.L().Warn("ws redis bridge unmarshal failed")
		return
	}
	h.bridgeMu.RLock()
	bridge := h.bridge
	h.bridgeMu.RUnlock()
	if bridge != nil && message.NodeID == bridge.nodeID {
		return // 本节点消息已在本地直发,跳过防双发
	}
	if message.Room == "" {
		h.Broadcast(message.Data)
		return
	}
	h.BroadcastRoom(message.Room, message.Data)
}

// redisMessage Redis 桥接消息协议。
type redisMessage struct {
	NodeID string `json:"node_id"`
	Room   string `json:"room"`
	Data   []byte `json:"data"`
}

// nodeIdentifier 生成本节点标识:环境变量优先,否则主机名+随机。
func nodeIdentifier() string {
	if env := os.Getenv("GBX_NODE_ID"); env != "" {
		return env
	}
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "node"
	}
	return host + "-" + randomSuffix()
}

// randomSuffix 随机后缀(6 字节十六进制)。
func randomSuffix() string {
	buffer := make([]byte, 6)
	if _, err := rand.Read(buffer); err != nil {
		return "000000"
	}
	return hex.EncodeToString(buffer)
}
