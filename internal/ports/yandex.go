package ports

import "context"

type STTService interface {
	Recognize(ctx context.Context, wav []byte) (text string, raw []byte, err error)
}
