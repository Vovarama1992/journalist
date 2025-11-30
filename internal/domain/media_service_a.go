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

type NewMediaService struct {
	repo   ports.MediaRepository
	stt    ports.STTService
	events chan ports.ChunkEvent
}

func NewNewMediaService(repo ports.MediaRepository, stt ports.STTService) *NewMediaService {
	return &NewMediaService{
		repo:   repo,
		stt:    stt,
		events: make(chan ports.ChunkEvent, 100),
	}
}

func (s *NewMediaService) Events() <-chan ports.ChunkEvent {
	return s.events
}

// СТАНЦИЯ 1 — PCM_START_ONE_CHUNK
func (s *NewMediaService) PCM_ONE(ctx context.Context, url string) ([]byte, error) {
	log.Printf("[PCM_ONE][IN] url=%s", url)

	cmd := exec.CommandContext(ctx,
		"ffmpeg", "-re", "-seekable", "0",
		"-i", url,
		"-vn", "-ac", "1", "-ar", "16000",
		"-t", "5",
		"-f", "s16le", "pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout: %w", err)
	}
	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}

	go func() {
		b, _ := io.ReadAll(stderr)
		if len(b) > 0 {
			log.Printf("[PCM_ONE][INSIDE] ffmpeg stderr=%s", b)
		}
	}()

	out, err := io.ReadAll(stdout)
	log.Printf("[PCM_ONE][OUT] pcm_len=%d err=%v", len(out), err)

	return out, nil
}

// СТАНЦИЯ 2 — WAV
func (s *NewMediaService) WAV(p []byte) []byte {
	log.Printf("[WAV][IN] len=%d", len(p))
	log.Printf("[WAV][OUT] len=%d", len(p))
	return p
}

// СТАНЦИЯ 3 — STT
func (s *NewMediaService) STT(ctx context.Context, wav []byte) (string, error) {
	log.Printf("[STT][IN] wav_len=%d", len(wav))
	txt, err := s.stt.Recognize(ctx, wav)
	log.Printf("[STT][OUT] txt=%.40s err=%v", txt, err)
	return txt, err
}

func (s *NewMediaService) Process(ctx context.Context, url, roomID string) (*models.Media, error) {

	media, err := s.repo.InsertMedia(ctx, &models.Media{SourceURL: url, Type: "audio"})
	if err != nil {
		return nil, err
	}

	go func() {
		chunk := 1

		for {
			select {
			case <-ctx.Done():
				return

			default:
				pcm, err := s.PCM_ONE(ctx, url)
				if err != nil {
					log.Printf("[ORCH_NEW][ERR] PCM_ONE: %v", err)
					return
				}

				wav := s.WAV(pcm)

				txt, err := s.STT(ctx, wav)
				if err != nil {
					chunk++
					continue
				}

				txt = strings.TrimSpace(txt)
				if txt == "" {
					chunk++
					continue
				}

				s.repo.InsertChunk(ctx, &models.MediaChunk{
					MediaID:     media.ID,
					ChunkNumber: chunk,
					Text:        txt,
				})

				s.events <- ports.ChunkEvent{
					MediaID:     media.ID,
					ChunkNumber: chunk,
					RoomID:      roomID,
					Text:        txt,
				}

				chunk++
			}
		}
	}()

	return media, nil
}
