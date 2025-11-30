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

// --------------------------
// СТАНЦИЯ 0 — RESOLVE URL
// --------------------------
func (s *MediaService) station0_resolveURL(ctx context.Context, src string) (string, error) {
	log.Printf("[S0 IN]     url=%s", src)

	// 1) если НЕ youtube — возвращаем как есть
	if !strings.Contains(src, "youtube.com") && !strings.Contains(src, "youtu.be") {
		log.Printf("[S0 STEP]   non-youtube url → pass directly")
		log.Printf("[S0 OUT]    direct=%s", src)
		return src, nil
	}

	// 2) youtube → yt-dlp bestaudio
	log.Printf("[S0 STEP]   youtube detected → yt-dlp bestaudio")

	const yt = "/usr/local/bin/yt-dlp"
	args := []string{
		"-f", "bestaudio/best", // универсально
		"--no-playlist",
		"-g",
		src,
	}

	out, err := exec.CommandContext(ctx, yt, args...).CombinedOutput()
	res := strings.TrimSpace(string(out))

	if err != nil {
		log.Printf("[S0 ERR]    yt-dlp: %v (%s)", err, res)
		return "", fmt.Errorf("resolve youtube: %w", err)
	}

	if res == "" || !strings.HasPrefix(res, "http") {
		log.Printf("[S0 ERR]    invalid result: %q", res)
		return "", fmt.Errorf("resolve: invalid output")
	}

	log.Printf("[S0 OUT]    direct=%s", res)
	return res, nil
}

// --------------------------
// СТАНЦИЯ 1 — START PCM STREAM
// --------------------------
func (s *MediaService) station1_startPCM(ctx context.Context, directURL string) (io.ReadCloser, error) {
	log.Printf("[S1 IN]     direct=%s", directURL)
	log.Printf("[S1 STEP]   spawning ffmpeg")

	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-re",
		"-seekable", "0",
		"-i", directURL,
		"-vn",
		"-ac", "1",
		"-ar", "16000",
		"-f", "s16le",
		"pipe:1",
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Printf("[S1 ERR]    %v", err)
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}

	stderr, _ := cmd.StderrPipe()

	if err := cmd.Start(); err != nil {
		log.Printf("[S1 ERR]    %v", err)
		return nil, fmt.Errorf("ffmpeg start: %w", err)
	}

	go func() {
		slurp, _ := io.ReadAll(stderr)
		if len(slurp) > 0 {
			log.Printf("[S1 STEP]   ffmpeg-stderr: %s", slurp)
		}
		log.Printf("[S1 STEP]   ffmpeg exited")
	}()

	log.Printf("[S1 OUT]    pcm_stream_ready")
	return stdout, nil
}

// --------------------------
// СТАНЦИЯ 2 — READ PCM CHUNK
// --------------------------
func (s *MediaService) station2_readPCM(stream io.Reader, bytes int) ([]byte, error) {
	log.Printf("[S2 IN]     want=%d", bytes)
	log.Printf("[S2 STEP]   reading…")

	buf := make([]byte, bytes)
	n, err := io.ReadFull(stream, buf)
	if err != nil {
		log.Printf("[S2 ERR]    %v", err)
		return nil, err
	}

	if n != bytes {
		log.Printf("[S2 ERR]    short read: %d/%d", n, bytes)
		return nil, fmt.Errorf("short read")
	}

	log.Printf("[S2 OUT]    chunk_ok (%d bytes)", n)
	return buf, nil
}

// --------------------------
// СТАНЦИЯ 3 — STT
// --------------------------
func (s *MediaService) station3_stt(ctx context.Context, pcm []byte) (string, error) {
	log.Printf("[S3 IN]     pcm=%d bytes", len(pcm))
	log.Printf("[S3 STEP]   calling STT")

	txt, err := s.stt.Recognize(ctx, pcm)
	if err != nil {
		log.Printf("[S3 ERR]    %v", err)
		return "", err
	}

	log.Printf("[S3 STEP]   raw=%.50s", txt)
	log.Printf("[S3 OUT]    text_ok")

	return txt, nil
}

// --------------------------
// ОРКЕСТР
// --------------------------
func (s *MediaService) ProcessMedia(ctx context.Context, sourceURL, mediaType, roomID string) (*models.Media, error) {

	media, err := s.repo.InsertMedia(ctx, &models.Media{
		SourceURL: sourceURL,
		Type:      mediaType,
	})
	if err != nil {
		return nil, err
	}

	// S0
	directURL, err := s.station0_resolveURL(ctx, sourceURL)
	if err != nil {
		return nil, err
	}

	// S1
	stream, err := s.station1_startPCM(ctx, directURL)
	if err != nil {
		return nil, err
	}

	go func() {
		defer stream.Close()

		const chunkBytes = 160000
		chunkNum := 1

		for {
			select {
			case <-ctx.Done():
				log.Printf("[CTX] stop")
				return

			default:
				// S2
				pcm, err := s.station2_readPCM(stream, chunkBytes)
				if err != nil {
					return
				}

				// S3
				txt, err := s.station3_stt(ctx, pcm)
				if err != nil {
					chunkNum++
					continue
				}

				txt = strings.TrimSpace(txt)
				if txt == "" {
					chunkNum++
					continue
				}

				s.repo.InsertChunk(ctx, &models.MediaChunk{
					MediaID:     media.ID,
					ChunkNumber: chunkNum,
					Text:        txt,
				})

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
