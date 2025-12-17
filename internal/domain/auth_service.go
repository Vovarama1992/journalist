package domain

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"

	"github.com/Vovarama1992/journalist/internal/ports"
	"github.com/jackc/pgx/v5/pgxpool"
)

type authService struct {
	pool   *pgxpool.Pool
	secret string
}

func NewAuthService(pool *pgxpool.Pool, secret string) ports.AuthService {
	return &authService{
		pool:   pool,
		secret: secret,
	}
}

func (s *authService) Login(ctx context.Context, password string) (string, error) {
	var realPass string

	err := s.pool.QueryRow(ctx,
		`SELECT password FROM journal_auth LIMIT 1`,
	).Scan(&realPass)

	if err != nil {
		return "", err
	}

	if password != realPass {
		return "", errors.New("invalid password")
	}

	token := s.sign("allowed")
	return token, nil
}

func (s *authService) ValidateToken(ctx context.Context, token string) (bool, error) {
	valid := s.sign("allowed")
	return token == valid, nil
}

func (s *authService) sign(msg string) string {
	h := hmac.New(sha256.New, []byte(s.secret))
	h.Write([]byte(msg))
	return hex.EncodeToString(h.Sum(nil))
}
