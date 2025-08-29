package tokenizer

import (
	"bufio"
	"os"
	"strings"
)

// Minimal WordPiece-like tokenizer placeholder. For production, replace with a
// robust library (e.g., sugarme/tokenizer WordPiece) and handle special tokens.
type WordPiece struct {
	vocab     map[string]int64
	unkID     int64
	clsID     int64
	sepID     int64
	maxSeqLen int
}

func LoadWordPieceFromVocab(path string, maxSeq int) (*WordPiece, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	vocab := make(map[string]int64, 60000)
	var idx int64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		tok := strings.TrimSpace(scanner.Text())
		if tok == "" {
			continue
		}
		vocab[tok] = idx
		idx++
	}
	wp := &WordPiece{vocab: vocab, maxSeqLen: maxSeq}
	// Defaults; real IDs should be looked up
	if id, ok := vocab["[UNK]"]; ok {
		wp.unkID = id
	} else {
		wp.unkID = 100
	}
	if id, ok := vocab["[CLS]"]; ok {
		wp.clsID = id
	} else {
		wp.clsID = 101
	}
	if id, ok := vocab["[SEP]"]; ok {
		wp.sepID = id
	} else {
		wp.sepID = 102
	}
	return wp, scanner.Err()
}

func (w *WordPiece) Tokenize(texts []string) ([][]int64, [][]int64, error) {
	ids := make([][]int64, len(texts))
	masks := make([][]int64, len(texts))
	for i, t := range texts {
		tokens := strings.Fields(t)
		seq := make([]int64, 0, w.maxSeqLen)
		mask := make([]int64, 0, w.maxSeqLen)
		seq = append(seq, w.clsID)
		mask = append(mask, 1)
		for _, tk := range tokens {
			id, ok := w.vocab[tk]
			if !ok {
				id = w.unkID
			}
			seq = append(seq, id)
			mask = append(mask, 1)
			if len(seq) >= w.maxSeqLen-1 {
				break
			}
		}
		seq = append(seq, w.sepID)
		mask = append(mask, 1)
		// pad
		for len(seq) < w.maxSeqLen {
			seq = append(seq, 0)
			mask = append(mask, 0)
		}
		ids[i] = seq
		masks[i] = mask
	}
	return ids, masks, nil
}
