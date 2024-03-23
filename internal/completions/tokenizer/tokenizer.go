package tokenizer

import (
	_ "embed"

	"github.com/pkoukk/tiktoken-go"
)

type Tokenizer struct {
	tk *tiktoken.Tiktoken
}

func (t *Tokenizer) Tokenize(text string) ([]int, error) {
	return t.tk.Encode(text, []string{"all"}, nil), nil
}
