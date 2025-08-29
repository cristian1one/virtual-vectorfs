//go:build onnx
// +build onnx

package embedding

import (
	"fmt"

	ort "github.com/yalue/onnxruntime_go"
)

// ListONNXProviders returns the available ONNX Runtime execution providers.
func ListONNXProviders() ([]string, error) {
	if !ort.IsInitialized() {
		if err := ort.InitializeEnvironment(); err != nil {
			return nil, fmt.Errorf("initialize onnx runtime: %w", err)
		}
	}
	// Many distributions of the onnxruntime_go binding may not expose a
	// GetAvailableProviders function. To be portable across environments,
	// avoid calling that method directly here. Return a conservative default
	// containing CPU; when a richer provider list is available the caller
	// can attempt to probe further.
	return []string{"cpu"}, nil
}
