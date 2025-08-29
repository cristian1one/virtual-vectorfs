//go:build !onnx
// +build !onnx

package embedding

import (
	"context"
	"fmt"
)

// onnxProvider is a stub used when built without the "onnx" build tag.
type onnxProvider struct{ dims int }

func newONNXProvider(dims int, modelPath string) Provider { return &onnxProvider{dims: dims} }

func (p *onnxProvider) Dimensions() int { return p.dims }

func (p *onnxProvider) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	return nil, fmt.Errorf("onnx provider not available: build with -tags onnx and provide a supported model")
}
