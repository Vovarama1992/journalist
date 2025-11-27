package models

import "time"

type Media struct {
	ID         int       `db:"id"`
	SourceURL  string    `db:"source_url"`  // исходный URL / технический
	StorageURL *string   `db:"storage_url"` // nullable, URL в реальном хранилище
	Type       string    `db:"media_type"`  // "audio" или "video"
	CreatedAt  time.Time `db:"created_at"`
}
