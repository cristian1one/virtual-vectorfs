package tokenizer

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	tk "github.com/sugarme/tokenizer"
	"github.com/sugarme/tokenizer/model/wordpiece"
	"github.com/sugarme/tokenizer/normalizer"
	"github.com/sugarme/tokenizer/pretokenizer"
	"github.com/sugarme/tokenizer/processor"
)

// SugarWordPiece wraps sugarme/tokenizer WordPiece (BERT-style)
type SugarWordPiece struct {
	t         *tk.Tokenizer
	maxSeqLen int
}

// NewSugarWordPiece loads vocab.txt and builds a BERT WordPiece tokenizer
func NewSugarWordPiece(vocabPath string, maxSeq int) (*SugarWordPiece, error) {
	// Prefer initializing WordPiece from a vocab file to avoid nil-map panics
	var wp wordpiece.WordPiece
	if fi, err := os.Stat(vocabPath); err == nil && !fi.IsDir() {
		if nw, err := wordpiece.NewWordPieceFromFile(vocabPath, "[UNK]"); err == nil {
			wp = nw
		} else {
			builder := wordpiece.NewWordPieceBuilder().Files(vocabPath)
			wp = builder.Build()
		}
	} else {
		vocabFile := filepath.Join(vocabPath, "vocab.txt")
		if fi2, err := os.Stat(vocabFile); err == nil && !fi2.IsDir() {
			if nw, err := wordpiece.NewWordPieceFromFile(vocabFile, "[UNK]"); err == nil {
				wp = nw
			} else {
				builder := wordpiece.NewWordPieceBuilder().Files(vocabFile)
				wp = builder.Build()
			}
		} else {
			// fallback: try builder directly with provided path
			builder := wordpiece.NewWordPieceBuilder().Files(vocabPath)
			wp = builder.Build()
		}
	}

	t := tk.NewTokenizer(wp)

	// Basic normalizer and pre-tokenizer similar to BERT
	t.WithNormalizer(normalizer.NewBertNormalizer(true, true, true, true))
	t.WithPreTokenizer(pretokenizer.NewBertPreTokenizer())

	// Determine special token ids: prefer tokenizer.json if present, else infer from vocab file
	clsID := 101
	sepID := 102

	// check for tokenizer.json next to vocabPath
	dir := filepath.Dir(vocabPath)
	tokJSON := filepath.Join(dir, "tokenizer.json")
	if _, err := os.Stat(tokJSON); err == nil {
		b, err := os.ReadFile(tokJSON)
		if err == nil {
			var m map[string]interface{}
			if err := json.Unmarshal(b, &m); err == nil {
				if modelRaw, ok := m["model"]; ok {
					if modelMap, ok := modelRaw.(map[string]interface{}); ok {
						if vocabRaw, ok := modelMap["vocab"]; ok {
							switch v := vocabRaw.(type) {
							case map[string]interface{}:
								if vval, ok := v["[CLS]"]; ok {
									if idf, ok := vval.(float64); ok {
										clsID = int(idf)
									}
								}
								if vval, ok := v["[SEP]"]; ok {
									if idf, ok := vval.(float64); ok {
										sepID = int(idf)
									}
								}
							case []interface{}:
								for idx, it := range v {
									if s, ok := it.(string); ok {
										if s == "[CLS]" {
											clsID = idx
										}
										if s == "[SEP]" {
											sepID = idx
										}
									}
								}
							}
						}
					}
				}
				if vocabRaw, ok := m["vocab"]; ok {
					switch v := vocabRaw.(type) {
					case map[string]interface{}:
						if vval, ok := v["[CLS]"]; ok {
							if idf, ok := vval.(float64); ok {
								clsID = int(idf)
							}
						}
						if vval, ok := v["[SEP]"]; ok {
							if idf, ok := vval.(float64); ok {
								sepID = int(idf)
							}
						}
					case []interface{}:
						for idx, it := range v {
							if s, ok := it.(string); ok {
								if s == "[CLS]" {
									clsID = idx
								}
								if s == "[SEP]" {
									sepID = idx
								}
							}
						}
					}
				}
			}
		}
	} else {
		// fallback: parse vocab file to discover special ids
		if f, err := os.Open(vocabPath); err == nil {
			defer f.Close()
			content, err := io.ReadAll(f)
			if err == nil {
				lines := string(content)
				// build map by line order
				idx := 0
				for _, line := range splitLines(lines) {
					token := line
					if token == "[CLS]" {
						clsID = idx
					}
					if token == "[SEP]" {
						sepID = idx
					}
					idx++
				}
			}
		}
	}

	// Post-processor to add special tokens with discovered ids
	template := processor.NewBertProcessing(processor.PostToken{Value: "[SEP]", Id: sepID}, processor.PostToken{Value: "[CLS]", Id: clsID})
	t.WithPostProcessor(template)
	// Truncation & padding
	t.WithTruncation(&tk.TruncationParams{MaxLength: maxSeq})
	// PaddingParams doesn't support MaxLength in current sugarme version
	t.WithPadding(&tk.PaddingParams{})
	return &SugarWordPiece{t: t, maxSeqLen: maxSeq}, nil
}

func (s *SugarWordPiece) Tokenize(texts []string) ([][]int64, [][]int64, error) {
	ids := make([][]int64, len(texts))
	masks := make([][]int64, len(texts))
	for i, txt := range texts {
		enc, err := s.t.Encode(tk.NewSingleEncodeInput(tk.NewInputSequence(txt)), true)
		if err != nil {
			return nil, nil, err
		}
		uids := enc.GetIds()
		umask := enc.GetAttentionMask()

		// enforce fixed-length output (pad/truncate to maxSeqLen)
		rowIDs := make([]int64, s.maxSeqLen)
		rowMask := make([]int64, s.maxSeqLen)
		n := len(uids)
		if n > s.maxSeqLen {
			n = s.maxSeqLen
		}
		for j := 0; j < n; j++ {
			rowIDs[j] = int64(uids[j])
			if j < len(umask) {
				rowMask[j] = int64(umask[j])
			} else {
				rowMask[j] = 1
			}
		}
		ids[i] = rowIDs
		masks[i] = rowMask
	}
	return ids, masks, nil
}

func splitLines(s string) []string {
	parts := strings.Split(s, "\n")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}
