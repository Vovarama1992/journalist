package domain

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"github.com/Vovarama1992/journalist/internal/models"
	"github.com/Vovarama1992/journalist/internal/ports"
)

type MediaService struct {
	repo   ports.MediaRepository
	stt    ports.STTService
	events chan ports.ChunkEvent
}

func NewMediaService(repo ports.MediaRepository, stt ports.STTService) *MediaService {
	return &MediaService{
		repo:   repo,
		stt:    stt,
		events: make(chan ports.ChunkEvent, 100),
	}
}

func (s *MediaService) Events() <-chan ports.ChunkEvent {
	return s.events
}

//////////////////////////////////////////////////////////////
// capture5secWav — ВЫРЕЗАЕТ РОВНО 5 СЕК PCM 16k mono s16le
//////////////////////////////////////////////////////////////

func (s *MediaService) capture5secWav(src string) ([]byte, error) {
	cmd := exec.Command(
		"ffmpeg",
		"-y",
		"-i", src,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-t", "5",
		"-f", "s16le",
		"pipe:1",
	)

	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg: %w", err)
	}

	data := out.Bytes()
	if len(data) < 8000 {
		return nil, fmt.Errorf("wav too small: %d bytes", len(data))
	}

	return data, nil
}

//////////////////////////////////////////////////////////////
// ProcessMedia — чистый ПТРП: каждые 5 сек ffmpeg → STT → WS
//////////////////////////////////////////////////////////////

func (s *MediaService) ProcessMedia(ctx context.Context, sourceURL, mediaType, roomID string) (*models.Media, error) {

	log.Printf("[media] start PTRP: %.60s…", sourceURL)

	// --- YouTube резолв ---
	if strings.Contains(sourceURL, "youtube") || strings.Contains(sourceURL, "youtu.be") {
		log.Printf("[media] youtube detected, resolving…")
		u, err := ResolveYouTube(sourceURL)
		if err != nil {
			return nil, fmt.Errorf("resolve youtube failed: %w", err)
		}
		log.Printf("[media] resolved: %.60s…", u)
		sourceURL = u
	}

	// --- создаём media запись ---
	media, err := s.repo.InsertMedia(ctx, &models.Media{
		SourceURL: sourceURL,
		Type:      mediaType,
	})
	if err != nil {
		return nil, err
	}

	/////////////////////////////////////////////////////////
	// Запускам бесконечный цикл ПТРП
	/////////////////////////////////////////////////////////

	go func() {
		chunkNum := 1

		for {
			select {
			case <-ctx.Done():
				log.Printf("[media] ctx done — stop PTRP")
				return
			default:
			}

			// 1) режем WAV 5 сек
			wav, err := s.capture5secWav(sourceURL)
			if err != nil {
				log.Printf("[media] capture error: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			// 2) STT
			txt, err := s.stt.Recognize(ctx, wav)
			if err != nil {
				log.Printf("[media] STT err chunk=%d: %v", chunkNum, err)
				chunkNum++
				continue
			}

			txt = strings.TrimSpace(txt)
			if txt == "" {
				log.Printf("[media] empty text chunk=%d", chunkNum)
				chunkNum++
				continue
			}

			// 3) сохр в БД
			_ = s.repo.InsertChunk(ctx, &models.MediaChunk{
				MediaID:     media.ID,
				ChunkNumber: chunkNum,
				Text:        txt,
			})

			// 4) пуш в websocket
			log.Printf("[media] SEND chunk=%d media=%d: %.40s", chunkNum, media.ID, txt)

			s.events <- ports.ChunkEvent{
				MediaID:     media.ID,
				ChunkNumber: chunkNum,
				RoomID:      roomID,
				Text:        txt,
			}

			chunkNum++
		}
	}()

	return media, nil
}
