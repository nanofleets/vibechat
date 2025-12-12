package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"github.com/rs/cors"
)

var (
	rdb      *redis.Client
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins in development
		},
	}
)

type Message struct {
	Nick        string `json:"nick"`
	Text        string `json:"text"`
	Timestamp   int64  `json:"timestamp"`
	Fingerprint string `json:"fingerprint"`
}

type Client struct {
	conn        *websocket.Conn
	nick        string
	fingerprint string
	tagline     string
	send        chan []byte
}

type UserStatus struct {
	Nick        string `json:"nick"`
	Fingerprint string `json:"fingerprint"`
	Tagline     string `json:"tagline"`
	LastSeen    int64  `json:"last_seen"`
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

func newHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) run(ctx context.Context) {
	// Try to subscribe to Redis pub/sub, but don't fail if it's not available
	pubsub := rdb.Subscribe(ctx, "vibechat:messages")
	defer pubsub.Close()

	go func() {
		for {
			msg, err := pubsub.ReceiveMessage(ctx)
			if err != nil {
				log.Printf("Redis pubsub error (will use direct broadcast): %v", err)
				return
			}
			log.Printf("Received from Redis pub/sub: %s", msg.Payload)
			h.broadcast <- []byte(msg.Payload)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("Client registered: %s (total: %d)", client.nick, len(h.clients))
			h.broadcastUserList()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("Client unregistered: %s (total: %d)", client.nick, len(h.clients))
			h.broadcastUserList()
		case message := <-h.broadcast:
			h.mu.Lock()
			log.Printf("Broadcasting to %d clients: %s", len(h.clients), string(message))
			for client := range h.clients {
				select {
				case client.send <- message:
					log.Printf("Sent to client %s", client.nick)
				default:
					log.Printf("Failed to send to client %s, closing", client.nick)
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.Unlock()
		}
	}
}

func (h *Hub) broadcastUserList() {
	h.mu.RLock()
	users := make([]map[string]string, 0, len(h.clients))
	for client := range h.clients {
		users = append(users, map[string]string{
			"nick":        client.nick,
			"fingerprint": client.fingerprint,
			"tagline":     client.tagline,
		})
	}
	h.mu.RUnlock()

	userListMsg := map[string]interface{}{
		"type":  "userlist",
		"users": users,
	}
	data, _ := json.Marshal(userListMsg)
	h.broadcast <- data
}

func (c *Client) readPump(hub *Hub) {
	defer func() {
		log.Printf("[WS] Client %s disconnected", c.nick)
		hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			log.Printf("[WS] Sending to %s: %s", c.nick, string(message))
			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Printf("[WS] Error sending to %s: %v", c.nick, err)
				return
			}
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func handleWebSocket(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nick := r.URL.Query().Get("nick")
		fingerprint := r.URL.Query().Get("fingerprint")

		if nick == "" || fingerprint == "" {
			http.Error(w, "nick and fingerprint required", http.StatusBadRequest)
			return
		}

		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}

		client := &Client{
			conn:        conn,
			nick:        nick,
			fingerprint: fingerprint,
			send:        make(chan []byte, 256),
		}

		log.Printf("[WS] New connection: %s (fingerprint: %s)", nick, fingerprint)
		hub.register <- client

		go client.writePump()
		go client.readPump(hub)
	}
}

func handlePostMessage(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var msg Message
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		msg.Timestamp = time.Now().UnixMilli()

		// Save to Redis with 24h TTL
		ctx := context.Background()
		key := fmt.Sprintf("vibechat:msg:%d", msg.Timestamp)
		data, _ := json.Marshal(msg)
		if err := rdb.Set(ctx, key, data, 24*time.Hour).Err(); err != nil {
			log.Printf("Redis set error: %v", err)
			http.Error(w, "Failed to save message", http.StatusInternalServerError)
			return
		}

		// Prepare message data
		msgData := map[string]interface{}{
			"type":    "message",
			"message": msg,
		}
		pubData, _ := json.Marshal(msgData)

		// Try to publish to Redis pub/sub (for multi-instance setups)
		// If publish succeeds, Redis pub/sub will broadcast to all instances
		// If it fails, fall back to direct broadcast for single instance
		log.Printf("Publishing to Redis: %s", string(pubData))
		if err := rdb.Publish(ctx, "vibechat:messages", pubData).Err(); err != nil {
			log.Printf("Redis publish error (using direct broadcast): %v", err)
			// Only broadcast directly if Redis pub/sub failed
			hub.broadcast <- pubData
		} else {
			log.Printf("Successfully published to Redis")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(msg)
	}
}

func handleGetMessages(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()
	keys, err := rdb.Keys(ctx, "vibechat:msg:*").Result()
	if err != nil {
		http.Error(w, "Failed to get messages", http.StatusInternalServerError)
		return
	}

	messages := []Message{}
	for _, key := range keys {
		val, err := rdb.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		var msg Message
		if err := json.Unmarshal([]byte(val), &msg); err != nil {
			continue
		}
		messages = append(messages, msg)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(messages)
}

// cleanupOldMessages keeps only the latest 500 messages
func cleanupOldMessages(ctx context.Context) {
	keys, err := rdb.Keys(ctx, "vibechat:msg:*").Result()
	if err != nil {
		log.Printf("Failed to get message keys for cleanup: %v", err)
		return
	}

	// If we have 500 or fewer messages, nothing to clean up
	if len(keys) <= 500 {
		return
	}

	// Parse timestamps from keys and sort
	type keyTime struct {
		key       string
		timestamp int64
	}

	keyTimes := make([]keyTime, 0, len(keys))
	for _, key := range keys {
		var ts int64
		if _, err := fmt.Sscanf(key, "vibechat:msg:%d", &ts); err == nil {
			keyTimes = append(keyTimes, keyTime{key: key, timestamp: ts})
		}
	}

	// Sort by timestamp (oldest first)
	sort.Slice(keyTimes, func(i, j int) bool {
		return keyTimes[i].timestamp < keyTimes[j].timestamp
	})

	// Delete oldest messages to keep only 500
	toDelete := len(keyTimes) - 500
	if toDelete > 0 {
		deleted := 0
		for i := 0; i < toDelete; i++ {
			if err := rdb.Del(ctx, keyTimes[i].key).Err(); err != nil {
				log.Printf("Failed to delete old message %s: %v", keyTimes[i].key, err)
			} else {
				deleted++
			}
		}
		log.Printf("Cleanup: deleted %d old messages (total was %d, now %d)", deleted, len(keys), len(keys)-deleted)
	}
}

// startMessageCleanup runs a background job to cleanup old messages
func startMessageCleanup(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cleanupOldMessages(ctx)
			}
		}
	}()
}

func handleUpdateTagline(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Nick        string `json:"nick"`
			Fingerprint string `json:"fingerprint"`
			Tagline     string `json:"tagline"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Limit tagline length
		if len(req.Tagline) > 30 {
			req.Tagline = req.Tagline[:30]
		}

		// Update the client's tagline if connected
		hub.mu.Lock()
		for client := range hub.clients {
			if client.nick == req.Nick && client.fingerprint == req.Fingerprint {
				client.tagline = req.Tagline
				log.Printf("[Tagline] %s set to: %s", client.nick, req.Tagline)
				break
			}
		}
		hub.mu.Unlock()

		// Broadcast updated user list
		hub.broadcastUserList()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func handleTyping(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Nick        string `json:"nick"`
			Fingerprint string `json:"fingerprint"`
			IsTyping    bool   `json:"is_typing"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Broadcast typing indicator
		typingData := map[string]interface{}{
			"type":        "typing",
			"nick":        req.Nick,
			"fingerprint": req.Fingerprint,
			"is_typing":   req.IsTyping,
		}
		data, _ := json.Marshal(typingData)
		hub.broadcast <- data

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}
}

func main() {
	ctx := context.Background()

	// Connect to Redis
	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "localhost:6379"
	}

	rdb = redis.NewClient(&redis.Options{
		Addr: redisURL,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}
	log.Println("Connected to Redis")

	hub := newHub()
	go hub.run(ctx)

	// Start message cleanup job (keeps only 500 latest messages)
	startMessageCleanup(ctx)

	mux := http.NewServeMux()
	mux.HandleFunc("/api/ws", handleWebSocket(hub))
	mux.HandleFunc("/api/chat/messages", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handlePostMessage(hub)(w, r)
		case http.MethodGet:
			handleGetMessages(w, r)
		default:
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/user/tagline", handleUpdateTagline(hub))
	mux.HandleFunc("/api/user/typing", handleTyping(hub))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	// CORS middleware
	handler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	}).Handler(mux)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting server on :%s", port)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
