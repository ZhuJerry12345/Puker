package game

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
)

func HandleGameH(w http.ResponseWriter, r *http.Request) {
	log.Printf("Serving game.html for %s", r.RemoteAddr)
	// 读取当前目录下的 game.html
	file, err := os.ReadFile("html/game.html")
	if err != nil {
		http.Error(w, "Frontend file not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(file)
}

func HandleStartGame(w http.ResponseWriter, r *http.Request) {
	log.Printf("Start game request from %s", r.RemoteAddr)
	if r.Method != http.MethodPost {
		http.Error(w, `{"error": "Method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}

	var deck *Deck
	var err error
	// 加载牌堆数据
	deck, err = LoadDeckFromYaml("game/poker_cards.yaml")
	if err != nil {
		log.Printf("Error loading deck: %v", err)
		http.Error(w, `{"error": "Failed to load game data"}`, http.StatusInternalServerError)
		return
	}

	log.Printf("Deck loaded successfully with %d cards", len(deck.Cards))

	deck.Shuffle()
	log.Printf("Deck shuffled")

	playerHand1, err := deck.Deliver(14)
	if err != nil {
		log.Printf("Error delivering cards to player 1: %v", err)
		http.Error(w, `{"error": "Failed to deal cards"}`, http.StatusInternalServerError)
		return
	}
	playerHand1.Sort()
	playerHand2, err := deck.Deliver(14)
	if err != nil {
		log.Printf("Error delivering cards to player 2: %v", err)
		http.Error(w, `{"error": "Failed to deal cards"}`, http.StatusInternalServerError)
		return
	}
	playerHand2.Sort()
	playerHand3, err := deck.Deliver(14)
	if err != nil {
		log.Printf("Error delivering cards to player 3: %v", err)
		http.Error(w, `{"error": "Failed to deal cards"}`, http.StatusInternalServerError)
		return
	}
	playerHand3.Sort()

	// 传输给前端
	response := struct {
		Player1 *Deck `json:"player1"`
		Player2 *Deck `json:"player2"`
		Player3 *Deck `json:"player3"`
	}{
		Player1: playerHand1,
		Player2: playerHand2,
		Player3: playerHand3,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Error encoding response: %v", err)
		http.Error(w, `{"error": "Failed to encode response"}`, http.StatusInternalServerError)
		return
	}

}
