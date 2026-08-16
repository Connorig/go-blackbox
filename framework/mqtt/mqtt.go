// Package mqtt 提供 MQTT 客户端集成(设备网关采集数据场景):
// 连接 Broker、订阅主题、发布消息、断线自动重连(基于 paho.mqtt.golang)。
package mqtt

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	mqttlib "github.com/eclipse/paho.mqtt.golang"
)

// Config MQTT 客户端配置。
type Config struct {
	// Broker 地址(如 tcp://127.0.0.1:1883 或 ssl://host:8883)。
	Broker string
	// ClientID 客户端标识(网关/服务唯一;空则自动生成)。
	ClientID string
	// Username/Password 认证(可选)。
	Username string
	Password string
	// CleanSession 会话清理(默认 true;false 时断线保留订阅)。
	CleanSession bool
	// Timeout 连接超时(默认 10s)。
	Timeout time.Duration
	// AutoReconnect 断线自动重连(默认 true,paho 内置)。
	AutoReconnect bool
}

// normalize 补齐默认值。
func (c Config) normalize() Config {
	if c.Timeout <= 0 {
		c.Timeout = 10 * time.Second
	}
	if c.ClientID == "" {
		c.ClientID = fmt.Sprintf("gbx-mqtt-%d", time.Now().UnixNano())
	}
	return c
}

// MessageHandler 消息回调(订阅处理)。
type MessageHandler func(topic string, payload []byte)

// Client MQTT 客户端。
type Client struct {
	client mqttlib.Client
	broker string
}

// NewClient 创建并连接 MQTT Broker(连接失败立即报错)。
// onMessage 为全局默认消息回调(所有未单独指定 handler 的订阅共用)。
func NewClient(config Config, onMessage MessageHandler) (*Client, error) {
	cfg := config.normalize()
	if cfg.Broker == "" {
		return nil, errors.New("mqtt: broker is required")
	}
	options := mqttlib.NewClientOptions().
		AddBroker(cfg.Broker).
		SetClientID(cfg.ClientID).
		SetCleanSession(cfg.CleanSession).
		SetAutoReconnect(cfg.AutoReconnect).
		SetConnectTimeout(cfg.Timeout).
		SetOrderMatters(false)
	if cfg.Username != "" {
		options.SetUsername(cfg.Username)
		options.SetPassword(cfg.Password)
	}
	if onMessage != nil {
		options.SetDefaultPublishHandler(func(client mqttlib.Client, message mqttlib.Message) {
			onMessage(message.Topic(), message.Payload())
		})
	}
	client := mqttlib.NewClient(options)
	token := client.Connect()
	if !token.WaitTimeout(cfg.Timeout) {
		return nil, errors.New("mqtt: connect timeout")
	}
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("mqtt: connect: %w", err)
	}
	return &Client{client: client, broker: cfg.Broker}, nil
}

// IsConnected 连接状态。
func (c *Client) IsConnected() bool {
	return c != nil && c.client != nil && c.client.IsConnected()
}

// Subscribe 订阅主题。
// qos:0 最多一次 / 1 至少一次 / 2 仅一次;handler 为空时走全局回调。
func (c *Client) Subscribe(topic string, qos byte, handler MessageHandler) error {
	if c == nil || c.client == nil {
		return errors.New("mqtt: client is nil")
	}
	token := c.client.Subscribe(topic, qos, func(client mqttlib.Client, message mqttlib.Message) {
		if handler != nil {
			handler(message.Topic(), message.Payload())
		}
	})
	token.Wait()
	return token.Error()
}

// Unsubscribe 取消订阅。
func (c *Client) Unsubscribe(topics ...string) error {
	if c == nil || c.client == nil {
		return errors.New("mqtt: client is nil")
	}
	token := c.client.Unsubscribe(topics...)
	token.Wait()
	return token.Error()
}

// Publish 发布消息(原始字节)。
func (c *Client) Publish(topic string, qos byte, retained bool, payload []byte) error {
	if c == nil || c.client == nil {
		return errors.New("mqtt: client is nil")
	}
	token := c.client.Publish(topic, qos, retained, payload)
	token.Wait()
	return token.Error()
}

// PublishJSON 发布 JSON 消息(设备网关上报数据常用)。
func (c *Client) PublishJSON(topic string, qos byte, value interface{}) error {
	payload, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("mqtt: marshal payload: %w", err)
	}
	return c.Publish(topic, qos, false, payload)
}

// Close 断开连接(超时 2s)。
func (c *Client) Close() {
	if c != nil && c.client != nil && c.client.IsConnected() {
		c.client.Disconnect(2000)
	}
}

// Native 返回原生 paho 客户端(高级操作入口)。
func (c *Client) Native() mqttlib.Client {
	if c == nil {
		return nil
	}
	return c.client
}

// 全局便捷入口:NewClient 成功后自动设置,业务直接 mqtt.Get() 获取。

var global *Client

// SetGlobal 设置全局客户端(NewClient 成功后自动调用)。
func SetGlobal(client *Client) { global = client }

// Get 获取全局 MQTT 客户端;未连接返回 nil。
func Get() *Client { return global }
