package room

import (
	"encoding/json"
	"log"
	"net/http"

	"puker/login"

	"github.com/gorilla/websocket"
)

/*
RoomManager 负责管理所有的房间，提供创建、删除和查询房间的功能。
Room 代表一个游戏房间，包含房间ID、名称、房主信息、玩家列表和房间状态。
RoomPlayer 代表一个房间内的玩家，包含玩家ID、WebSocket连接和状态。
通过使用通道（regester 和 unregister）来处理玩家的加入和离开，确保线程安全。
使用读写锁（mu）来保护对房间数据的并发访问，避免数据竞争和不一致问题。
提供 HandleCreateRoom 函数作为创建房间的 HTTP 处理器，验证用户身份并返回房主信息。
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

func HandleCreateRoom(w http.ResponseWriter, r *http.Request) {
	log.Printf("创建房间 request from %s", r.RemoteAddr)
	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	// 在session中获取房主信息
	user, err := login.GetUserFromRequest(r)
	if err != nil {
		http.Error(w, `{"error": "未找到用户信息"}`, http.StatusUnauthorized)
		return
	}

	// 测试：返回房主信息
	log.Printf("房主信息: Username=%s", user.Username)

	type Response struct {
		HosterName string `json:"HosterName"`
	}
	resp := Response{HosterName: user.Username}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		http.Error(w, `{"error": "Failed to encode response"}`, http.StatusInternalServerError)
		return
	}

}
