package room

import (
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

type RoomPlayer struct {
	ID     string          // 玩家ID或用户名
	Conn   *websocket.Conn // 玩家WebSocket连接
	Status string          // 玩家状态（如：在线、离线、准备中）
	Room   *Room           // 玩家所在的房间 用于注销玩家
}

type Room struct {
	ID         string               // 房间ID
	Name       string               // 房间名称
	Hoster     string               // 房主ID或用户名
	RoomPlayer map[*RoomPlayer]bool // 房间内的玩家列表
	regester   chan *RoomPlayer     // 注册玩家的通道
	unregister chan *RoomPlayer     // 注销玩家的通道
	Status     string               // 房间状态（如：等待中、游戏中、已结束）
	mu         sync.RWMutex         // 读写锁，保护房间数据的并发访问
}

// NewRoom 使用房间id创建一个新的房间实例，并将其添加到房间管理器中,启动房间。
func (rm *RoomManager) NewRoom(id string, name string, roomplayer RoomPlayer) {
	rm.mu.Lock()
	room := &Room{
		ID:         id,
		Name:       name,
		Hoster:     roomplayer.ID,
		RoomPlayer: make(map[*RoomPlayer]bool),
		regester:   make(chan *RoomPlayer),
		unregister: make(chan *RoomPlayer),
		Status:     "等待中",
	}
	room.RoomPlayer[&roomplayer] = true // 将房主添加到玩家列表
	rm.Room[room.ID] = room
	rm.mu.Unlock()
	log.Println("房间创建成功: ", room.ID)
	go room.RunRoom() // 启动房间
}

func (r *Room) RunRoom() {
	log.Printf("房间 %s 已启动，等待玩家加入...", r.ID)
	for {
		select {
		case player := <-r.regester:
			r.mu.Lock()
			r.RoomPlayer[player] = true
			r.mu.Unlock()
			log.Printf("玩家 %s 加入了房间 %s, 当前在线 %d", player.ID, r.ID, len(r.RoomPlayer))
		case player := <-r.unregister:
			r.mu.Lock()
			delete(r.RoomPlayer, player)
			r.mu.Unlock()
			log.Printf("玩家 %s 离开了房间 %s, 当前在线 %d", player.ID, r.ID, len(r.RoomPlayer))
			// 如果房间内没有玩家了，可以考虑销毁房间（可选功能）
			if len(r.RoomPlayer) == 0 {
				log.Printf("房间 %s 已空，准备销毁...", r.ID)
			}
		}
	}

}

type RoomManager struct {
	Room map[string]*Room //房间列表 由于Room包含锁，需要使用指针类型以避免值传递
	mu   sync.RWMutex     //读写锁
}

var roomManager = RoomManager{
	Room: make(map[string]*Room),
}
