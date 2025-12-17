package models

type MediaChunk struct {
	ID          int    `db:"id"`
	MediaID     int    `db:"media_id"`
	ChunkNumber int    `db:"chunk_number"`
	Text        string `db:"text"`
	FilePath    string `db:"file_path"`
	Status      string `db:"status"`
}
