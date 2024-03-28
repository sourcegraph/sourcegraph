package tokenusage_test

import (
	"testing"

	"github.com/sourcegraph/sourcegraph/internal/completions/tokenusage"
	"github.com/sourcegraph/sourcegraph/internal/rcache"
)

func TestTokenizeAndCalculateUsage(t *testing.T) {
	rcache.SetupForTest(t)
	mockCache := rcache.NewWithTTL("LLMUsage", 1800)
	manager := tokenusage.NewManager()

	err := manager.TokenizeAndCalculateUsage("input text", "output text", "anthropic", "feature1")
	if err != nil {
		t.Fatalf("TokenizeAndCalculateUsage returned an error: %v", err)
	}

	// Verify that token counts are updated in the cache
	inputKey := "anthropic:feature1:input"
	outputKey := "anthropic:feature1:output"

	if val, exists, _ := mockCache.GetInt64(inputKey); !exists || val <= 0 {
		t.Errorf("Expected input token count to be updated in cache, but key %s was not found or value is not positive", inputKey)
	}

	if val, exists, _ := mockCache.GetInt64(outputKey); !exists || val <= 0 {
		t.Errorf("Expected output token count to be updated in cache, but key %s was not found or value is not positive", outputKey)
	}
}

func TestGetAllTokenUsageData(t *testing.T) {
	rcache.SetupForTest(t)
	manager := tokenusage.NewManager()
	cache := rcache.NewWithTTL("LLMUsage", 1800)
	cache.SetInt("LLMUsage:model1:feature1:stream:input", 10)
	cache.SetInt("LLMUsage:model1:feature1:stream:output", 20)

	usageSummary, err := manager.GetAllTokenUsageData()

	if err != nil {
		t.Error(err)
	}

	llmUsage, ok := usageSummary["llm_usage"].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected llm_usage key to be present and be a map")
	}

	models, ok := llmUsage["models"].([]map[string]interface{})
	if !ok || len(models) != 2 {
		t.Fatalf("Expected models to be a slice of map with 2 items, got %d", len(models))
	}

	// Prepare expected results for easier comparison
	expected := map[string]int64{
		"LLMUsage:model1:feature1:stream:input":  10,
		"LLMUsage:model1:feature1:stream:output": 20,
	}

	for _, model := range models {
		description, ok := model["description"].(string)
		if !ok {
			t.Errorf("Expected description to be a string")
			continue
		}
		tokens, ok := model["tokens"].(int64)
		if !ok {
			t.Errorf("Expected tokens to be an int64")
			continue
		}

		if expectedTokens, exists := expected[description]; !exists || expectedTokens != tokens {
			t.Errorf("Expected %d tokens for %s, got %d", expectedTokens, description, tokens)
		}
	}
}
