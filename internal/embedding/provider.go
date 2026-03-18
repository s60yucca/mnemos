package embedding

import "context"

// IEmbeddingProvider defines the interface for generating embeddings
type IEmbeddingProvider interface {
	Name() string
	Dimensions() int
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Healthy(ctx context.Context) bool
}
