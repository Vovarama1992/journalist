package ports

import "context"

type GPTService interface {
	ProcessChunk(
		ctx context.Context,
		lastChunk string,
		newChunk string,
	) (string, error)
}
