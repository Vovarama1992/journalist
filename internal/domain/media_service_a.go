package domain

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/Vovarama1992/journalist/internal/domain/stations"
	"github.com/Vovarama1992/journalist/internal/models"
	"github.com/Vovarama1992/journalist/internal/ports"
)

type MediaService struct {
	repo ports.MediaRepository

	s1 *stations.S1ResolveURL
	s2 *stations.S2GrabPCM
	s3 *stations.S3PCMtoWAV
	s4 *stations.S4WAVtoText
	s5 *stations.S5GPT

	logger *log.Logger

	mu      sync.Mutex
	mediaID int
	roomID  string
	events  chan ports.ChunkEvent
}

func NewMediaService(
	repo ports.MediaRepository,
	s1 *stations.S1ResolveURL,
	s2 *stations.S2GrabPCM,
	s3 *stations.S3PCMtoWAV,
	s4 *stations.S4WAVtoText,
	gpt ports.GPTService,
) *MediaService {
	return &MediaService{
		repo:   repo,
		s1:     s1,
		s2:     s2,
		s3:     s3,
		s4:     s4,
		s5:     stations.NewS5GPT(gpt),
		events: make(chan ports.ChunkEvent, 100),
	}
}

func (m *MediaService) Events() <-chan ports.ChunkEvent { return m.events }

// ========================================================================
// PROCESS
// ========================================================================
func (m *MediaService) Process(
	ctx context.Context,
	srcURL string,
	roomID string,
	mediaID int,
) (*models.Media, error) {

	m.roomID = roomID

	var media *models.Media
	var err error

	if mediaID > 0 {
		media, err = m.repo.GetMediaByID(ctx, mediaID)
		if err != nil {
			return nil, err
		}
		if media == nil {
			return nil, fmt.Errorf("media not found")
		}
		m.mediaID = media.ID
		srcURL = media.SourceURL
	} else {
		media, err = m.repo.InsertMedia(ctx, &models.Media{
			SourceURL: srcURL,
			Type:      "audio",
		})
		if err != nil {
			return nil, err
		}
		m.mediaID = media.ID
	}

	// ---------- LOGGER ----------
	logDir := "/app/logs"
	_ = os.MkdirAll(logDir, 0755)

	logName := fmt.Sprintf(
		"media_%d_room_%s_%s.log",
		m.mediaID,
		m.roomID,
		time.Now().Format("2006-01-02T15-04-05"),
	)
	logPath := filepath.Join(logDir, logName)

	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	m.logger = log.New(io.MultiWriter(os.Stdout, f), "", log.LstdFlags|log.Lmicroseconds)
	// ----------------------------

	m.logger.Printf("[START] media=%d room=%s", m.mediaID, m.roomID)

	go m.ingestLoop(ctx, srcURL)
	return media, nil
}

// ========================================================================
// LOOP
// ========================================================================
func (m *MediaService) ingestLoop(ctx context.Context, srcURL string) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Printf("[INGEST-LOOP][STOP] media=%d", m.mediaID)
			return
		case <-ticker.C:
			go m.ingestOne(ctx, srcURL)
		}
	}
}

// ========================================================================
// ONE INGEST (БЕЗ СМЕЩЕНИЯ)
// ========================================================================
func (m *MediaService) ingestOne(ctx context.Context, srcURL string) {
	start := time.Now()
	m.logger.Printf("[INGEST][START] media=%d", m.mediaID)

	audioURL, err := m.s1.Run(ctx, srcURL)
	if err != nil || audioURL == "" {
		m.logger.Printf("[S1][FAIL] media=%d err=%v", m.mediaID, err)
		return
	}

	pcm, err := m.s2.Run(ctx, audioURL)
	if err != nil || len(pcm) == 0 {
		m.logger.Printf("[S2][FAIL] media=%d err=%v", m.mediaID, err)
		return
	}

	chunkID, filePath, err := m.createPendingChunk(ctx, pcm)
	if err != nil {
		m.logger.Printf("[PENDING][FAIL] media=%d err=%v", m.mediaID, err)
		return
	}

	defer func() {
		_ = m.repo.CompleteChunk(ctx, m.mediaID, chunkID, "")
	}()

	wav := m.s3.Run(pcm)

	raw, err := m.s4.Run(ctx, wav)
	if err != nil || raw == "" {
		m.logger.Printf("[S4][FAIL] media=%d chunk=%d err=%v", m.mediaID, chunkID, err)
		return
	}

	// GPT БЕЗ prevText
	proc, err := m.s5.Run(ctx, "", raw)
	if err != nil || proc == "" {
		m.logger.Printf("[S5][SKIP] media=%d chunk=%d err=%v", m.mediaID, chunkID, err)
		return
	}

	if err := m.repo.CompleteChunk(ctx, m.mediaID, chunkID, proc); err != nil {
		m.logger.Printf("[DB][FAIL] media=%d chunk=%d err=%v", m.mediaID, chunkID, err)
		return
	}

	_ = os.Remove(filePath)

	m.events <- ports.ChunkEvent{
		MediaID:     m.mediaID,
		ChunkNumber: chunkID,
		RoomID:      m.roomID,
		Text:        proc,
	}

	m.logger.Printf("[DONE] media=%d chunk=%d dur=%s",
		m.mediaID, chunkID, time.Since(start))
}

// ========================================================================
// CREATE PENDING
// ========================================================================
func (m *MediaService) createPendingChunk(ctx context.Context, pcm []byte) (int, string, error) {
	dir := fmt.Sprintf("/tmp/journalist/media_%d", m.mediaID)
	_ = os.MkdirAll(dir, 0755)

	filename := fmt.Sprintf("chunk_%d.pcm", time.Now().UnixNano())
	path := filepath.Join(dir, filename)

	if err := os.WriteFile(path, pcm, 0644); err != nil {
		return 0, "", err
	}

	chunk, err := m.repo.InsertPendingChunk(ctx, m.mediaID, path)
	if err != nil {
		return 0, "", err
	}

	m.logger.Printf("[PENDING] media=%d chunk=%d", m.mediaID, chunk.ChunkNumber)
	return chunk.ChunkNumber, path, nil
}
