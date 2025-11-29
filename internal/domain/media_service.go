package domain

import (
	"bufio"
	"context"
	"fmt"
	"io"
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

///////////////////////////////////////////////////////////////////////
// 1) Новый метод: получить ровно 5 секунд PCM WAV
///////////////////////////////////////////////////////////////////////

func (s *MediaService) capture5secWav(url string) ([]byte, error) {
	/*
		5 секунд PCM s16le → полностью совместимо с Yandex
	*/

	cmd := exec.Command(
		"ffmpeg",
		"-i", url,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-t", "5",
		"-f", "s16le",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("ffmpeg start: %w", err)
	}

	data, err := io.ReadAll(bufio.NewReader(stdout))
	if err != nil {
		return nil, fmt.Errorf("read wav: %w", err)
	}

	cmd.Wait()

	if len(data) < 8000 {
		return nil, fmt.Errorf("wav too small: %d", len(data))
	}

	return data, nil
}

///////////////////////////////////////////////////////////////////////
// 2) ProcessMedia — только ПТРП, без старого ffmpeg stream
///////////////////////////////////////////////////////////////////////

func (s *MediaService) ProcessMedia(ctx context.Context, sourceURL, mediaType, roomID string) (*models.Media, error) {
	log.Printf("[media] start PTRP: %.60s…", sourceURL)

	// ----------- YOUTUBE RESOLVE -----------
	if strings.Contains(sourceURL, "youtube") || strings.Contains(sourceURL, "youtu.be") {
		log.Printf("[media] youtube detected, resolving…")
		u, err := ResolveYouTube(sourceURL)
		if err != nil {
			return nil, fmt.Errorf("resolve youtube failed: %w", err)
		}
		log.Printf("[media] resolved: %.60s…", u)
		sourceURL = u
	}

	// ----------- MEDIA ROW -----------
	media, err := s.repo.InsertMedia(ctx, &models.Media{
		SourceURL: sourceURL,
		Type:      mediaType,
	})
	if err != nil {
		return nil, err
	}

	///////////////////////////////////////////////////////////////////////
	// 3) Бесконечный цикл: каждые 5 секунд — отрезали — STT — пушим
	///////////////////////////////////////////////////////////////////////

	go func() {
		chunk := 1

		for {
			select {
			case <-ctx.Done():
				log.Printf("[media] ctx done, stop")
				return
			default:
			}

			// 1) WAV
			wav, err := s.capture5secWav(sourceURL)
			if err != nil {
				log.Printf("[media] wav error: %v", err)
				time.Sleep(1 * time.Second)
				continue
			}

			// 2) STT
			txt, err := s.stt.Recognize(ctx, wav)
			if err != nil {
				log.Printf("[media] STT err chunk=%d: %v", chunk, err)
				chunk++
				continue
			}

			txt = strings.TrimSpace(txt)
			if txt == "" {
				log.Printf("[media] empty text chunk=%d", chunk)
				chunk++
				continue
			}

			// 3) save
			_ = s.repo.InsertChunk(ctx, &models.MediaChunk{
				MediaID:     media.ID,
				ChunkNumber: chunk,
				Text:        txt,
			})

			// 4) push
			log.Printf("[media] SEND chunk=%d media=%d: %.40s", chunk, media.ID, txt)

			s.events <- ports.ChunkEvent{
				MediaID:     media.ID,
				ChunkNumber: chunk,
				RoomID:      roomID,
				Text:        txt,
			}

			chunk++
		}
	}()

	return media, nil
}

///////////////////////////////////////////////////////////////////////
// 3) Старый резолвер оставляем как есть
///////////////////////////////////////////////////////////////////////
