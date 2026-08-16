# WebSocket 房间化使用指南(WS ROOM GUIDELINES)

gbx push/ws 支持房间维度发布/订阅:客户端与房间多对多归属,
房间广播与全局广播并存,业务不再需要自研 RoomHub。

## 一、适用场景

| 场景 | 用法 |
|---|---|
| 直播弹幕 | 推流房间 join 观众连接,BroadcastRoom 弹幕隔离,OnJoin/OnLeave 推人数 |
| 聊天室/群聊/客服会话 | 按会话/群 ID 建房间 |
| 实时协作(文档/画布) | 按文档 ID 隔离 |
| 定向推送 | 按用户/设备建房间 |
| 全站通知/系统公告 | 沿用全局 Broadcast |

## 二、API 一览(在现有 Hub 上增量,完全向后兼容)

```go
// 房间
hub.Join(room, client)              // 加入房间(幂等,可多房间)
hub.Leave(room, client)             // 离开房间(幂等)
hub.BroadcastRoom(room, data)       // 房间内广播(非阻塞,满队列丢弃)
hub.CountRoom(room)                 // 房间在线数
hub.Rooms()                         // 非空房间列表

// 生命周期回调(锁外触发,内部可安全调用房间 API)
hub.OnJoin(func(h *Hub, c *Client, room string) { ... })   // 欢迎语/人数推送/审计
hub.OnLeave(func(h *Hub, c *Client, room string) { ... })  // 人数更新/资源回收

// 业务属性
client.SetMeta("uid", "user-1")     // 连接升级后注入业务属性
client.Meta()                       // 属性快照
client.MetaValue("uid")             // 单值读取
```

## 三、直播弹幕典型用法

```go
hub := ws.NewHub(func(c *ws.Client, data []byte) {
    // 收到客户端弹幕消息 → 解析 → 入库 → 房间广播
    stream := c.MetaValue("stream").(string)
    hub.BroadcastRoom(stream, response)
})
hub.OnJoin(func(h *ws.Hub, c *ws.Client, room string) {
    h.BroadcastRoom(room, viewersJSON(h.CountRoom(room))) // 人数推送
})
hub.OnLeave(func(h *ws.Hub, c *ws.Client, room string) {
    h.BroadcastRoom(room, viewersJSON(h.CountRoom(room)))
})

// WS handler:升级后注入业务属性并加入房间
app.Get("/ws/{stream}", iris.FromStd(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // 用自定义 upgrader 场景:先取得 client 再 Join
})))
```

## 四、设计语义(实现约定)

1. 房间多对多:一个连接可同时进多个房间(多屏场景),Join/Leave 幂等
2. 房间与全局广播并存:Broadcast 全员,BroadcastRoom 只达房间成员
3. 连接断开自动清理:readPump 退出 → 移除全部房间归属 → OnLeave 逐个触发
4. 并发安全:房间表独立 RWMutex;慢连接不拖垮房间(非阻塞 Send)
5. 单实例内存房间表;多实例部署需 Redis pub/sub 跨节点广播(后续迭代)

## 五、验收要点

- 双房间隔离:A 房广播,B 房连接收不到;全局广播仍全员可达
- Join/Leave 幂等、断开自动清理、OnLeave 触发(含房间名)
- CountRoom 与连接数一致(断开清理后)
- 现有 Broadcast/Count/Handle/Run/Close 行为不变
