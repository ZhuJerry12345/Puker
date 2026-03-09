package main

import (
	"log"
	"net/http"
	"os"
	"puker/game"
	"puker/login"
	"puker/room"
)

// ================= 主函数 =================

func HandleIndex(w http.ResponseWriter, r *http.Request) {
	log.Printf("Serving index.html for %s", r.RemoteAddr)
	// 读取当前目录下的 index.html
	file, err := os.ReadFile("html/index.html")
	if err != nil {
		http.Error(w, "Frontend file not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(file)
}

func main() {
	mux := http.NewServeMux()

	// 公开路由
	mux.HandleFunc("/", HandleIndex)
	mux.HandleFunc("/login", login.HandleLoginH)   // 提供前端页面
	mux.HandleFunc("/signup", login.HandleSignupH) // 提供注册页面
	mux.HandleFunc("/game", game.HandleGameH)      // 提供游戏页面
	mux.HandleFunc("/room", room.HandleRoomH)      // 提供房间页面

	mux.HandleFunc("/api/login", login.HandleLogin)                                 // 登录接口
	mux.HandleFunc("/api/logout", login.HandleLogout)                               // 登出接口
	mux.HandleFunc("/api/check-login", login.HandleCheckLogin)                      // 检查登录状态接口
	mux.HandleFunc("/api/startGame", game.HandleStartGame)                          // 受保护的游戏接口
	mux.HandleFunc("/api/create-room", login.AuthMiddleware(room.HandleCreateRoom)) // 受保护的创建房间接口

	// 受保护路由 (需要 Session)
	mux.HandleFunc("/api/user", login.AuthMiddleware(login.HandleUserData))

	port := ":80"
	log.Printf("服务器启动于 http://localhost%s", port)
	log.Printf("模式：有状态认证 (Session/Cookie)")
	log.Printf("测试账号: admin / 123456")

	if err := http.ListenAndServe(port, mux); err != nil {
		log.Fatal(err)
	}
}
