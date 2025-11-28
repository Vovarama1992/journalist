package delivery

import (
	"fmt"
	"log"
	"net/http"

	"github.com/Vovarama1992/journalist/internal/domain"
)

func WSHandler(hub *Hub, mediaService *domain.MediaService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := Upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, "failed to upgrade", http.StatusBadRequest)
			return
		}

		roomID := r.URL.Query().Get("roomID")
		if roomID == "" {
			roomID = "default"
		}

		hub.Register(roomID, conn)
		defer hub.Unregister(roomID)

		//------------------------------------------------------
		// получаем от клиента URL для обработки
		//------------------------------------------------------

		_, msg, err := conn.ReadMessage()
		if err != nil {
			log.Printf("read url failed: %v", err)
			return
		}
		url := string(msg)

		_ = hub.SendToRoom(roomID, []byte("processing started"))

		//------------------------------------------------------
		// слушаем события чанков → пушим в WebSocket
		//------------------------------------------------------

		go func() {
			for event := range mediaService.Events() {
				payload := []byte(
					fmt.Sprintf(`{"mediaId": %d, "chunk": %d, "text": "%s"}`,
						event.MediaID,
						event.ChunkNumber,
						event.Text,
					),
				)
				hub.SendToRoom(roomID, payload)
			}
		}()

		//------------------------------------------------------
		// запускаем обработку медиа
		//------------------------------------------------------

		go func() {
			_, err := mediaService.ProcessMedia(r.Context(), url, "audio")
			if err != nil {
				log.Printf("process media error: %v", err)
			}
			hub.SendToRoom(roomID, []byte("processing finished"))
		}()

		//------------------------------------------------------
		// держим WebSocket открытым
		//------------------------------------------------------

		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				return
			}
		}
	}
}
