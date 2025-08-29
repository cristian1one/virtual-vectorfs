//go:build onnx
// +build onnx

package embedding

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ZanzyTHEbar/virtual-vectorfs/vvfs/embedding/tokenizer"

	ort "github.com/yalue/onnxruntime_go"
)

// ONNX-backed provider under onnx build tag.
// Initializes ORT and opens a dynamic session. Tokenizer: prefers sugarme WordPiece.
type onnxProvider struct {
	dims        int
	modelPath   string
	mu          sync.Mutex
	session     *ort.DynamicAdvancedSession
	inputNames  []string
	outputNames []string
	tok         tokenizer.Tokenizer
}

func newONNXProvider(dims int, modelPath string) Provider {
	return &onnxProvider{dims: dims, modelPath: modelPath}
}

func (p *onnxProvider) Dimensions() int { return p.dims }

func (p *onnxProvider) ensureSession() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.session != nil {
		return nil
	}
	if p.modelPath == "" {
		return fmt.Errorf("onnx model path is required")
	}
	if !ort.IsInitialized() {
		if err := ort.InitializeEnvironment(); err != nil {
			return fmt.Errorf("initialize onnx runtime: %w", err)
		}
	}
	// Probe IO
	ins, outs, err := ort.GetInputOutputInfo(p.modelPath)
	if err != nil {
		return fmt.Errorf("get IO info: %w", err)
	}
	var inputNames []string
	// Prefer common names
	var idsName, maskName, tokTypeName string
	for _, ii := range ins {
		name := ii.Name
		n := strings.ToLower(name)
		if strings.Contains(n, "input_ids") || n == "input_ids" || n == "ids" {
			idsName = name
		}
		if strings.Contains(n, "attention_mask") || n == "attention_mask" || n == "mask" {
			maskName = name
		}
		if strings.Contains(n, "token_type") || n == "token_type_ids" {
			tokTypeName = name
		}
	}
	if idsName != "" {
		inputNames = append(inputNames, idsName)
	}
	if maskName != "" {
		inputNames = append(inputNames, maskName)
	}
	if tokTypeName != "" {
		inputNames = append(inputNames, tokTypeName)
	}
	// Fallback: take first two int tensor inputs
	if len(inputNames) == 0 {
		for _, ii := range ins {
			if ii.DataType == ort.TensorElementDataTypeInt64 {
				inputNames = append(inputNames, ii.Name)
				if len(inputNames) >= 2 {
					break
				}
			}
		}
	}
	if len(inputNames) == 0 {
		return fmt.Errorf("could not determine ONNX input names")
	}
	// Choose first float output by default
	var outputNames []string
	for _, oi := range outs {
		if oi.DataType == ort.TensorElementDataTypeFloat {
			outputNames = append(outputNames, oi.Name)
			break
		}
	}
	if len(outputNames) == 0 {
		return fmt.Errorf("could not determine ONNX output name")
	}
	// EP detection and options
	var opts *ort.SessionOptions
	// Attempt to construct SessionOptions for the requested EP; fall back to CPU if session
	// creation with the EP fails.
	if onnxEPPreference != "" && onnxEPPreference != "cpu" {
		if o, e := ort.NewSessionOptions(); e == nil {
			_ = o.SetGraphOptimizationLevel(ort.GraphOptimizationLevelEnableAll)
			_ = o.SetIntraOpNumThreads(0)
			_ = o.SetInterOpNumThreads(0)
			switch onnxEPPreference {
			case "cuda":
				if cu, e2 := ort.NewCUDAProviderOptions(); e2 == nil {
					_ = o.AppendExecutionProviderCUDA(cu)
					_ = cu.Destroy()
				}
			case "tensorrt":
				if trt, e2 := ort.NewTensorRTProviderOptions(); e2 == nil {
					_ = o.AppendExecutionProviderTensorRT(trt)
					_ = trt.Destroy()
				}
			case "coreml":
				_ = o.AppendExecutionProviderCoreMLV2(map[string]string{})
			case "dml":
				_ = o.AppendExecutionProviderDirectML(onnxDeviceID)
			}
			opts = o
		}
	}
	// Create session with names and optional options
	var s *ort.DynamicAdvancedSession
	if opts != nil {
		s, err = ort.NewDynamicAdvancedSession(p.modelPath, inputNames, outputNames, opts)
		_ = opts.Destroy()
	} else {
		s, err = ort.NewDynamicAdvancedSession(p.modelPath, inputNames, outputNames, nil)
	}
	if err != nil {
		return fmt.Errorf("create onnx session: %w", err)
	}
	p.session = s
	p.inputNames = inputNames
	p.outputNames = outputNames
	// Initialize tokenizer (prefer sugarme WordPiece; fallback placeholder)
	modelDir := filepath.Dir(p.modelPath)
	vocabPath := filepath.Join(modelDir, "vocab.txt")
	if swp, werr := tokenizer.NewSugarWordPiece(vocabPath, 256); werr == nil {
		p.tok = swp
	} else if wp, werr2 := tokenizer.LoadWordPieceFromVocab(vocabPath, 256); werr2 == nil {
		p.tok = wp
	} else {
		return fmt.Errorf("failed to initialize tokenizer: %v", werr)
	}
	return nil
}

func (p *onnxProvider) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	// implement chunking based on onnxBatchSize
	if err := p.ensureSession(); err != nil {
		return nil, err
	}
	if len(inputs) == 0 {
		return [][]float32{}, nil
	}
	all := make([][]float32, 0, len(inputs))
	for i := 0; i < len(inputs); i += onnxBatchSize {
		end := i + onnxBatchSize
		if end > len(inputs) {
			end = len(inputs)
		}
		chunk := inputs[i:end]
		vecs, err := p.embedChunk(ctx, chunk)
		if err != nil {
			return nil, err
		}
		all = append(all, vecs...)
	}
	return all, nil
}

func (p *onnxProvider) embedChunk(ctx context.Context, inputs []string) ([][]float32, error) {
	// Tokenize
	ids, masks, err := p.tok.Tokenize(inputs)
	if err != nil {
		return nil, err
	}
	batch := len(ids)
	if batch == 0 {
		return [][]float32{}, nil
	}
	seq := len(ids[0])
	// Flatten for tensors
	flatIDs := make([]int64, batch*seq)
	flatMask := make([]int64, batch*seq)
	for i := 0; i < batch; i++ {
		copy(flatIDs[i*seq:(i+1)*seq], ids[i])
		if i < len(masks) {
			copy(flatMask[i*seq:(i+1)*seq], masks[i])
		}
	}
	shape := ort.NewShape(int64(batch), int64(seq))
	idsTensor, err := ort.NewTensor(shape, flatIDs)
	if err != nil {
		return nil, fmt.Errorf("ids tensor: %w", err)
	}
	defer idsTensor.Destroy()
	maskTensor, err := ort.NewTensor(shape, flatMask)
	if err != nil {
		return nil, fmt.Errorf("mask tensor: %w", err)
	}
	defer maskTensor.Destroy()
	// Prepare inputs in same order as inputNames
	inVals := make([]ort.Value, len(p.inputNames))
	for i, name := range p.inputNames {
		ln := strings.ToLower(name)
		if strings.Contains(ln, "input_ids") || ln == "input_ids" || ln == "ids" {
			inVals[i] = idsTensor
		} else if strings.Contains(ln, "attention_mask") || ln == "attention_mask" || ln == "mask" {
			inVals[i] = maskTensor
		} else {
			zero := make([]int64, batch*seq)
			zeroTensor, e := ort.NewTensor(shape, zero)
			if e != nil {
				return nil, fmt.Errorf("alloc zero tensor: %w", e)
			}
			defer zeroTensor.Destroy()
			inVals[i] = zeroTensor
		}
	}
	outs := make([]ort.Value, len(p.outputNames))
	if err := p.session.Run(inVals, outs); err != nil {
		return nil, fmt.Errorf("onnx run: %w", err)
	}
	defer func() {
		for _, v := range outs {
			if v != nil {
				v.Destroy()
			}
		}
	}()
	// Assume first output is [batch, dim] float32
	if t, ok := outs[0].(*ort.Tensor[float32]); ok {
		data := t.GetData()
		shape := t.GetShape()
		if len(shape) != 2 {
			return nil, fmt.Errorf("unexpected output rank %d", len(shape))
		}
		rows, cols := int(shape[0]), int(shape[1])
		vecs := make([][]float32, rows)
		for r := 0; r < rows; r++ {
			start := r * cols
			raw := make([]float32, cols)
			copy(raw, data[start:start+cols])
			vecs[r] = AdjustToDims(raw, p.dims)
		}
		return vecs, nil
	}
	return nil, fmt.Errorf("unexpected output type")
}
