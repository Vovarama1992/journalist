package delivery

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

// Hub управляет WebSocket-соединениями
type Hub struct {
	mu          sync.Mutex
	connections map[string]*websocket.Conn // roomID/userID -> conn
}

// NewHub создаёт новый хаб
func NewHub() *Hub {
	return &Hub{
		connections: make(map[string]*websocket.Conn),
	}
}

// Register добавляет подключение
func (h *Hub) Register(roomID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connections[roomID] = conn
}

// Unregister удаляет подключение
func (h *Hub) Unregister(roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if conn, ok := h.connections[roomID]; ok {
		conn.Close()
		delete(h.connections, roomID)
	}
}

// SendToRoom отправляет сообщение клиенту по roomID
func (h *Hub) SendToRoom(roomID string, message []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	conn, ok := h.connections[roomID]
	if !ok {
		return nil
	}
	return conn.WriteMessage(websocket.TextMessage, message)
}

// Upgrader для WebSocket
var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}
