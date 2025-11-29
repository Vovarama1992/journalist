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
// 1) create media
///////////////////////////////////////////////////////////////////////

func (s *MediaService) createMedia(ctx context.Context, src, typ string) (*models.Media, error) {
	return s.repo.InsertMedia(ctx, &models.Media{
		SourceURL: src,
		Type:      typ,
	})
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
// 4) save chunk + STT
///////////////////////////////////////////////////////////////////////

func (s *MediaService) saveAndProcessChunk(ctx context.Context, mediaID int, chunkNum int, audio []byte) {
	log.Printf("[media] chunk %d save (%d bytes)", chunkNum, len(audio))

	chunk := &models.MediaChunk{
		MediaID:     mediaID,
		ChunkNumber: chunkNum,
		Text:        "",
	}

	_ = s.repo.InsertChunk(ctx, chunk)

	go func(c *models.MediaChunk, data []byte) {
		text, err := s.stt.Recognize(ctx, data)
		if err != nil {
			log.Printf("[media] STT error chunk %d: %v", c.ChunkNumber, err)
			return
		}

		_ = s.repo.UpdateChunkText(ctx, c.ID, text)

		s.events <- ports.ChunkEvent{
			MediaID:     c.MediaID,
			ChunkNumber: c.ChunkNumber,
			Text:        text,
		}

		log.Printf("[media] chunk %d text ready", c.ChunkNumber)
	}(chunk, audio)
}

///////////////////////////////////////////////////////////////////////
// 5) ProcessMedia — FINAL FIXED VERSION
///////////////////////////////////////////////////////////////////////

func (s *MediaService) ProcessMedia(ctx context.Context, sourceURL, mediaType string) (*models.Media, error) {
	log.Printf("[media] start: %.60s…", sourceURL)

	// resolve youtube
	if strings.Contains(sourceURL, "youtube") || strings.Contains(sourceURL, "youtu.be") {
		log.Printf("[media] youtube detected, resolving…")

		u, err := ResolveYouTube(sourceURL)
		if err != nil {
			return nil, fmt.Errorf("resolve youtube failed: %w", err)
		}

		log.Printf("[media] resolved: %.60s…", u)
		sourceURL = u
	}

	// create media row
	media, err := s.createMedia(ctx, sourceURL, mediaType)
	if err != nil {
		return nil, err
	}

	// ffmpeg pipe
	reader, cmd, err := s.startFFmpeg(ctx, sourceURL)
	if err != nil {
		return nil, err
	}

	// accumulator (ONLY for current chunk)
	var buf []byte
	chunkNum := 0

	// chunk timing: ~6 seconds
	const chunkInterval = 6 * time.Second
	ticker := time.NewTicker(chunkInterval)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			if len(buf) == 0 {
				continue
			}

			// FINAL COPY (no shared slice!)
			audioCopy := make([]byte, len(buf))
			copy(audioCopy, buf)

			buf = buf[:0] // zero but keep capacity

			chunkNum++
			s.saveAndProcessChunk(ctx, media.ID, chunkNum, audioCopy)
		}
	}()

	// read ffmpeg stream
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
