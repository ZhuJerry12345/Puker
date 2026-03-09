package room

import (
	"encoding/json"
	"log"
	"sync"
)

type Room struct {
	ID         string             // 房间ID
	Name       string             // 房间名称
	Hoster     string             // 房主ID或用户名
	PlayerList map[string]*Player // 房间内的玩家列表

	send chan []byte // 发送消息的通道

	join  chan *Player // 玩家加入房间的通道
	leave chan *Player // 玩家离开房间的通道
	done  chan bool    // 处理完成的通道

	Status string       // 房间状态（如：等待中、游戏中、已结束）
	mu     sync.RWMutex // 读写锁，保护房间数据的并发访问
}

func (r *Room) Run() {
	log.Printf("房间 %s 已启动，等待玩家加入...", r.ID)
	for {
		select {
		case player := <-r.join:
			log.Printf("玩家 %s 请求加入房间 %s", player.ID, r.ID)
			r.mu.Lock()                      // 房间上锁
			r.PlayerList[player.ID] = player // 将玩家添加到房间的玩家列表
			r.mu.Unlock()

			player.mu.Lock() // 玩家上锁
			player.Room = r  // 将玩家的房间设置为当前房间
			player.Status = "房间中"
			player.mu.Unlock()

			log.Printf("玩家 %s 加入了房间 %s, 当前在线 %d", player.ID, r.ID, len(r.PlayerList))
			r.done <- true
		case player := <-r.leave:
			pName := player.ID
			r.mu.Lock() // 房间上锁
			hName := r.Hoster
			delete(r.PlayerList, player.ID) // 将玩家从房间的玩家列表中移除
			r.mu.Unlock()

			player.mu.Lock() // 玩家上锁
			if player.Status == "掉线中" {
				PM.unregister <- player // 玩家掉线且不在房间中，注销玩家
				<-PM.done
			}
			player.Room = nil // 将玩家的房间设置为 nil，表示不在任何房间内
			player.Status = "空闲"
			player.mu.Unlock()

			log.Printf("玩家 %s 离开了房间 %s, 当前在线 %d", player.ID, r.ID, len(r.PlayerList))

			// 如果房间内没有玩家了，销毁房间
			if len(r.PlayerList) == 0 {
				log.Printf("房间 %s 已空，准备销毁...", r.ID)
				RM.close <- r // 发送关闭房间的请求到房间管理器
				<-RM.done     // 等待房间关闭完成
				return
			}
			// 如果离开的玩家是房主，随机指定一个新的房主
			if pName == hName {
				log.Println("房主离开了房间")
				for _, p := range r.PlayerList {
					r.Hoster = p.ID
					log.Printf("新的房主是 %s", p.ID)
					break
				}
			}
		}
	}
}

func JoinRoom(p *Player, data json.RawMessage) {
	log.Println("处理加入房间请求,来源", p.ID, "数据:", string(data))
	type req struct {
		RoomID string `json:"room_id"`
	}
	var r req
	err := json.Unmarshal(data, &r)
	if err != nil {
		log.Printf("加入房间请求格式错误: %v", err)
		return
	}

	// 判断用户是否已经在房间中，若在，则拒绝加入房间请求
	p.mu.RLock()
	if p.Room != nil {
		log.Printf("玩家 %s 已经在房间 %s 中，无法加入新房间", p.ID, p.Room.ID)
		p.mu.RUnlock()
		return
	}
	p.mu.RUnlock()

	RM.mu.RLock() // 房间管理器上锁
	room, exists := RM.RoomList[r.RoomID]
	RM.mu.RUnlock()
	if !exists {
		log.Printf("房间 %s 不存在", r.RoomID)
		return
	}
	room.join <- p // 发送加入房间的请求到房间协程
	<-room.done    // 等待加入房间完成

	room.BroadcastPlayerList() // 广播房间玩家列表
}

func LeaveRoom(p *Player, data json.RawMessage) {
	log.Println("处理离开房间请求,来源", p.ID, "数据:", string(data))
	type req struct {
		RoomID string `json:"room_id"`
	}
	var r req
	err := json.Unmarshal(data, &r)
	if err != nil {
		log.Printf("离开房间请求格式错误: %v", err)
		return
	}

	RM.mu.RLock() // 房间管理器上锁
	room, exists := RM.RoomList[r.RoomID]
	RM.mu.RUnlock()
	if !exists {
		log.Printf("房间 %s 不存在", r.RoomID)
		return
	}
	room.leave <- p // 发送离开房间的请求到房间协程
	<-room.done     // 等待离开房间完成

	room.BroadcastPlayerList() // 广播房间玩家列表
}

func (r *Room) ReplaceWithRobot(p *Player) {
	log.Printf("玩家 %s 掉线了，正在将其替换为机器人...", p.ID)

	flag := false

	p.mu.Lock() // 玩家上锁
	p.Status = "掉线中"
	p.mu.Unlock()

	r.mu.RLock()
	// 如果房间没有活人，则关闭房间
	for _, player := range r.PlayerList {
		if player.Status != "掉线中" {
			flag = true
			break
		}
	}
	r.mu.RUnlock()
	if !flag {
		log.Printf("房间 %s 没有活人，准备关闭房间...", r.ID)
		RM.close <- r // 发送关闭房间的请求到房间管理器
		<-RM.done     // 等待房间关闭完成
		return
	}

	log.Printf("玩家 %s 已被替换为机器人", p.ID)
	r.BroadcastPlayerList() // 广播房间玩家列表
}

// 根据玩家所在房间的玩家列表生成玩家ID列表，并发送给请求的玩家
func RoomPlayerList(p *Player, data json.RawMessage) {
	log.Println("处理房间玩家列表请求,来源", p.ID, "数据:", string(data))
	type req struct {
		RoomID string `json:"room_id"`
	}
	var r req
	err := json.Unmarshal(data, &r)
	if err != nil {
		log.Printf("房间玩家列表请求格式错误: %v", err)
		return
	}

	room := p.Room
	if room == nil {
		log.Printf("玩家 %s 不在房间中，无法获取玩家列表", p.ID)
		return
	}

	room.mu.RLock() // 房间上锁
	playerIDs := make([]string, 0, len(room.PlayerList))
	for id := range room.PlayerList {
		playerIDs = append(playerIDs, id)
	}
	room.mu.RUnlock()

	type res struct {
		Event string   `json:"event"`
		Data  []string `json:"data"`
	}
	response := res{
		Event: "RoomPlayerList",
		Data:  playerIDs,
	}
	respBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("房间玩家列表响应格式错误: %v", err)
		return
	}
	p.send <- respBytes // 将玩家列表发送给请求的玩家
}

// 因房间成员变动而向房间内所有玩家广播当前玩家列表
func (r *Room) BroadcastPlayerList() {
	log.Println("准备广播房间玩家列表,房间", r.ID)

	type PlayerStatus struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}

	r.mu.RLock() // 房间上锁
	playerList := make([]PlayerStatus, 0, len(r.PlayerList))
	for _, p := range r.PlayerList {
		playerList = append(playerList, PlayerStatus{Name: p.ID, Status: p.Status})
	}
	r.mu.RUnlock()

	type res struct {
		Event string         `json:"event"`
		Data  []PlayerStatus `json:"data"`
	}

	response := res{
		Event: "RoomPlayerList",
		Data:  playerList,
	}

	respBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("房间玩家列表广播格式错误: %v", err)
		return
	}

	r.mu.RLock()
	for _, player := range r.PlayerList {
		if player.Status != "掉线中" {
			player.SendMessage(respBytes) // 向房间内所有玩家发送当前玩家列表
		}
	}
	r.mu.RUnlock()
}
