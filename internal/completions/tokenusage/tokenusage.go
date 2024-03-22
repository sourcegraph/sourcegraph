package tokenusage

import (
	"fmt"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/completions/tokenizer"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type Manager struct {
	Cache *rcache.Cache
}

func NewManager() *Manager {
	return &Manager{
		Cache: rcache.NewWithTTL("LLMUsage", 1800),
	}
}

func (m *Manager) TokenizeAndCalculateUsage(inputText, outputText, model, feature string) error {
	tokenizer, err := tokenizer.NewTokenizer(model)
	if err != nil {
		return errors.Newf("failed to create tokenizer: %w", err)
	}

	inputTokens, err := tokenizer.Tokenize(inputText)
	if err != nil {
		return errors.Newf("failed to tokenize input text: %w", err)
	}

	outputTokens, err := tokenizer.Tokenize(outputText)
	if err != nil {
		return errors.Newf("failed to tokenize output text: %w", err)
	}

	baseKey := fmt.Sprintf("%s:%s:", model, feature)

	if err := m.updateTokenCounts(baseKey+"input", int64(len(inputTokens))); err != nil {
		return errors.Newf("failed to update input token counts: %w", err)
	}
	if err := m.updateTokenCounts(baseKey+"output", int64(len(outputTokens))); err != nil {
		return errors.Newf("failed to update output token counts: %w", err)
	}
	return nil
}

func (m *Manager) updateTokenCounts(key string, tokenCount int64) error {
	if _, err := m.Cache.IncrbyInt64(key, tokenCount); err != nil {
		return errors.Newf("failed to increment token count for key %s: %w", key, err)
	}
	return nil
}

func (m *Manager) GetAllTokenUsageData() (map[string]interface{}, error) {
	allKeys := m.Cache.ListAllKeys()
	var models []map[string]interface{}

	for _, key := range allKeys {
		// Removing redundant prefix from the key
		cleanedKey := strings.SplitN(key, "LLMUsage:", 2)[1]
		value, found, err := m.Cache.GetInt64(cleanedKey)
		//  decrease by int 64 value if found and no error
		if !found {
			// Skip keys that are not found or have conversion errors
			if err != nil {
				return nil, errors.Newf("failed to GetAllTokenUsageData for key %s: %w", key, err)
			}
			continue
		}
		if value < 0 {
			return nil, errors.Newf("negative token count for key %s: %d", key, value)
		}
		if _, err := m.Cache.DecrbyInt64(cleanedKey, value); err != nil {
			return nil, errors.Newf("failed to decrement token count for key %s: %w", key, err)
		}
		model := map[string]interface{}{
			"description": cleanedKey,
			"tokens":      value,
		}
		models = append(models, model)
	}

	result := map[string]interface{}{
		"llm_usage": map[string]interface{}{
			"models": models,
		},
	}
	fmt.Println("this models", models)
	return result, nil
}

// func (m *Manager) WriteToPostgres(db database.DB) {
// 	allKeys := m.Cache.ListAllKeys()
// 	var models []map[string]interface{}

// 	for _, key := range allKeys {
// 		// Removing redundant prefix from the key
// 		cleanedKey := strings.SplitN(key, "LLMUsage:", 2)[1]
// 		value, found, err := m.Cache.GetInt64(cleanedKey)
// 		//  decrease by int 64 value if found and no error
// 		if !found {
// 			// Skip keys that are not found or have conversion errors
// 			if err != nil {
// 				return nil, errors.Newf("failed to GetAllTokenUsageData for key %s: %w", key, err)
// 			}
// 			continue
// 		}
// 		if value < 0 {
// 			return nil, errors.Newf("negative token count for key %s: %d", key, value)
// 		}
// 		if _, err := m.Cache.DecrbyInt64(cleanedKey, value); err != nil {
// 			return nil, errors.Newf("failed to decrement token count for key %s: %w", key, err)
// 		}
// 		model := map[string]interface{}{
// 			"description": cleanedKey,
// 			"tokens":      value,
// 		}
// 		models = append(models, model)
// 	}

// 	result := map[string]interface{}{
// 		"llm_usage": map[string]interface{}{
// 			"models": models,
// 		},
// 	}
// 	fmt.Println("this models", models)
// 	return result, nil
// }
