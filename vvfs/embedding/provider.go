package embedding

import (
	"context"
	"strings"
)

// Provider produces fixed-dimension embeddings from input strings
type Provider interface {
	Dimensions() int
	Embed(ctx context.Context, inputs []string) ([][]float32, error)
}

// NewProviderLegacy returns a provider by dims/modelPath (legacy signature)
func NewProviderLegacy(dims int, modelPath string) Provider {
	return NewProvider("hash", dims, modelPath)
}

// NewProvider selects an embedding provider by name (e.g., "hash", "nomic-v2", "onnx").
// modelPath is reserved for future local model loading.
// Unknown providers fall back to a deterministic hash-based embedder.
func NewProvider(providerName string, dims int, modelPath string) Provider {
	if dims <= 0 {
		dims = 384
	}
	name := strings.ToLower(strings.TrimSpace(providerName))
	switch name {
	case "hash", "", "dev":
		return NewHashProvider(dims)
	case "nomic", "nomic-v2", "nomic-v2-moe":
		return newONNXProvider(dims, modelPath)
	default:
		if strings.HasPrefix(name, "onnx") || strings.HasPrefix(providerName, "onnx:") {
			return newONNXProvider(dims, modelPath)
		}
		return NewHashProvider(dims)
	}
}
