package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
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
	send        chan []byte
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
	// Subscribe to Redis pub/sub
	pubsub := rdb.Subscribe(ctx, "vibechat:messages")
	defer pubsub.Close()

	go func() {
		for {
			msg, err := pubsub.ReceiveMessage(ctx)
			if err != nil {
				log.Printf("Redis pubsub error: %v", err)
				return
			}
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
			h.broadcastUserList()
		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			h.broadcastUserList()
		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
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

			if err := c.conn.WriteMessage(websocket.TextMessage, message); err != nil {
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

		hub.register <- client

		go client.writePump()
		go client.readPump(hub)
	}
}

func handlePostMessage(w http.ResponseWriter, r *http.Request) {
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

	// Publish to pub/sub
	msgData := map[string]interface{}{
		"type":    "message",
		"message": msg,
	}
	pubData, _ := json.Marshal(msgData)
	if err := rdb.Publish(ctx, "vibechat:messages", pubData).Err(); err != nil {
		log.Printf("Redis publish error: %v", err)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msg)
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

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", handleWebSocket(hub))
	mux.HandleFunc("/api/chat/messages", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			handlePostMessage(w, r)
		} else if r.Method == http.MethodGet {
			handleGetMessages(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
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
