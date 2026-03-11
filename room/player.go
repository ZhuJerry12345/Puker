package room

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Player struct {
	ID   string          // 玩家ID或用户名
	Conn *websocket.Conn // 玩家WebSocket连接

	Status string // 玩家状态（如：掉线中、房间中、准备中）
	Room   *Room  // 玩家所在的房间 用于注销玩家

	send chan []byte // 发送消息的通道

	mu sync.RWMutex // 读写锁，保护玩家数据的并发访问
}

type PlayerManager struct {
	PlayerList map[string]*Player // 玩家ID到玩家实例的映射
	mu         sync.RWMutex       // 读写锁，保护玩家数据的并发访问

	register   chan *Player // 注册玩家的通道
	unregister chan *Player // 注销玩家的通道
	done       chan bool    // 处理完成的通道
}

var PM = PlayerManager{
	PlayerList: make(map[string]*Player),
	register:   make(chan *Player),
	unregister: make(chan *Player),
	done:       make(chan bool),
}

// 接收玩家发送的信息，并检查ws是否断开
// 若ws断开且玩家不在任何房间内，则将玩家从全局玩家列表中移除，避免内存泄漏
func (p *Player) ReadPump() {
	defer func() {
		close(p.send) // 关闭发送通道，通知 WritePump 退出
		p.Conn.Close()
		if p.Room == nil { // 不在房间中，注销玩家
			PM.unregister <- p
			<-PM.done
		} else { // 在房间中，记录玩家为掉线状态
			p.Room.ReplaceWithRobot(p) // 将玩家替换为机器人，保持游戏继续
		}
	}()

	for {
		// 读取消息
		_, data, err := p.Conn.ReadMessage()
		if err != nil {
			log.Printf("玩家 %s 连接断开: %v", p.ID, err)
			break
		}
		// 打印玩家信息
		log.Printf("玩家 %s 发送消息: %s", p.ID, string(data))
		// 处理玩家发送的消息（如：加入房间、离开房间、游戏操作等）
		ProcessEvent(p, data)
	}
}

func (p *Player) WritePump() {
	ticker := time.NewTicker(30 * time.Second) // 心跳间隔
	defer func() {
		ticker.Stop()

		p.Conn.Close()
	}()
	for {
		select {
		case message, ok := <-p.send:
			if !ok {
				// 发送通道关闭，关闭连接
				p.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			p.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second)) // 设置写超时
			if err := p.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("玩家 %s 发送消息失败: %v", p.ID, err)
				return
			}
		case <-ticker.C:
			// 发送心跳包
			p.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second)) // 设置写超时
			if err := p.Conn.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Printf("玩家 %s 心跳包发送失败: %v", p.ID, err)
				return
			}
		}
	}
}

func (p *Player) SendMessage(message []byte) {
	p.mu.Lock()
	p.send <- message
	p.mu.Unlock()
}

// 注册玩家
func (pm *PlayerManager) Resgiter(p *Player) {
	pm.register <- p
	<-pm.done
}

// 注销玩家
func (pm *PlayerManager) Unregister(p *Player) {
	pm.unregister <- p
	<-pm.done
}

// 获取在线玩家列表
func (pm *PlayerManager) GetOnlinePlayers() []string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	players := make([]string, 0, len(pm.PlayerList))
	for id := range pm.PlayerList {
		players = append(players, id)
	}
	return players
}

// Run 启动房间管理器，监听注册和注销通道，处理玩家的加入和离开,创建和销毁房间。
// 注意：当玩家从所有房间离开后且中断连接时，才将玩家移除出全局玩家列表，避免玩家在房间内切换时被误删除。
func (pm *PlayerManager) Run() {
	log.Println("房间管理器已启动，等待玩家注册和注销...")
	for {
		select {
		case player := <-pm.register:
			pm.mu.Lock()
			pm.PlayerList[player.ID] = player
			pm.mu.Unlock()
			log.Printf("玩家 %s 已注册, 当前在线玩家数: %d", player.ID, len(pm.PlayerList))
			pm.done <- true
		case player := <-pm.unregister:
			pm.mu.Lock()
			delete(pm.PlayerList, player.ID)
			pm.mu.Unlock()
			log.Printf("玩家 %s 已注销, 当前在线玩家数: %d", player.ID, len(pm.PlayerList))
			pm.done <- true
		}
	}
}

func UpdatePlayerStatus(p *Player) {
	log.Println("处理更新玩家状态请求,来源", p.ID)
	type msg struct {
		Event  string `json:"event"`
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	m := msg{
		Event: "ChatUpdatePlayerStatus",
		Name:  p.ID,
	}
	p.mu.RLock()
	m.Status = p.Status
	p.mu.RUnlock()

	message, _ := json.Marshal(m)
	p.SendMessage(message)
}

// 广播消息给所有在线玩家
func (pm *PlayerManager) Broadcast(from string, d json.RawMessage) {
	log.Println("准备广播消息,来源", from, "数据:", string(d))
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	// 解析消息内容
	type res struct {
		Message string `json:"message"`
	}
	var r res
	err := json.Unmarshal(d, &r)
	if err != nil {
		log.Printf("消息格式错误: %v", err)
		return
	}

	// 构造广播消息
	type msg struct {
		Event string      `json:"event"`
		Data  interface{} `json:"data"`
	}
	type data struct {
		From    string `json:"from"`
		Message string `json:"message"`
	}
	m := msg{
		Event: "ChatBroadcast",
		Data: data{
			From:    from,
			Message: r.Message,
		},
	}
	message, _ := json.Marshal(m)
	for _, player := range pm.PlayerList {
		log.Printf("向玩家 %s 广播消息: %s", player.ID, string(message))
		player.SendMessage(message)
	}
}

// 返回在线玩家列表给请求的玩家
func (pm *PlayerManager) PlayList(p *Player) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	type msg struct {
		Event string      `json:"event"`
		Data  interface{} `json:"data"`
	}
	type data struct {
		Players []string `json:"players"`
	}
	players := make([]string, 0, len(pm.PlayerList))
	for id := range pm.PlayerList {
		players = append(players, id)
	}
	m := msg{
		Event: "PlayList",
		Data: data{
			Players: players,
		},
	}
	message, _ := json.Marshal(m)
	p.SendMessage(message)
}
