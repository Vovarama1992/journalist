package ws

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type Hub struct {
	mu    sync.RWMutex
	rooms map[string]map[*websocket.Conn]bool
}

func NewHub() *Hub {
	return &Hub{
		rooms: make(map[string]map[*websocket.Conn]bool),
	}
}

func (h *Hub) Register(roomID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.rooms[roomID]; !ok {
		h.rooms[roomID] = make(map[*websocket.Conn]bool)
	}
	h.rooms[roomID][conn] = true
}

func (h *Hub) Unregister(roomID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	conns, ok := h.rooms[roomID]
	if !ok {
		return
	}

	if _, ok := conns[conn]; ok {
		delete(conns, conn)
		conn.Close()
	}

	if len(conns) == 0 {
		delete(h.rooms, roomID)
	}
}

func (h *Hub) SendToRoom(roomID string, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	conns, ok := h.rooms[roomID]
	if !ok {
		return
	}

	for conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Printf("[hub] send err room=%s: %v", roomID, err)
		}
	}
}

var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}
