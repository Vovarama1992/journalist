package ports

import "context"

type AuthService interface {
	Login(ctx context.Context, password string) (string, error)
	ValidateToken(ctx context.Context, token string) (bool, error)
}
