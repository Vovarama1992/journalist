package infra

import (
	"context"
	"fmt"

	"github.com/Vovarama1992/journalist/internal/models"
	"github.com/Vovarama1992/journalist/internal/ports"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PostgresMediaRepo struct {
	pool *pgxpool.Pool
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
