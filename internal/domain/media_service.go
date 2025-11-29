package domain

import (
	"bufio"
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

///////////////////////////////////////////////////////////////////////
// 2) start ffmpeg
///////////////////////////////////////////////////////////////////////

func (s *MediaService) startFFmpeg(ctx context.Context, url string) (*bufio.Reader, *exec.Cmd, error) {
	log.Printf("[media] ffmpeg start: %.60s…", url)

	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-i", url,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-c:a", "libopus",
		"-b:a", "24k",
		"-f", "ogg",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, fmt.Errorf("start ffmpeg: %w", err)
	}

	return bufio.NewReader(stdout), cmd, nil
}

///////////////////////////////////////////////////////////////////////
// 3) read chunk from ffmpeg
///////////////////////////////////////////////////////////////////////

func (s *MediaService) readFromFFmpeg(reader *bufio.Reader) ([]byte, error) {
	tmp := make([]byte, 256*1024)
	n, err := reader.Read(tmp)
	if n > 0 {
		return tmp[:n], nil
	}
	return nil, err
}

///////////////////////////////////////////////////////////////////////
// 4) save chunk + STT (+ roomID)
///////////////////////////////////////////////////////////////////////

func (s *MediaService) saveAndProcessChunk(
	ctx context.Context,
	mediaID int,
	chunkNum int,
	audio []byte,
	roomID string,
) {
	rawSize := len(audio)
	log.Printf("[media] chunk %d raw-bytes=%d", chunkNum, rawSize)

	// показать первые ~40 байт
	prev := 40
	if rawSize < prev {
		prev = rawSize
	}
	log.Printf("[media] chunk %d preview=%v", chunkNum, audio[:prev])

	// сначала пишем в БД с пустым текстом
	chunk := &models.MediaChunk{
		MediaID:     mediaID,
		ChunkNumber: chunkNum,
		Text:        "",
	}
	_ = s.repo.InsertChunk(ctx, chunk)

	// отдельная горутина → STT
	go func(c *models.MediaChunk, data []byte) {
		text, err := s.stt.Recognize(ctx, data)
		if err != nil {
			log.Printf("[media] STT error chunk %d: %v", c.ChunkNumber, err)
			return
		}

		log.Printf("[media] STT text chunk=%d len=%d text=%.40s...", c.ChunkNumber, len(text), text)

		// если пусто — просто не шлём в WebSocket
		if strings.TrimSpace(text) == "" {
			log.Printf("[media] skip empty text chunk=%d", c.ChunkNumber)
			return
		}

		// обновляем текст в БД
		_ = s.repo.UpdateChunkText(ctx, c.ID, text)

		// пушим наружу
		s.events <- ports.ChunkEvent{
			RoomID:      roomID,
			MediaID:     c.MediaID,
			ChunkNumber: c.ChunkNumber,
			Text:        text,
		}

		log.Printf("[SEND] room=%s chunk=%d media=%d text=%.40s...",
			roomID, c.ChunkNumber, c.MediaID, text)
	}(chunk, audio)
}

///////////////////////////////////////////////////////////////////////
// 5) ProcessMedia — принимает roomID
///////////////////////////////////////////////////////////////////////

func (s *MediaService) ProcessMedia(ctx context.Context, sourceURL, mediaType, roomID string) (*models.Media, error) {
	log.Printf("[media] start: %.60s…", sourceURL)

	// youtube
	if strings.Contains(sourceURL, "youtube") || strings.Contains(sourceURL, "youtu.be") {
		log.Printf("[media] youtube detected, resolving…")

		u, err := ResolveYouTube(sourceURL)
		if err != nil {
			return nil, fmt.Errorf("resolve youtube failed: %w", err)
		}

		log.Printf("[media] resolved: %.60s…", u)
		sourceURL = u
	}

	// create media
	media, err := s.repo.InsertMedia(ctx, &models.Media{
		SourceURL: sourceURL,
		Type:      mediaType,
	})
	if err != nil {
		return nil, err
	}

	// start ffmpeg
	reader, cmd, err := s.startFFmpeg(ctx, sourceURL)
	if err != nil {
		return nil, err
	}

	var buf []byte
	chunkNum := 0

	const minChunkSize = 20 * 1024 // 20 KB

	ticker := time.NewTicker(6 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			if len(buf) < minChunkSize {
				log.Printf("[media] skip chunk: too small (%d bytes)", len(buf))
				continue
			}

			// copy exact audio slice
			audioCopy := make([]byte, len(buf))
			copy(audioCopy, buf)

			// reset buffer
			buf = buf[:0]

			chunkNum++
			s.saveAndProcessChunk(ctx, media.ID, chunkNum, audioCopy, roomID)
		}
	}()

	// read ffmpeg
	for {
		frame, err := s.readFromFFmpeg(reader)
		if frame != nil && len(frame) > 0 {
			buf = append(buf, frame...)
		}
		if err != nil {
			log.Printf("[media] ffmpeg read stop: %v", err)
			break
		}
	}

	cmd.Wait()
	log.Printf("[media] finished mediaID=%d", media.ID)
	return media, nil
}
