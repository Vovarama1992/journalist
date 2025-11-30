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

type AgressiveMediaService struct {
	repo   ports.MediaRepository
	stt    ports.STTService
	events chan ports.ChunkEvent
}

func NewAgressiveMediaService(repo ports.MediaRepository, stt ports.STTService) *AgressiveMediaService {
	return &AgressiveMediaService{
		repo:   repo,
		stt:    stt,
		events: make(chan ports.ChunkEvent, 100),
	}
}

func (s *AgressiveMediaService) Events() <-chan ports.ChunkEvent {
	return s.events
}

////////////////////////////////////////////////////////////////////////////////
// СТАНЦИЯ 0 — RESOLVE
////////////////////////////////////////////////////////////////////////////////

func (s *AgressiveMediaService) RESOLVE(ctx context.Context, raw string) (string, error) {
	log.Printf("[RESOLVE][IN] raw=%s", raw)

	if strings.Contains(raw, "youtube.com") || strings.Contains(raw, "youtu.be") {

		extract := func(format string) (string, error) {
			out, err := exec.CommandContext(
				ctx,
				"yt-dlp",
				"-f", format,
				"--extractor-args", "youtube:player_client=default",
				"--no-playlist",
				"-g",
				raw,
			).CombinedOutput()

			log.Printf("[RESOLVE][INSIDE] fmt=%s out=%q err=%v", format, out, err)

			if err != nil {
				return "", err
			}

			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			last := strings.TrimSpace(lines[len(lines)-1])

			if !strings.HasPrefix(last, "http") {
				return "", fmt.Errorf("invalid url line: %q", last)
			}
			return last, nil
		}

		if url, err := extract("bestaudio"); err == nil {
			log.Printf("[RESOLVE][OUT] bestaudio=%s", url)
			return url, nil
		}

		if url, err := extract("best"); err == nil {
			log.Printf("[RESOLVE][OUT] best=%s", url)
			return url, nil
		}

		return "", fmt.Errorf("yt-dlp failed for all formats")
	}

	log.Printf("[RESOLVE][OUT] passthrough=%s", raw)
	return raw, nil
}

////////////////////////////////////////////////////////////////////////////////
// СТАНЦИЯ 1 — PCM_ONESHOT (подключение ffmpeg на 5 секунд)
////////////////////////////////////////////////////////////////////////////////

func (s *AgressiveMediaService) PCM_ONESHOT(ctx context.Context, url string) ([]byte, error) {
	log.Printf("[PCM_ONESHOT][IN] url=%s", url)

	cmd := exec.CommandContext(ctx,
		"ffmpeg",
		"-hide_banner",
		"-loglevel", "error",
		"-y",
		"-ss", "0",
		"-t", "5",
		"-i", url,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-f", "s16le",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("pipe: %w", err)
	}

	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start: %w", err)
	}

	// INSIDE
	go func() {
		errbuf, _ := io.ReadAll(stderr)
		if len(errbuf) > 0 {
			log.Printf("[PCM_ONESHOT][INSIDE] ffmpeg stderr: %s", errbuf)
		}
	}()

	const need = 160000
	buf := make([]byte, need)

	n, err := io.ReadFull(stdout, buf)
	log.Printf("[PCM_ONESHOT][INSIDE] n=%d err=%v", n, err)

	cmd.Wait()

	if err != nil {
		return nil, fmt.Errorf("read: %w", err)
	}

	log.Printf("[PCM_ONESHOT][OUT] ok len=%d", len(buf))
	return buf, nil
}

////////////////////////////////////////////////////////////////////////////////
// СТАНЦИЯ 2 — WAV (заглушка)
////////////////////////////////////////////////////////////////////////////////

func (s *AgressiveMediaService) WAV(pcm []byte) []byte {
	log.Printf("[WAV][IN] pcm_len=%d", len(pcm))
	out := pcm
	log.Printf("[WAV][OUT] wav_len=%d", len(out))
	return out
}

////////////////////////////////////////////////////////////////////////////////
// СТАНЦИЯ 3 — STT
////////////////////////////////////////////////////////////////////////////////

func (s *AgressiveMediaService) STT(ctx context.Context, wav []byte) (string, error) {
	log.Printf("[STT][IN] wav_len=%d", len(wav))
	txt, err := s.stt.Recognize(ctx, wav)
	log.Printf("[STT][OUT] txt=%.40s err=%v", txt, err)
	return txt, err
}

////////////////////////////////////////////////////////////////////////////////
// ОРКЕСТР
////////////////////////////////////////////////////////////////////////////////

func (s *AgressiveMediaService) Process(ctx context.Context, url, roomID string) (*models.Media, error) {

	media, err := s.repo.InsertMedia(ctx, &models.Media{
		SourceURL: url,
		Type:      "audio",
	})
	if err != nil {
		return nil, err
	}

	// RESOLVE
	resolved, err := s.RESOLVE(ctx, url)
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
				// СТАНЦИЯ PCM_ONESHOT
				pcm, err := s.PCM_ONESHOT(ctx, resolved)
				if err != nil {
					log.Printf("[ORCH][ERR] PCM_ONESHOT: %v", err)
					return
				}

				// WAV
				wav := s.WAV(pcm)

				// STT
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
