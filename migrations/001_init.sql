-- ====================================
-- MIGRATION 001 — INITIAL STRUCTURE
-- ====================================

-- Хранилище пароля для доступа к сервису
CREATE TABLE IF NOT EXISTS journal_auth (
    id SERIAL PRIMARY KEY,
    password TEXT NOT NULL
);

-- Вставляем дефолтный пароль (замени при необходимости)
INSERT INTO journal_auth (password)
VALUES ('rossaprimavera');