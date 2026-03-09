package room

import (
	"encoding/json"
	"log"
	"sync"
)

type RoomManager struct {
	RoomList map[string]*Room //房间id到房间实例的映射 由于Room包含锁，需要使用指针类型以避免值传递

	create chan *Room   // 创建房间的通道
	close  chan *Room   // 关闭房间的通道
	done   chan bool    // 处理完成的通道
	mu     sync.RWMutex //读写锁
}

var RM = RoomManager{
	RoomList: make(map[string]*Room),
	create:   make(chan *Room),
	close:    make(chan *Room),
	done:     make(chan bool),
}

func (rm *RoomManager) Run() {
	log.Printf("房间管理器已启动，等待房间创建和关闭请求...")
	for {
		select {
		case room := <-rm.create:
			rm.mu.Lock() // 房间管理器上锁
			rm.RoomList[room.ID] = room
			rm.mu.Unlock()
			log.Printf("房间 %s 已创建, 当前房间总数 %d", room.ID, len(rm.RoomList))
			go room.Run()   // 启动房间的协程
			rm.done <- true // 发送处理完成的信号
		case room := <-rm.close:
			//移除房间内所有玩家
			room.mu.Lock()
			for _, player := range room.PlayerList {
				player.mu.Lock()
				if player.Status != "掉线中" {
					player.Room = nil // 将玩家的房间设置为 nil，表示不在任何房间内
					player.Status = "空闲"
				} else {
					PM.unregister <- player // 玩家掉线且不在房间中，注销玩家
					<-PM.done
				}
				player.mu.Unlock()
			}
			room.mu.Unlock()

			rm.mu.Lock() // 房间管理器上锁
			delete(rm.RoomList, room.ID)
			rm.mu.Unlock()
			log.Printf("房间 %s 已关闭, 当前房间总数 %d", room.ID, len(rm.RoomList))
			rm.done <- true // 发送处理完成的信号
		}
	}
}

func (rm *RoomManager) CreateRoom(p *Player, data json.RawMessage) {
	log.Println("处理创建房间请求,来源", p.ID, "数据:", string(data))
	// type req struct {
	// 	RoomName string `json:"room_name"`
	// }
	// var r req
	// err := json.Unmarshal(data, &r)
	// if err != nil {
	// 	log.Printf("创建房间请求格式错误: %v", err)
	// 	return
	// }

	// 判断用户是否已经在房间中，若在，则拒绝创建房间请求
	p.mu.RLock()
	if p.Room != nil {
		log.Printf("玩家 %s 已经在房间 %s 中，无法创建新房间", p.ID, p.Room.ID)
		p.mu.RUnlock()
		return
	}
	p.mu.RUnlock()

	// 创建房间实例
	room := &Room{
		ID:         p.ID,
		Name:       p.ID,
		Hoster:     p.ID,
		PlayerList: make(map[string]*Player),
		join:       make(chan *Player),
		leave:      make(chan *Player),

		done:   make(chan bool),
		Status: "等待中",
		mu:     sync.RWMutex{},
	}
	// 将房间添加到房间管理器
	rm.create <- room
	<-rm.done // 等待房间创建完成
	// 将玩家加入房间
	room.join <- p
	<-room.done // 等待玩家加入完成

	room.BroadcastPlayerList() // 广播房间玩家列表
}
