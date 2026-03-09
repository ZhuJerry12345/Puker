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
		PM.Broadcast(r.Data, p.ID)
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
	case "StartGame":
		log.Println("收到StartGame事件,来源", p.ID, "数据:", string(r.Data))
		StartGame(p, r.Data)
	default:
		log.Printf("未知事件类型: %s", r.Event)
	}

}

func StartGame(p *Player, d []byte) {
	log.Printf("StartGame event received from player %s with data: %s", p.ID, string(d))
	// 目前仅给该玩家发牌，后续可以扩展为给房间内所有玩家发牌
	var deck *game.Deck
	var err error
	// 加载牌堆数据
	deck, err = game.LoadDeckFromYaml("game/poker_cards.yaml")
	if err != nil {
		log.Printf("Error loading deck: %v", err)
		return
	}

	log.Printf("Deck loaded successfully with %d cards", len(deck.Cards))

	deck.Shuffle()
	log.Printf("Deck shuffled")

	playerHand, err := deck.Deliver(14)
	if err != nil {
		log.Printf("Error delivering cards to player %s: %v", p.ID, err)
		return
	}
	playerHand.Sort()

	// 发送牌给玩家
	response := struct {
		Event string     `json:"event"`
		Data  *game.Deck `json:"data"`
	}{
		Event: "StartGame",
		Data:  playerHand,
	}
	if err := p.Conn.WriteJSON(response); err != nil {
		log.Printf("Error sending StartGame event to player %s: %v", p.ID, err)
	}
}
