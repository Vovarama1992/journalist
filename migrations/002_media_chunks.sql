CREATE TABLE media (
    id SERIAL PRIMARY KEY,
    source_url TEXT NOT NULL,           -- исходный URL / технический
    storage_url TEXT,                   -- nullable, URL в реальном хранилище
    media_type VARCHAR(16) NOT NULL,    -- "audio" или "video"
    created_at TIMESTAMP WITH TIME ZONE DEFAULT now()
);

CREATE TABLE media_chunk (
    id SERIAL PRIMARY KEY,
    media_id INT NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    chunk_number INT NOT NULL,
    text TEXT,     -- сам смысловой чанк
    UNIQUE(media_id, chunk_number)
);

ALTER TABLE media_chunk
    ADD COLUMN status VARCHAR(16) NOT NULL DEFAULT 'pending',
    ADD COLUMN file_path TEXT,
    ADD COLUMN created_at TIMESTAMPTZ DEFAULT now();