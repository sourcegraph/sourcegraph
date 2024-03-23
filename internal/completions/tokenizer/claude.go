package tokenizer

import (
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/pkoukk/tiktoken-go"
)

// claudeJSON is a claudeEncodingFile sourced from Anthropic to allow us to
// emulate their tokenization:
// https://github.com/anthropics/anthropic-tokenizer-typescript/blob/main/claude.json
//
// Also see https://github.com/sourcegraph/srcgql/pull/3
//
//go:embed claude.json
var claudeJSON string

type claudeEncodingFile struct {
	PatStr        string         `json:"pat_str"`
	SpecialTokens map[string]int `json:"special_tokens"`
	BPERanks      string         `json:"bpe_ranks"`
}

// NewAnthropicClaudeTokenizer is a tokenizer that emulates Anthropic's
// tokenization for Claude.
func NewAnthropicClaudeTokenizer() (*Tokenizer, error) {
	var claudeEncodingFile claudeEncodingFile
	err := json.Unmarshal([]byte(claudeJSON), &claudeEncodingFile)
	if err != nil {
		return nil, err
	}

	bpeRanks := strings.Fields(claudeEncodingFile.BPERanks)
	ranks := make(map[string]int, len(bpeRanks))
	for i, encoded := range bpeRanks {
		rank, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			continue
		}
		ranks[string(rank)] = i
	}

	claudeEncoding := &tiktoken.Encoding{
		Name:           "claude",
		PatStr:         claudeEncodingFile.PatStr,
		MergeableRanks: ranks,
		SpecialTokens:  claudeEncodingFile.SpecialTokens,
	}

	bpe, err := tiktoken.NewCoreBPE(claudeEncoding.MergeableRanks, claudeEncoding.SpecialTokens, claudeEncoding.PatStr)
	if err != nil {
		return nil, err
	}

	specialTokensSet := map[string]any{}
	for k := range claudeEncoding.SpecialTokens {
		specialTokensSet[k] = true
	}

	return &Tokenizer{
		tk: tiktoken.NewTiktoken(bpe, claudeEncoding, specialTokensSet),
	}, nil
}
