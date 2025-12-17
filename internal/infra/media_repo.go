package infra

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Vovarama1992/journalist/internal/models"
	"github.com/Vovarama1992/journalist/internal/ports"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresMediaRepo struct {
	pool *pgxpool.Pool
}

func trim(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

func NewPostgresMediaRepo(pool *pgxpool.Pool) ports.MediaRepository {
	return &PostgresMediaRepo{pool: pool}
}

func (r *PostgresMediaRepo) InsertMedia(ctx context.Context, media *models.Media) (*models.Media, error) {
	query := `
		INSERT INTO media (source_url, media_type)
		VALUES ($1, $2)
		RETURNING id, created_at
	`
	row := r.pool.QueryRow(ctx, query, media.SourceURL, media.Type)
	if err := row.Scan(&media.ID, &media.CreatedAt); err != nil {
		return nil, fmt.Errorf("insert media: %w", err)
	}
	return media, nil
}

func (r *PostgresMediaRepo) InsertChunk(ctx context.Context, chunk *models.MediaChunk) error {
	query := `
		INSERT INTO media_chunk (media_id, chunk_number, text)
		VALUES ($1, $2, $3)
		RETURNING id
	`
	row := r.pool.QueryRow(ctx, query, chunk.MediaID, chunk.ChunkNumber, chunk.Text)
	if err := row.Scan(&chunk.ID); err != nil {
		return fmt.Errorf("insert chunk: %w", err)
	}
	return nil
}

func (r *PostgresMediaRepo) UpdateChunkText(ctx context.Context, chunkID int, text string) error {
	query := `
		UPDATE media_chunk
		SET text = $1
		WHERE id = $2
	`
	_, err := r.pool.Exec(ctx, query, text, chunkID)
	return err
}

func (r *PostgresMediaRepo) GetLastChunkNumber(ctx context.Context, mediaID int) (int, error) {
	query := `
		SELECT COALESCE(MAX(chunk_number), 0)
		FROM media_chunk
		WHERE media_id = $1
	`
	var last int
	err := r.pool.QueryRow(ctx, query, mediaID).Scan(&last)
	return last, err
}
func (r *PostgresMediaRepo) GetLastChunk(ctx context.Context, mediaID int) (*models.MediaChunk, error) {
	query := `
		SELECT id, media_id, chunk_number, text
		FROM media_chunk
		WHERE media_id = $1
		  AND text IS NOT NULL
		  AND text <> ''
		ORDER BY chunk_number DESC
		LIMIT 1
	`

	var c models.MediaChunk

	err := r.pool.QueryRow(ctx, query, mediaID).Scan(
		&c.ID,
		&c.MediaID,
		&c.ChunkNumber,
		&c.Text,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get last non-empty chunk: %w", err)
	}

	return &c, nil
}

func (r *PostgresMediaRepo) GetLastCompletedChunk(ctx context.Context, mediaID int) (*models.MediaChunk, error) {
	query := `
        SELECT id, media_id, chunk_number, text
        FROM media_chunk
        WHERE media_id = $1
          AND status = 'done'
        ORDER BY chunk_number DESC
        LIMIT 1
    `

	var c models.MediaChunk

	err := r.pool.QueryRow(ctx, query, mediaID).Scan(
		&c.ID,
		&c.MediaID,
		&c.ChunkNumber,
		&c.Text,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			log.Printf("[DB][PREV] media=%d → no completed chunks", mediaID)
			return nil, nil
		}
		return nil, fmt.Errorf("get last completed chunk: %w", err)
	}

	log.Printf("[DB][PREV] media=%d got chunk=%d text=%q",
		mediaID,
		c.ChunkNumber,
		trim(c.Text, 180),
	)

	return &c, nil
}

func (r *PostgresMediaRepo) GetMediaByID(ctx context.Context, id int) (*models.Media, error) {
	query := `
		SELECT id, source_url, media_type, created_at
		FROM media
		WHERE id = $1
	`

	var m models.Media

	err := r.pool.QueryRow(ctx, query, id).Scan(
		&m.ID,
		&m.SourceURL,
		&m.Type,
		&m.CreatedAt,
	)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("get media by id: %w", err)
	}

	return &m, nil
}

func (r *PostgresMediaRepo) GetMediaHistory(ctx context.Context, mediaID int) (string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT COALESCE(text, '') 
         FROM media_chunk 
         WHERE media_id=$1 
         ORDER BY chunk_number ASC`,
		mediaID,
	)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var sb strings.Builder
	for rows.Next() {
		var txt string
		if err := rows.Scan(&txt); err != nil {
			return "", err
		}
		if txt != "" {
			sb.WriteString(txt)
			sb.WriteString(" ")
		}
	}

	return strings.TrimSpace(sb.String()), nil
}

func (r *PostgresMediaRepo) InsertPendingChunk(
	ctx context.Context,
	mediaID int,
	filePath string,
) (*models.MediaChunk, error) {

	query := `
		INSERT INTO media_chunk (media_id, chunk_number, file_path, status)
		VALUES (
			$1,
			COALESCE((
				SELECT MAX(chunk_number)+1 FROM media_chunk WHERE media_id=$1
			), 1),
			$2,
			'pending'
		)
		RETURNING id, chunk_number
	`

	var c models.MediaChunk
	err := r.pool.QueryRow(ctx, query, mediaID, filePath).Scan(&c.ID, &c.ChunkNumber)
	if err != nil {
		return nil, fmt.Errorf("insert pending chunk: %w", err)
	}

	c.MediaID = mediaID
	c.FilePath = filePath
	c.Status = "pending"
	return &c, nil
}

// ================================================================
// NEW: завершить обработку чанка (STT+GPT)
// ================================================================
func (r *PostgresMediaRepo) CompleteChunk(
	ctx context.Context,
	mediaID int,
	chunkNumber int,
	text string,
) error {

	start := time.Now()
	log.Printf("[DB][COMPLETE][START] media=%d chunk=%d at=%s",
		mediaID, chunkNumber, start.Format(time.RFC3339Nano),
	)

	query := `
		UPDATE media_chunk
		SET text = $1, status = 'done'
		WHERE media_id = $2 AND chunk_number = $3
	`
	_, err := r.pool.Exec(ctx, query, text, mediaID, chunkNumber)
	if err != nil {
		return err
	}

	log.Printf("[DB][COMPLETE][OK] media=%d chunk=%d dur=%s",
		mediaID, chunkNumber, time.Since(start),
	)

	return nil
}
