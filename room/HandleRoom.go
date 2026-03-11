package room

import (
	"encoding/json"
	"log"
	"net/http"

	"puker/game"
	"puker/login"

	"github.com/gorilla/websocket"
)

/*

 */

// 处理房间页面请求
func HandleRoomH(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "html/room.html")
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源的连接
	},
}

func HandleRoomWebSocket(w http.ResponseWriter, r *http.Request) {
	log.Printf("WebSocket连接请求来自 %s", r.RemoteAddr)

	// 在session中获取玩家信息
	user, err := login.GetUserFromRequest(r)
	if err != nil {
		log.Printf("未找到用户信息: %v", err)
		http.Error(w, `{"error": "未找到用户信息"}`, http.StatusUnauthorized)
		return
	}
	log.Printf("玩家信息: Username=%s", user.Username)

	// 升级到 WebSocket 连接
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		http.Error(w, `{"error": "Failed to upgrade to WebSocket"}`, http.StatusInternalServerError)
		return
	}
	log.Printf("WebSocket连接已建立: %s", r.RemoteAddr)

	// 判断是否已经在房间中,若在房间中，则重设玩家的WebSocket连接,并设置相关房间状态
	PM.mu.RLock()
	if player, exists := PM.PlayerList[user.Username]; exists {
		PM.mu.RUnlock()
		player.mu.Lock()
		log.Printf("玩家 %s 已经在房间 %s 中", user.Username, player.Room.ID)
		player.Conn = conn
		player.Status = "房间中"                // 设置玩家状态为在线
		player.send = make(chan []byte, 256) // 重设发送通道，避免阻塞
		go player.ReadPump()
		go player.WritePump()
		player.mu.Unlock()
		log.Printf("玩家 %s 重连成功", player.ID)
		// 发送玩家列表更新通知
		player.Room.BroadcastPlayerList()
		return
	}
	PM.mu.RUnlock()

	// 创建玩家并添加到在线玩家列表
	player := &Player{
		ID:     user.Username,
		Conn:   conn,
		Status: "空闲",
		send:   make(chan []byte, 256), // 带缓冲的发送通道，避免阻塞
	}
	PM.Resgiter(player)

	log.Println("玩家", user.Username, "上线")
	// 处理玩家的 WebSocket 消息
	go player.ReadPump()
	go player.WritePump()

}

// 根据消息头转发至对应函数处理
func ProcessEvent(p *Player, d []byte) {
	// 读取消息头
	type res struct {
		Event string          `json:"event"`
		Data  json.RawMessage `json:"data"`
	}
	var r res
	err := json.Unmarshal(d, &r)
	if err != nil {
		log.Printf("消息格式错误: %v", err)
		return
	}
	switch r.Event {
	case "Broadcast":
		log.Println("收到Broadcast事件,来源", p.ID, "数据:", string(r.Data))
		PM.Broadcast(p.ID, r.Data)
	case "RoomPlayerList":
		log.Println("收到RoomPlayerList事件,来源", p.ID, "数据:", string(r.Data))
		RoomPlayerList(p, r.Data)
	case "CreateRoom":
		log.Println("收到CreateRoom事件,来源", p.ID, "数据:", string(r.Data))
		RM.CreateRoom(p, r.Data)
	case "JoinRoom":
		log.Println("收到JoinRoom事件,来源", p.ID, "数据:", string(r.Data))
		JoinRoom(p, r.Data)
	case "LeaveRoom":
		log.Println("收到LeaveRoom事件,来源", p.ID, "数据:", string(r.Data))
		LeaveRoom(p, r.Data)
	case "ToggleReady":
		log.Println("收到ToggleReady事件,来源", p.ID, "数据:", string(r.Data))
		ToggleReady(p, r.Data)
	case "UpdatePlayerStatus":
		log.Println("收到UpdatePlayerStatus事件,来源", p.ID, "数据:", string(r.Data))
		UpdatePlayerStatus(p)
	case "StartGame":
		log.Println("收到StartGame事件,来源", p.ID, "数据:", string(r.Data))
		StartGame(p, r.Data)
	default:
		log.Printf("未知事件类型: %s", r.Event)
	}

}

type CardTable struct {
	Hand   *game.Deck `json:"hand"`
	Public *game.Deck `json:"public"`
}

func StartGame(p *Player, d []byte) {
	log.Printf("StartGame event received from player %s with data: %s", p.ID, string(d))

	// 判断是否是房主
	if p.Room == nil {
		log.Printf("玩家 %s 不在任何房间中", p.ID)
		return
	}

	room := p.Room
	room.mu.RLock()
	isOwner := room.Hoster == p.ID
	room.mu.RUnlock()
	if !isOwner {
		log.Printf("玩家 %s 不是房主，无法开始游戏", p.ID)
		return
	}

	// 判断房间内玩家是否都准备了
	room.mu.RLock()
	for _, player := range room.PlayerList {
		if player.Status != "准备中" {
			log.Printf("玩家 %s 状态为 %s,无法开始游戏", player.ID, player.Status)
			room.mu.RUnlock()
			return
		}
	}
	room.mu.RUnlock()

	var deck *game.Deck
	var err error
	// 加载牌堆数据
	deck, err = game.LoadDeckFromYaml("game/poker_cards.yaml")
	if err != nil {
		log.Printf("Error loading deck: %v", err)
		return
	}
	log.Printf("Deck loaded successfully with %d cards", len(deck.Cards))

	// 洗牌
	deck.Shuffle()
	log.Printf("Deck shuffled")

	// 发5张公共牌
	publicCards, err := deck.Deliver(5)
	if err != nil {
		log.Printf("Error delivering cards to player %s: %v", p.ID, err)
		return
	}

	// 发送牌给所有玩家
	for _, player := range room.PlayerList {
		hand, err := deck.Deliver(2)
		if err != nil {
			log.Printf("Error delivering cards to player %s: %v", player.ID, err)
			return
		}
		DeliverCards(player, hand, publicCards)
	}
}

func DeliverCards(p *Player, hand *game.Deck, public *game.Deck) {
	response := struct {
		Event string    `json:"event"`
		Data  CardTable `json:"data"`
	}{
		Event: "GameCards",
		Data: CardTable{
			Hand:   hand,
			Public: public,
		},
	}

	r, _ := json.Marshal(response)
	p.SendMessage(r)
}
