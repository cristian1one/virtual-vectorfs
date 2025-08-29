package tokenizer

import (
	"fmt"
)

// Tokenizer converts raw text to model-ready token IDs and attention masks
type Tokenizer interface {
	Tokenize(texts []string) (inputIDs [][]int64, attentionMasks [][]int64, err error)
}

// Config holds basic tokenizer settings
type Config struct {
	MaxSeqLen int
}

// ErrUnsupported indicates the tokenizer could not be initialized
var ErrUnsupported = fmt.Errorf("unsupported tokenizer configuration")
