package ports

import "context"

type STTService interface {
	Recognize(ctx context.Context, wav []byte) (string, error)
}
