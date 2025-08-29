package tokenizer

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"testing"
)

// TestTokenizerParity compares our Go tokenizer output against a reference
// HuggingFace tokenizer (Python). If Python or transformers isn't available
// the test is skipped.
func TestTokenizerParity(t *testing.T) {
	py, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 not found; skipping parity test")
	}

	// Dump vocab tokens ordered by id from HF bert-base-uncased
	dumpVocab := `import json
from transformers import AutoTokenizer
t=AutoTokenizer.from_pretrained("bert-base-uncased")
v=t.get_vocab()
inv=sorted(v.items(), key=lambda kv:kv[1])
tokens=[k for k,_ in inv]
print(json.dumps(tokens))`

	cmd := exec.Command(py, "-c", dumpVocab)
	out, err := cmd.Output()
	if err != nil {
		t.Skipf("python transformers not available or network issue: %v", err)
	}

	var tokens []string
	if err := json.Unmarshal(out, &tokens); err != nil {
		t.Fatalf("failed to parse vocab JSON: %v", err)
	}

	// write vocab to temp file
	tmp, err := ioutil.TempFile("", "vocab-*.txt")
	if err != nil {
		t.Fatalf("tempfile: %v", err)
	}
	defer os.Remove(tmp.Name())
	for _, tk := range tokens {
		if _, err := tmp.WriteString(tk + "\n"); err != nil {
			t.Fatalf("write vocab: %v", err)
		}
	}
	tmp.Close()

	// build tokenizer from vocab
	swp, err := NewSugarWordPiece(tmp.Name(), 64)
	if err != nil {
		t.Fatalf("NewSugarWordPiece: %v", err)
	}

	// sample sentences
	sents := []string{
		"hello world",
		"the quick brown fox jumps over the lazy dog",
	}

	idsGo, maskGo, err := swp.Tokenize(sents)
	if err != nil {
		t.Fatalf("Tokenize failed: %v", err)
	}

	// Ask Python HF tokenizer for encodings
	pyEnc := `import json
from transformers import AutoTokenizer
t=AutoTokenizer.from_pretrained("bert-base-uncased")
s=["hello world","the quick brown fox jumps over the lazy dog"]
out=[]
for x in s:
    enc=t(x, padding='max_length', truncation=True, max_length=64)
    out.append({'ids':enc['input_ids'],'mask':enc['attention_mask']})
print(json.dumps(out))`

	cmd2 := exec.Command(py, "-c", pyEnc)
	out2, err := cmd2.Output()
	if err != nil {
		t.Skipf("python encode failed: %v", err)
	}

	var pyRes []struct {
		Ids  []int `json:"ids"`
		Mask []int `json:"mask"`
	}
	if err := json.Unmarshal(out2, &pyRes); err != nil {
		t.Fatalf("parse py enc: %v", err)
	}

	if len(pyRes) != len(sents) || len(idsGo) != len(sents) {
		t.Fatalf("mismatched counts")
	}
	for i := range sents {
		// compare lengths
		if len(pyRes[i].Ids) != len(idsGo[i]) {
			t.Fatalf("len mismatch i=%d py=%d go=%d", i, len(pyRes[i].Ids), len(idsGo[i]))
		}
		for j := range idsGo[i] {
			if int(idsGo[i][j]) != pyRes[i].Ids[j] {
				t.Fatalf("id mismatch i=%d j=%d py=%d go=%d", i, j, pyRes[i].Ids[j], idsGo[i][j])
			}
		}
		if len(pyRes[i].Mask) != len(maskGo[i]) {
			t.Fatalf("mask len mismatch i=%d", i)
		}
		for j := range maskGo[i] {
			if int(maskGo[i][j]) != pyRes[i].Mask[j] {
				t.Fatalf("mask mismatch i=%d j=%d py=%d go=%d", i, j, pyRes[i].Mask[j], maskGo[i][j])
			}
		}
	}
}
