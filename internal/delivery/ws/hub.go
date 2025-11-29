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

	if h.rooms[roomID] == nil {
		h.rooms[roomID] = make(map[*websocket.Conn]bool)
	}
	h.rooms[roomID][conn] = true
}

func (h *Hub) Unregister(roomID string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if conns, ok := h.rooms[roomID]; ok {
		for conn := range conns {
			conn.Close()
			delete(conns, conn)
		}
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
			log.Printf("[hub] sendToRoom err room=%s: %v", roomID, err)
		}
	}
}

func (h *Hub) Broadcast(msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for roomID, conns := range h.rooms {
		for conn := range conns {
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				log.Printf("[hub] broadcast err room=%s: %v", roomID, err)
			}
		}
	}
}

var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}
