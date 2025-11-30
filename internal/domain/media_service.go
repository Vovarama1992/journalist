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
	log.Printf("[FFMPEG] start stream url=%s", url)

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

	// ЛОГИРУЕМ ВСЁ STDERR
	go func() {
		buf, _ := io.ReadAll(stderr)
		if len(buf) > 0 {
			log.Printf("[FFMPEG STDERR] %s", buf)
		}
		log.Printf("[FFMPEG] exited")
	}()

	return stdout, nil
}

func (s *MediaService) ProcessMedia(ctx context.Context, sourceURL, mediaType, roomID string) (*models.Media, error) {
	log.Printf("[MEDIA] PTRP start url=%s", sourceURL)

	media, err := s.repo.InsertMedia(ctx, &models.Media{
		SourceURL: sourceURL,
		Type:      mediaType,
	})
	if err != nil {
		return nil, err
	}

	stream, err := s.continuousCapture(ctx, sourceURL)
	if err != nil {
		return nil, fmt.Errorf("capture error: %w", err)
	}

	go func() {
		defer func() {
			log.Printf("[MEDIA] closing stream")
			stream.Close()
		}()

		const chunkBytes = 160000
		buf := make([]byte, chunkBytes)
		chunkNum := 1

		for {
			select {
			case <-ctx.Done():
				log.Printf("[MEDIA] ctx.Done stop PTRP")
				return

			default:
				log.Printf("[MEDIA] reading chunk=%d", chunkNum)

				n, err := io.ReadFull(stream, buf)
				if err != nil {
					log.Printf("[MEDIA] readFull err: %v", err)
					return
				}

				if n != chunkBytes {
					log.Printf("[MEDIA] WARNING short chunk read: %d/%d", n, chunkBytes)
				}

				log.Printf("[MEDIA] chunk=%d read OK, sending to STT", chunkNum)

				// CALL STT
				txt, err := s.stt.Recognize(ctx, buf)
				if err != nil {
					log.Printf("[MEDIA] STT err chunk=%d: %v", chunkNum, err)
					chunkNum++
					continue
				}

				log.Printf("[MEDIA] STT result chunk=%d: %.50s", chunkNum, txt)

				txt = strings.TrimSpace(txt)
				if txt == "" {
					log.Printf("[MEDIA] empty text chunk=%d", chunkNum)
					chunkNum++
					continue
				}

				err = s.repo.InsertChunk(ctx, &models.MediaChunk{
					MediaID:     media.ID,
					ChunkNumber: chunkNum,
					Text:        txt,
				})
				if err != nil {
					log.Printf("[MEDIA] DB insert err chunk=%d: %v", chunkNum, err)
				}

				log.Printf("[MEDIA] SEND chunk=%d text=%s", chunkNum, txt)

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
