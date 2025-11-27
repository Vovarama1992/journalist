package models

import "time"

type MediaChunk struct {
	ID          int       `db:"id"`
	MediaID     int       `db:"media_id"`
	ChunkNumber int       `db:"chunk_number"`
	Data        []byte    `db:"data"`
	CreatedAt   time.Time `db:"created_at"`
}
