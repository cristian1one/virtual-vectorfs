//go:build !onnx
// +build !onnx

package embedding

import "fmt"

// ListONNXProviders is a stub when the package is built without ONNX support.
func ListONNXProviders() ([]string, error) {
	return nil, fmt.Errorf("onnx support not built in; rebuild with -tags=onnx to enable")
}
