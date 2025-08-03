package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"file-ranking/internal/logger"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // 允许所有来源
		},
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

type WebSocketHub struct {
	clients    map[*websocket.Conn]bool
	broadcast  chan []byte
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.RWMutex
}

func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan []byte, 100),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
}

func (h *WebSocketHub) Run() {
	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			h.clients[conn] = true
			h.mu.Unlock()
			logger.GetInstance().Info("WebSocket client connected")

		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[conn]; ok {
				delete(h.clients, conn)
				conn.Close()
			}
			h.mu.Unlock()
			logger.GetInstance().Info("WebSocket client disconnected")

		case message := <-h.broadcast:
			h.mu.RLock()
			for conn := range h.clients {
				err := conn.WriteMessage(websocket.TextMessage, message)
				if err != nil {
					closeConn := conn
					go func() {
						h.unregister <- closeConn
					}()
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *WebSocketHub) HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		logger.GetInstance().Error("WebSocket upgrade error: %v", err)
		return
	}

	h.register <- conn

	go func() {
		defer func() {
			h.unregister <- conn
		}()
		
		conn.SetReadLimit(1024)
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			conn.SetPongHandler(func(string) error {
				conn.SetReadDeadline(time.Now().Add(60 * time.Second))
				return nil
			})

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				break
			}
		}
	}()
}

func (h *WebSocketHub) BroadcastRanking(ranking interface{}) {
	data, err := json.Marshal(ranking)
	if err != nil {
		logger.GetInstance().Error("Broadcast JSON marshal error: %v", err)
		return
	}

	select {
	case h.broadcast <- data:
	default:
		// 丢弃消息避免阻塞
	}
}