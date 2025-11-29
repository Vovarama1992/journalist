package ws

import (
	"log"
	"net/http"

	"github.com/Vovarama1992/journalist/internal/domain"
)

func WSHandler(hub *Hub, mediaService *domain.MediaService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// апгрейд до ws
		conn, err := Upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, "failed to upgrade", http.StatusBadRequest)
			return
		}

		// roomID
		roomID := r.URL.Query().Get("roomID")
		if roomID == "" {
			roomID = "default"
		}

		log.Printf("[WS] connected room=%s", roomID)

		// регистрируем соединение
		hub.Register(roomID, conn)
		defer hub.Unregister(roomID)

		//----------------------------------------------------
		// 1) читаем URL от клиента
		//----------------------------------------------------
		_, urlBytes, err := conn.ReadMessage()
		if err != nil {
			log.Printf("[WS] read url failed room=%s err=%v", roomID, err)
			return
		}
		url := string(urlBytes)

		log.Printf("[WS] room=%s url=%s", roomID, url)

		// пушим клиенту: начало обработки
		hub.SendToRoom(roomID, []byte(`{"status":"processing_started"}`))

		//----------------------------------------------------
		// 2) запускаем процесс FFmpeg + STT
		//----------------------------------------------------
		go func() {
			_, err := mediaService.ProcessMedia(r.Context(), url, "audio", roomID)
			if err != nil {
				log.Printf("[WS] process error room=%s err=%v", roomID, err)
			}

			hub.SendToRoom(roomID, []byte(`{"status":"processing_finished"}`))
			log.Printf("[WS] finished room=%s", roomID)
		}()

		//----------------------------------------------------
		// 3) держим WebSocket открытым
		//----------------------------------------------------
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				log.Printf("[WS] disconnected room=%s", roomID)
				return
			}
		}
	}
}
