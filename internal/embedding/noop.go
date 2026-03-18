package embedding

import "context"

// NoopProvider returns zero vectors — used when no embedding provider is configured
type NoopProvider struct {
	dims int
}

func NewNoopProvider(dims int) *NoopProvider {
	if dims <= 0 {
		dims = 384
	}
	return &NoopProvider{dims: dims}
}

func (n *NoopProvider) Name() string       { return "noop" }
func (n *NoopProvider) Dimensions() int    { return n.dims }
func (n *NoopProvider) Healthy(_ context.Context) bool { return true }

func (n *NoopProvider) Embed(_ context.Context, _ string) ([]float32, error) {
	return make([]float32, n.dims), nil
}

func (n *NoopProvider) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = make([]float32, n.dims)
	}
	return result, nil
}
