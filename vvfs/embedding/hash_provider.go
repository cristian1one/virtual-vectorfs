package embedding

import (
	"context"
	"crypto/sha256"
)

type hashProvider struct{ dims int }

func NewHashProvider(dims int) *hashProvider {
	if dims <= 0 {
		dims = 384
	}
	return &hashProvider{dims: dims}
}

func (h *hashProvider) Dimensions() int { return h.dims }

func (h *hashProvider) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	out := make([][]float32, len(inputs))
	for i, s := range inputs {
		sum := sha256.Sum256([]byte(s))
		vec := make([]float32, h.dims)
		// repeat hash bytes to fill dims
		for j := 0; j < h.dims; j++ {
			b := sum[j%len(sum)]
			vec[j] = (float32(int(b)) - 128.0) / 128.0
		}
		out[i] = vec
	}
	return out, nil
}
