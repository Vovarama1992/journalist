package domain

import (
	"context"
	"fmt"
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

	mu             sync.Mutex
	currentChunkID int
	pending        map[int]string // chunkID → filePath
	mediaID        int
	roomID         string
	events         chan ports.ChunkEvent
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
		repo:    repo,
		s1:      s1,
		s2:      s2,
		s3:      s3,
		s4:      s4,
		s5:      stations.NewS5GPT(gpt),
		pending: map[int]string{},
		events:  make(chan ports.ChunkEvent, 100),
	}
}

func (m *MediaService) Events() <-chan ports.ChunkEvent { return m.events }

// =====================================================
// Старт обработки медиа
// =====================================================
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
		srcURL = media.SourceURL
	} else {
		media, err = m.repo.InsertMedia(ctx, &models.Media{
			SourceURL: srcURL,
			Type:      "audio",
		})
		if err != nil {
			return nil, err
		}
	}
	m.mediaID = media.ID

	last, _ := m.repo.GetLastChunk(ctx, media.ID)
	if last != nil {
		m.currentChunkID = last.ChunkNumber + 1
	} else {
		m.currentChunkID = 1
	}

	go m.ingestLoop(ctx, srcURL)

	return media, nil
}

// =====================================================
// IN GEST LOOP — каждые 8 сек режем 15 сек PCM
// =====================================================
func (m *MediaService) ingestLoop(ctx context.Context, srcURL string) {
	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			go m.ingestOne(ctx, srcURL)
		}
	}
}

// =====================================================
// Один цикл инжеста: 15 сек PCM + запись в БД + ожидание очереди
// =====================================================
func (m *MediaService) ingestOne(ctx context.Context, srcURL string) {
	// --- S1 ---
	audioURL, err := m.s1.Run(ctx, srcURL)
	if err != nil {
		log.Printf("[INGEST] S1 err=%v", err)
		return
	}

	// --- S2 (15 сек PCM) ---
	pcm, err := m.s2.Run(ctx, audioURL)
	if err != nil || len(pcm) == 0 {
		log.Printf("[INGEST] S2 err=%v", err)
		return
	}

	// --- save PCM file ---
	dir := fmt.Sprintf("/tmp/journalist/media_%d", m.mediaID)
	_ = os.MkdirAll(dir, 0755)

	// chunkID — фиксируется при создании pending-записи
	chunkID, filePath, err := m.createPendingChunk(ctx, pcm)
	if err != nil {
		log.Printf("[INGEST] pending chunk create failed: %v", err)
		return
	}

	// --- Wait until this chunk is current ---
	for {
		m.mu.Lock()
		ok := (chunkID == m.currentChunkID)
		m.mu.Unlock()

		if ok {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// --- PROCESS S3 / S4 / S5 ---
	wav := m.s3.Run(pcm)

	raw, err := m.s4.Run(ctx, wav)
	if err != nil {
		log.Printf("[INGEST] S4 err=%v", err)
		return
	}

	prevChunk, _ := m.repo.GetLastChunk(ctx, m.mediaID)
	prevText := ""
	if prevChunk != nil {
		prevText = prevChunk.Text
	}

	proc, err := m.s5.Run(ctx, prevText, raw)
	if err != nil || proc == "" {
		proc = raw
	}

	// --- SAVE RESULT ---
	err = m.repo.CompleteChunk(ctx, chunkID, proc)
	if err != nil {
		log.Printf("[INGEST] save result err=%v", err)
		return
	}

	// delete PCM file
	_ = os.Remove(filePath)

	// EVENTS for WS
	m.events <- ports.ChunkEvent{
		MediaID:     m.mediaID,
		ChunkNumber: chunkID,
		RoomID:      m.roomID,
		Text:        proc,
	}

	// move current pointer
	m.mu.Lock()
	m.currentChunkID++
	m.mu.Unlock()
}

// =====================================================
// Создание pending-чанка в БД + запись PCM в файл
// =====================================================
func (m *MediaService) createPendingChunk(ctx context.Context, pcm []byte) (int, string, error) {

	dir := fmt.Sprintf("/tmp/journalist/media_%d", m.mediaID)
	_ = os.MkdirAll(dir, 0755)

	// file path
	filename := fmt.Sprintf("chunk_%d.pcm", time.Now().UnixNano())
	path := filepath.Join(dir, filename)

	err := os.WriteFile(path, pcm, 0644)
	if err != nil {
		return 0, "", err
	}

	// write DB row (pending)
	chunk, err := m.repo.InsertPendingChunk(ctx, m.mediaID, path)
	if err != nil {
		return 0, "", err
	}

	return chunk.ID, path, nil
}
