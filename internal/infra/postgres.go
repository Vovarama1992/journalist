package infra

import (
	"context"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

func NewPgxPool(ctx context.Context) (*pgxpool.Pool, error) {
	dsn := os.Getenv("DATABASE_URL")
	return pgxpool.New(ctx, dsn)
}
