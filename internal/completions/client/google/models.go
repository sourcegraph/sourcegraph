package google

// For latest available Google Gemini models,
// See: https://ai.google.dev/gemini-api/docs/models/gemini
const providerName = "google"

// Default API endpoint URL
const (
	defaultAPIHost = "generativelanguage.googleapis.com"
	defaultAPIPath = "/v1beta/models"
)

// Latest stable versions
const Gemini15Flash = "gemini-1.5-flash"
const Gemini15Pro = "gemini-1.5-pro"
const GeminiPro = "gemini-pro"

// Latest versions
const Gemini15FlashLatest = "gemini-1.5-flash-latest"
const Gemini15ProLatest = "gemini-1.5-pro-latest"
const GeminiProLatest = "gemini-pro-latest"

// Fixed stable versions
// NOTE: Only the fixed stable versions support context caching.
// Ref: https://ai.google.dev/gemini-api/docs/caching?lang=node
const Gemini15Flash001 = "gemini-1.5-flash-001"
const Gemini15Pro001 = "gemini-1.5-pro-001"
