package domain

import (
	"context"
	"fmt"
	"io"
	"log"
	"os/exec"
	"strings"

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

func (s *MediaService) continuousCapture(ctx context.Context, url string) (io.ReadCloser, error) {
	log.Printf("[stream] continuousCapture START url=%.80s", url)

	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-re",
		"-seekable", "0",
		"-i", url,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-f", "s16le",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("ffmpeg start: %w", err)
	}

	// логируем stderr в фоне
	go func() {
		errLog, _ := io.ReadAll(stderr)
		if len(errLog) > 0 {
			log.Printf("[stream] ffmpeg stderr: %s", errLog)
		}
		log.Printf("[stream] ffmpeg finished")
	}()

	return stdout, nil
}

// ProcessMedia — запускает поток ffmpeg → режет каждые 5 секунд → STT → WS.

func (s *MediaService) ProcessMedia(ctx context.Context, sourceURL, mediaType, roomID string) (*models.Media, error) {
	log.Printf("[media] start PTRP: %.60s…", sourceURL)

	// создаём запись media
	media, err := s.repo.InsertMedia(ctx, &models.Media{
		SourceURL: sourceURL,
		Type:      mediaType,
	})
	if err != nil {
		return nil, err
	}

	// единственный ffmpeg-поток (универсальный)
	stream, err := s.continuousCapture(ctx, sourceURL)
	if err != nil {
		return nil, fmt.Errorf("continuousCapture: %w", err)
	}

	go func() {
		defer stream.Close()

		const chunkBytes = 160000 // 5 сек PCM
		buf := make([]byte, chunkBytes)
		chunkNum := 1

		for {
			select {
			case <-ctx.Done():
				log.Printf("[media] ctx done — stop PTRP")
				return

			default:
				// читаем ровно 5 сек
				n, err := io.ReadFull(stream, buf)
				if err != nil {
					log.Printf("[media] readFull err: %v", err)
					return
				}
				if n < chunkBytes {
					log.Printf("[media] short read %d bytes", n)
					continue
				}

				// STT
				txt, err := s.stt.Recognize(ctx, buf)
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

				// сохраняем в БД
				_ = s.repo.InsertChunk(ctx, &models.MediaChunk{
					MediaID:     media.ID,
					ChunkNumber: chunkNum,
					Text:        txt,
				})

				log.Printf("[media] SEND chunk=%d media=%d: %.40s", chunkNum, media.ID, txt)

				// отправляем в WS
				s.events <- ports.ChunkEvent{
					MediaID:     media.ID,
					ChunkNumber: chunkNum,
					RoomID:      roomID,
					Text:        txt,
				}

				chunkNum++
			}
		}
	}()

	return media, nil
}
