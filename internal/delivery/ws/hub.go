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
	log.Printf("[hub] init")
	return &Hub{
		rooms: make(map[string]map[*websocket.Conn]bool),
	}
}

func (h *Hub) Register(roomID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.rooms[roomID]; !ok {
		h.rooms[roomID] = make(map[*websocket.Conn]bool)
		log.Printf("[hub] create room=%s", roomID)
	}

	h.rooms[roomID][conn] = true
	log.Printf("[hub] register room=%s conns=%d", roomID, len(h.rooms[roomID]))
}

func (h *Hub) Unregister(roomID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	conns, ok := h.rooms[roomID]
	if !ok {
		log.Printf("[hub] unregister skip: no room=%s", roomID)
		return
	}

	if _, ok := conns[conn]; ok {
		delete(conns, conn)
		conn.Close()
		log.Printf("[hub] unregister room=%s conns=%d", roomID, len(conns))
	}

	if len(conns) == 0 {
		delete(h.rooms, roomID)
		log.Printf("[hub] delete room=%s", roomID)
	}
}

func (h *Hub) SendToRoom(roomID string, msg []byte) {
	h.mu.RLock()
	conns, ok := h.rooms[roomID]
	connCount := len(conns)
	h.mu.RUnlock()

	// === КЛЮЧЕВОЙ ЛОГ ===
	if !ok || connCount == 0 {
		log.Printf("[hub][SEND-SKIP] room=%s reason=no_active_connections", roomID)
		return
	}

	log.Printf("[hub][SEND] room=%s conns=%d bytes=%d", roomID, connCount, len(msg))

	h.mu.RLock()
	defer h.mu.RUnlock()

	for conn := range conns {
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			log.Printf("[hub][SEND-ERR] room=%s err=%v", roomID, err)
		} else {
			log.Printf("[hub][SEND-OK] room=%s", roomID)
		}
	}
}

var Upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}
