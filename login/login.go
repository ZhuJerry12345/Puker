package login

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// 模拟数据库
var U1 = User{ID: 1, Username: "admin", Email: "admin@system.com", Role: "admin", Bio: "超级管理员"}
var U2 = User{ID: 2, Username: "user1", Email: "user1@test.com", Role: "user", Bio: "普通测试用户"}

var mockDB = map[string]*User{
	"admin": &U1,
	"user1": &U2,
}
var mockPasswords = map[string]string{
	"admin": "123456",
	"user1": "123456",
}

// ================= 配置与数据结构 =================

const (
	SessionCookieName = "session_id"
	SessionDuration   = 30 * time.Minute // Session 有效期
	UserMaxSessions   = 1                // 每个用户允许的最大 Session 数量 (可选功能，限制单用户多设备登录)
)

// 用户模型
type User struct {
	ID        uint     `json:"id"`
	Username  string   `json:"username"`
	Email     string   `json:"email"`
	Role      string   `json:"role"`
	Bio       string   `json:"bio"`
	SessionID []string `json:"-"` // sessionID用于管理用户登录状态
}

// Session 模型 (存储在服务器端)
type Session struct {
	User      *User     // 关联用户信息
	CreatedAt time.Time // 创建时间
	ExpiresAt time.Time // 过期时间
}

// SessionStore 线程安全的内存存储
// 通过 sync.RWMutex 来保护对 sessions map 的访问，确保在并发环境下的安全性。
// sessions map 存储 sessionID 到 Session 的映射，允许快速查找和管理用户会话。
// 通过Session查询用户的其他SessionID，支持单用户多设备登录。
type SessionStore struct {
	sync.RWMutex
	sessions map[string]*Session
}

// 全局 SessionStore 实例
var store = &SessionStore{
	sessions: make(map[string]*Session),
}

// 生成随机 Session ID
// 实际生产环境中，Session ID 应该足够随机和唯一，以防止被猜测或碰撞。
// ToDo：使用 crypto/rand 包生成一个 32 字节的随机值，并将其编码为十六进制字符串，确保足够的安全性。
func generateSessionID() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// 为用户创建 Session，返回 Session ID
func createSession(user *User) (string, error) {
	id, err := generateSessionID()
	if err != nil {
		return "", err
	}

	// 判断用户是否已达到最大 Session 数量限制 (可选功能)
	if UserMaxSessions > 0 && len(user.SessionID) >= UserMaxSessions {
		return "", fmt.Errorf("用户 '%s' 已达到最大登录设备数量", user.Username)
	}
	log.Printf("用户 '%s' 当前登录设备数量: %d", user.Username, len(user.SessionID))

	now := time.Now()
	session := &Session{
		User:      user,
		CreatedAt: now,
		ExpiresAt: now.Add(SessionDuration),
	}

	store.Lock()
	store.sessions[id] = session
	store.Unlock()

	user.SessionID = append(user.SessionID, id) // 关联用户和 Session ID

	// ToDo:启动一个一次性协程，在过期后清理内存 (简单实现，生产环境可用时间轮或定期清理)
	// go func() {
	// 	time.Sleep(SessionDuration + 1*time.Minute)
	// 	store.Lock()
	// 	delete(store.sessions, id)
	// 	store.Unlock()
	// }()

	return id, nil
}

// 通过Session ID获取 Session
func getSession(id string) (*Session, bool) {
	store.RLock()
	defer store.RUnlock()

	session, exists := store.sessions[id]
	if !exists {
		return nil, false
	}

	// 检查是否过期
	if time.Now().After(session.ExpiresAt) {
		return nil, false
	}

	return session, true
}

// 删除 Session (登出)
func deleteSession(id string) {
	store.Lock()
	defer store.Unlock()
	user := store.sessions[id].User
	// 从用户的 SessionID 列表中移除
	for i, sid := range user.SessionID {
		if sid == id {
			user.SessionID = append(user.SessionID[:i], user.SessionID[i+1:]...)
			break
		}
	}
	delete(store.sessions, id)
}

// ================= 中间件 =================

// authMiddleware: 验证 Cookie 中的 SessionID
// 验证通过后，用户信息User被注入到r.Context()中，标签为 "authenticated_user"
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("认证中间件启动")
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil || cookie.Value == "" {
			log.Println("认证失败: 没有找到cookie")
			http.Error(w, `{"error": "认证失败: 没有找到cookie"}`, http.StatusUnauthorized)
			return
		}
		log.Println("Session ID found in cookie:", cookie.Value)

		session, exists := getSession(cookie.Value)
		if !exists {
			// Session 过期或不存在，清除 Cookie
			http.SetCookie(w, &http.Cookie{
				Name:   SessionCookieName,
				Value:  "",
				Path:   "/",
				MaxAge: -1,
			})
			log.Println("Session过期或不存在")
			http.Error(w, `{"error": "认证失败: Session 过期"}`, http.StatusUnauthorized)
			return
		}
		log.Println("Session User:", session.User.Username)

		// // 将用户信息注入 Context
		// // 定义 context key
		// type contextKey string
		// userKey := contextKey("authenticated_user")

		// newR := r.WithContext(context.WithValue(r.Context(), userKey, session.User))
		newR := r.WithContext(context.WithValue(r.Context(), "authenticated_user", session.User))
		log.Println("认证中间件结束")

		next(w, newR)
	}
}

// 检索链接对应的用户
func GetUserFromRequest(r *http.Request) (*User, error) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil, fmt.Errorf("未找到用户信息")
	}

	store.Lock()
	session, exists := store.sessions[cookie.Value]
	store.Unlock()

	if !exists || time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("用户信息已过期")
	}

	return session.User, nil
}

// 检查登录状态，error为nil表示已登录，返回用户信息；error不为nil表示未登录或Session无效
func CheckLogin(r *http.Request) (*User, error) {
	cookie, err := r.Cookie(SessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil, fmt.Errorf("未找到Cookie信息")
	}

	store.Lock()
	session, exists := store.sessions[cookie.Value]
	store.Unlock()

	if !exists || time.Now().After(session.ExpiresAt) { //seesion不存在或超时
		return nil, fmt.Errorf("用户信息已过期")
	}

	return session.User, nil
}

// ================= 业务 Handler =================

// 处理静态文件 (login.html)
func HandleLoginH(w http.ResponseWriter, r *http.Request) {
	log.Printf("Serving login.html for %s", r.RemoteAddr)
	// 读取当前目录下的 index.html
	file, err := os.ReadFile("html/login.html")
	if err != nil {
		http.Error(w, "Frontend file not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(file)
}

// 处理静态文件 (signup.html)
func HandleSignupH(w http.ResponseWriter, r *http.Request) {
	log.Printf("Serving signup.html for %s", r.RemoteAddr)
	// 读取当前目录下的 signup.html
	file, err := os.ReadFile("html/signup.html")
	if err != nil {
		http.Error(w, "Frontend file not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(file)
}

// 处理检查登录状态
func HandleCheckLogin(w http.ResponseWriter, r *http.Request) {
	user, err := CheckLogin(r)
	if err != nil {
		http.Error(w, `{"error": "未登录"}`, http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":  "已登录",
		"username": user.Username,
	})
}

// 处理登录
func HandleLogin(w http.ResponseWriter, r *http.Request) {
	log.Printf("Login attempt from %s", r.RemoteAddr)
	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error": "Invalid JSON"}`, http.StatusBadRequest)
		return
	}

	// 1. 验证账号密码
	var user *User
	user, exists := mockDB[req.Username]
	if !exists || mockPasswords[req.Username] != req.Password {
		log.Printf("尝试登录用户名: %s 失败", req.Username)
		http.Error(w, `{"error": "Invalid username or password"}`, http.StatusUnauthorized)
		return
	}
	log.Printf("用户 '%s' 认证成功", req.Username)

	// 2. 创建服务器端 Session (有状态核心)
	sessionID, err := createSession(user)
	if err != nil {
		log.Printf("创建session时出错: %v", err)
		http.Error(w, `{"error": "Internal server error"}`, http.StatusInternalServerError)
		return
	}
	log.Printf("Session created for user '%s'", req.Username)

	// 3. 设置 HttpOnly Cookie (关键安全步骤)
	// HttpOnly: 防止 JS 读取，防 XSS
	// Path: /: 整个站点有效
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // 生产环境 HTTPS 时应设为 true
		MaxAge:   int(SessionDuration.Seconds()),
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message":  "Login successful",
		"username": user.Username,
	})
	log.Printf("Login successful for user '%s', session ID: %s", req.Username, sessionID)
}

// 处理登出
func HandleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	cookie, err := r.Cookie(SessionCookieName)
	if err == nil && cookie.Value != "" {
		deleteSession(cookie.Value) // 从服务器内存删除
	}

	// 清除浏览器 Cookie
	http.SetCookie(w, &http.Cookie{
		Name:   SessionCookieName,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Logged out successfully",
	})
}

// 处理受保护的用户数据 (/api/user)
func HandleUserData(w http.ResponseWriter, r *http.Request) {
	// // 从 Context 获取用户 (由中间件注入)
	// type contextKey string
	// userKey := contextKey("authenticated_user")

	// user, ok := r.Context().Value(userKey).(User)
	user, ok := r.Context().Value("authenticated_user").(*User)
	if !ok {
		log.Println("Error retrieving user from context")
		http.Error(w, `{"error": "User context missing"}`, http.StatusInternalServerError)
		return
	}

	// 构造返回数据
	responseData := map[string]interface{}{
		"id":          user.ID,
		"username":    user.Username,
		"email":       user.Email,
		"role":        user.Role,
		"bio":         user.Bio,
		"server_time": time.Now().Format(time.RFC3339),
		"message":     "Access granted via Stateful Session.",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(responseData)
}
