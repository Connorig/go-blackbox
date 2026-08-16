package ws

import "strings"

// 房间化扩展:Client 与房间多对多归属,房间广播与全局广播并存。
// 设计要点:
//   - 房间表独立 RWMutex,Join/Leave/BroadcastRoom/CountRoom 直接加锁操作,不走事件循环;
//   - 慢连接隔离:房间广播复用 Client.Send(非阻塞,队列满丢弃),不拖垮房间;
//   - 断开清理:removeClient 时收集并清理全部房间归属,回调在锁外触发,避免死锁;
//   - 内存版房间表(单实例);多实例部署需配合 Redis pub/sub 跨节点(后续迭代,文档注明)。

// RoomEvent 是客户端进出房间的生命周期回调。
// 回调在房间表锁外执行,内部可安全调用 BroadcastRoom/CountRoom 等房间 API。
type RoomEvent func(h *Hub, c *Client, room string)

// OnJoin 注册客户端加入房间回调(人数统计/欢迎语/审计)。
func (h *Hub) OnJoin(fn RoomEvent) *Hub {
	if h == nil {
		return h
	}
	h.roomsMu.Lock()
	h.onJoin = fn
	h.roomsMu.Unlock()
	return h
}

// OnLeave 注册客户端离开房间回调(人数推送/资源回收)。
func (h *Hub) OnLeave(fn RoomEvent) *Hub {
	if h == nil {
		return h
	}
	h.roomsMu.Lock()
	h.onLeave = fn
	h.roomsMu.Unlock()
	return h
}

// Join 客户端加入房间(幂等,重复加入不重复计数)。
// 一个连接可同时属于多个房间(多屏/多会话场景)。
func (h *Hub) Join(room string, client *Client) {
	if h == nil || client == nil {
		return
	}
	room = strings.TrimSpace(room)
	if room == "" {
		return
	}
	var callback RoomEvent
	h.roomsMu.Lock()
	if h.rooms == nil {
		h.rooms = make(map[string]map[*Client]struct{})
	}
	members, exists := h.rooms[room]
	if !exists {
		members = make(map[*Client]struct{})
		h.rooms[room] = members
	}
	if _, joined := members[client]; !joined {
		members[client] = struct{}{}
		callback = h.onJoin
	}
	h.roomsMu.Unlock()
	if callback != nil {
		callback(h, client, room)
	}
}

// Leave 客户端离开房间(幂等)。
func (h *Hub) Leave(room string, client *Client) {
	if h == nil || client == nil {
		return
	}
	room = strings.TrimSpace(room)
	if room == "" {
		return
	}
	var callback RoomEvent
	h.roomsMu.Lock()
	if members, exists := h.rooms[room]; exists {
		if _, joined := members[client]; joined {
			delete(members, client)
			if len(members) == 0 {
				delete(h.rooms, room)
			}
			callback = h.onLeave
		}
	}
	h.roomsMu.Unlock()
	if callback != nil {
		callback(h, client, room)
	}
}

// BroadcastRoom 向房间内全部客户端广播(非阻塞,队列满丢弃)。
func (h *Hub) BroadcastRoom(room string, data []byte) {
	if h == nil || data == nil {
		return
	}
	h.roomsMu.RLock()
	members := h.rooms[room]
	clients := make([]*Client, 0, len(members))
	for client := range members {
		clients = append(clients, client)
	}
	h.roomsMu.RUnlock()
	for _, client := range clients {
		client.Send(data)
	}
}

// CountRoom 返回房间在线连接数。
func (h *Hub) CountRoom(room string) int {
	if h == nil {
		return 0
	}
	h.roomsMu.RLock()
	defer h.roomsMu.RUnlock()
	return len(h.rooms[room])
}

// Rooms 返回当前非空房间列表(顺序不保证)。
func (h *Hub) Rooms() []string {
	if h == nil {
		return nil
	}
	h.roomsMu.RLock()
	defer h.roomsMu.RUnlock()
	rooms := make([]string, 0, len(h.rooms))
	for room, members := range h.rooms {
		if len(members) > 0 {
			rooms = append(rooms, room)
		}
	}
	return rooms
}

// leaveAllRooms 移除客户端全部房间归属,返回需触发 OnLeave 的房间列表。
// 调用方必须持有 h.mu 之外的锁环境时注意:本函数不触发回调,回调由调用方在锁外执行。
func (h *Hub) leaveAllRooms(client *Client) []string {
	h.roomsMu.Lock()
	defer h.roomsMu.Unlock()
	var left []string
	for room, members := range h.rooms {
		if _, joined := members[client]; joined {
			delete(members, client)
			if len(members) == 0 {
				delete(h.rooms, room)
			}
			left = append(left, room)
		}
	}
	return left
}
