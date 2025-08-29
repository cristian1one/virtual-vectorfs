package embedding

import "strings"

var onnxEPPreference string
var onnxDeviceID int
var onnxBatchSize int = 32
var onnxEPOptions map[string]string

// SetONNXBatchSize sets the preferred batch size for ONNX batched inference.
func SetONNXBatchSize(n int) {
	if n > 0 {
		onnxBatchSize = n
	}
}

// SetONNXExecutionProvider sets preferred ONNX Runtime EP: "cuda", "tensorrt", "coreml", "dml", or "cpu".
func SetONNXExecutionProvider(ep string) {
	onnxEPPreference = strings.ToLower(strings.TrimSpace(ep))
}

// SetONNXDeviceID sets device ID used by some EPs (e.g., DirectML, CUDA fallback cases).
func SetONNXDeviceID(id int) { onnxDeviceID = id }

// SetONNXEPOptions sets provider-specific options for the selected EP.
func SetONNXEPOptions(opts map[string]string) { onnxEPOptions = opts }
