package conf

import (
	"fmt"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/conf/deploy"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/license"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestAuthPasswordResetLinkDuration(t *testing.T) {
	tests := []struct {
		name string
		sc   *Unified
		want int
	}{{
		name: "password link expiry has a default value if null",
		sc:   &Unified{},
		want: defaultPasswordLinkExpiry,
	}, {
		name: "password link expiry has a default value if blank",
		sc:   &Unified{SiteConfiguration: schema.SiteConfiguration{AuthPasswordResetLinkExpiry: 0}},
		want: defaultPasswordLinkExpiry,
	}, {
		name: "password link expiry can be customized",
		sc:   &Unified{SiteConfiguration: schema.SiteConfiguration{AuthPasswordResetLinkExpiry: 60}},
		want: 60,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			Mock(test.sc)
			if got, want := AuthPasswordResetLinkExpiry(), test.want; got != want {
				t.Fatalf("AuthPasswordResetLinkExpiry() = %v, want %v", got, want)
			}
		})
	}
}

func TestGitLongCommandTimeout(t *testing.T) {
	tests := []struct {
		name string
		sc   *Unified
		want time.Duration
	}{{
		name: "Git long command timeout has a default value if null",
		sc:   &Unified{},
		want: defaultGitLongCommandTimeout,
	}, {
		name: "Git long command timeout has a default value if blank",
		sc:   &Unified{SiteConfiguration: schema.SiteConfiguration{GitLongCommandTimeout: 0}},
		want: defaultGitLongCommandTimeout,
	}, {
		name: "Git long command timeout can be customized",
		sc:   &Unified{SiteConfiguration: schema.SiteConfiguration{GitLongCommandTimeout: 60}},
		want: time.Duration(60) * time.Second,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			Mock(test.sc)
			if got, want := GitLongCommandTimeout(), test.want; got != want {
				t.Fatalf("GitLongCommandTimeout() = %v, want %v", got, want)
			}
		})
	}
}

func TestGitMaxCodehostRequestsPerSecond(t *testing.T) {
	tests := []struct {
		name string
		sc   *Unified
		want int
	}{
		{
			name: "not set should return default",
			sc:   &Unified{SiteConfiguration: schema.SiteConfiguration{}},
			want: -1,
		},
		{
			name: "bad value should return default",
			sc:   &Unified{SiteConfiguration: schema.SiteConfiguration{GitMaxCodehostRequestsPerSecond: pointers.Ptr(-100)}},
			want: -1,
		},
		{
			name: "set 0 should return 0",
			sc:   &Unified{SiteConfiguration: schema.SiteConfiguration{GitMaxCodehostRequestsPerSecond: pointers.Ptr(0)}},
			want: 0,
		},
		{
			name: "set non-0 should return non-0",
			sc:   &Unified{SiteConfiguration: schema.SiteConfiguration{GitMaxCodehostRequestsPerSecond: pointers.Ptr(100)}},
			want: 100,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			Mock(test.sc)
			if got, want := GitMaxCodehostRequestsPerSecond(), test.want; got != want {
				t.Fatalf("GitMaxCodehostRequestsPerSecond() = %v, want %v", got, want)
			}
		})
	}
}

func TestGitMaxConcurrentClones(t *testing.T) {
	tests := []struct {
		name string
		sc   *Unified
		want int
	}{
		{
			name: "not set should return default",
			sc:   &Unified{SiteConfiguration: schema.SiteConfiguration{}},
			want: 5,
		},
		{
			name: "bad value should return default",
			sc: &Unified{
				SiteConfiguration: schema.SiteConfiguration{
					GitMaxConcurrentClones: -100,
				},
			},
			want: 5,
		},
		{
			name: "set non-zero should return non-zero",
			sc: &Unified{
				SiteConfiguration: schema.SiteConfiguration{
					GitMaxConcurrentClones: 100,
				},
			},
			want: 100,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			Mock(test.sc)
			if got, want := GitMaxConcurrentClones(), test.want; got != want {
				t.Fatalf("GitMaxConcurrentClones() = %v, want %v", got, want)
			}
		})
	}
}

func TestAuthLockout(t *testing.T) {
	defer Mock(nil)

	tests := []struct {
		name string
		mock *schema.AuthLockout
		want *schema.AuthLockout
	}{
		{
			name: "missing entire config",
			mock: nil,
			want: &schema.AuthLockout{
				ConsecutivePeriod:      3600,
				FailedAttemptThreshold: 5,
				LockoutPeriod:          1800,
			},
		},
		{
			name: "missing all fields",
			mock: &schema.AuthLockout{},
			want: &schema.AuthLockout{
				ConsecutivePeriod:      3600,
				FailedAttemptThreshold: 5,
				LockoutPeriod:          1800,
			},
		},
		{
			name: "missing some fields",
			mock: &schema.AuthLockout{
				ConsecutivePeriod: 7200,
			},
			want: &schema.AuthLockout{
				ConsecutivePeriod:      7200,
				FailedAttemptThreshold: 5,
				LockoutPeriod:          1800,
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			Mock(&Unified{
				SiteConfiguration: schema.SiteConfiguration{
					AuthLockout: test.mock,
				},
			})

			got := AuthLockout()
			assert.Equal(t, test.want, got)
		})
	}
}

func TestIsAccessRequestEnabled(t *testing.T) {
	falseVal, trueVal := false, true
	tests := []struct {
		name string
		sc   *Unified
		want bool
	}{
		{
			name: "not set should return default true",
			sc:   &Unified{SiteConfiguration: schema.SiteConfiguration{}},
			want: true,
		},
		{
			name: "parent object set should return default true",
			sc: &Unified{
				SiteConfiguration: schema.SiteConfiguration{
					AuthAccessRequest: &schema.AuthAccessRequest{},
				},
			},
			want: true,
		},
		{
			name: "explicitly set enabled=true should return true",
			sc: &Unified{
				SiteConfiguration: schema.SiteConfiguration{
					AuthAccessRequest: &schema.AuthAccessRequest{Enabled: &trueVal},
				},
			},
			want: true,
		},
		{
			name: "explicitly set enabled=false should return false",
			sc: &Unified{
				SiteConfiguration: schema.SiteConfiguration{
					AuthAccessRequest: &schema.AuthAccessRequest{
						Enabled: &falseVal,
					},
				},
			},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			Mock(test.sc)
			have := IsAccessRequestEnabled()
			assert.Equal(t, test.want, have)
		})
	}
}

func TestCodyEnabled(t *testing.T) {
	tests := []struct {
		name string
		sc   schema.SiteConfiguration
		want bool
	}{
		{
			name: "nothing set",
			sc:   schema.SiteConfiguration{},
			want: false,
		},
		{
			name: "cody enabled",
			sc:   schema.SiteConfiguration{CodyEnabled: pointers.Ptr(true)},
			want: true,
		},
		{
			name: "cody disabled",
			sc:   schema.SiteConfiguration{CodyEnabled: pointers.Ptr(false)},
			want: false,
		},
		{
			name: "cody enabled, completions configured",
			sc:   schema.SiteConfiguration{CodyEnabled: pointers.Ptr(true), Completions: &schema.Completions{Model: "foobar"}},
			want: true,
		},
		{
			name: "cody disabled, completions enabled",
			sc:   schema.SiteConfiguration{CodyEnabled: pointers.Ptr(false), Completions: &schema.Completions{Enabled: pointers.Ptr(true), Model: "foobar"}},
			want: false,
		},
		{
			name: "cody disabled, completions configured",
			sc:   schema.SiteConfiguration{CodyEnabled: pointers.Ptr(false), Completions: &schema.Completions{Model: "foobar"}},
			want: false,
		},
		{
			// Legacy support: remove this once completions.enabled is removed
			name: "cody.enabled not set, completions configured but not enabled",
			sc:   schema.SiteConfiguration{Completions: &schema.Completions{Model: "foobar"}},
			want: false,
		},
		{
			// Legacy support: remove this once completions.enabled is removed
			name: "cody.enabled not set, completions configured and enabled",
			sc:   schema.SiteConfiguration{Completions: &schema.Completions{Enabled: pointers.Ptr(true), Model: "foobar"}},
			want: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			Mock(&Unified{SiteConfiguration: test.sc})
			have := CodyEnabled()
			assert.Equal(t, test.want, have)
		})
	}
}

func TestGetCompletionsConfig(t *testing.T) {
	licenseKey := "theasdfkey"
	licenseAccessToken := license.GenerateLicenseKeyBasedAccessToken(licenseKey)
	zeroConfigDefaultWithLicense := &conftypes.CompletionsConfig{
		ChatModel:                "anthropic/claude-2.0",
		ChatModelMaxTokens:       12000,
		FastChatModel:            "anthropic/claude-instant-1",
		FastChatModelMaxTokens:   9000,
		CompletionModel:          "anthropic/claude-instant-1",
		CompletionModelMaxTokens: 9000,
		AccessToken:              licenseAccessToken,
		Provider:                 "sourcegraph",
		Endpoint:                 "https://cody-gateway.sourcegraph.com",
	}

	testCases := []struct {
		name         string
		siteConfig   schema.SiteConfiguration
		deployType   string
		wantConfig   *conftypes.CompletionsConfig
		wantDisabled bool
	}{
		{
			name: "Completions disabled",
			siteConfig: schema.SiteConfiguration{
				LicenseKey: licenseKey,
				Completions: &schema.Completions{
					Enabled: pointers.Ptr(false),
				},
			},
			wantDisabled: true,
		},
		{
			name: "Completions disabled, but Cody enabled",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Completions: &schema.Completions{
					Enabled: pointers.Ptr(false),
				},
			},
			// cody.enabled=true and completions.enabled=false, the newer
			// cody.enabled takes precedence and completions is enabled.
			wantConfig: zeroConfigDefaultWithLicense,
		},
		{
			name: "cody.enabled and empty completions object",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Completions: &schema.Completions{},
			},
			wantConfig: zeroConfigDefaultWithLicense,
		},
		{
			name: "cody.enabled set false",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(false),
				Completions: &schema.Completions{},
			},
			wantDisabled: true,
		},
		{
			name: "no cody config",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: nil,
				Completions: nil,
			},
			wantDisabled: true,
		},
		{
			name: "Invalid provider",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Completions: &schema.Completions{
					Provider: "invalid",
				},
			},
			wantDisabled: true,
		},
		{
			name: "anthropic completions",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Completions: &schema.Completions{
					Enabled:     pointers.Ptr(true),
					Provider:    "anthropic",
					AccessToken: "asdf",
				},
			},
			wantConfig: &conftypes.CompletionsConfig{
				ChatModel:                "claude-2.0",
				ChatModelMaxTokens:       12000,
				FastChatModel:            "claude-instant-1",
				FastChatModelMaxTokens:   9000,
				CompletionModel:          "claude-instant-1",
				CompletionModelMaxTokens: 9000,
				AccessToken:              "asdf",
				Provider:                 "anthropic",
				Endpoint:                 "https://api.anthropic.com/v1/complete",
			},
		},
		{
			name: "anthropic completions, with only completions.enabled",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Completions: &schema.Completions{
					Enabled:         pointers.Ptr(true),
					Provider:        "anthropic",
					AccessToken:     "asdf",
					ChatModel:       "claude-v1",
					CompletionModel: "claude-instant-1",
				},
			},
			wantConfig: &conftypes.CompletionsConfig{
				ChatModel:                "claude-v1",
				ChatModelMaxTokens:       9000,
				FastChatModel:            "claude-instant-1",
				FastChatModelMaxTokens:   9000,
				CompletionModel:          "claude-instant-1",
				CompletionModelMaxTokens: 9000,
				AccessToken:              "asdf",
				Provider:                 "anthropic",
				Endpoint:                 "https://api.anthropic.com/v1/complete",
			},
		},
		{
			name: "soucregraph completions defaults",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Completions: &schema.Completions{
					Provider: "sourcegraph",
				},
			},
			wantConfig: zeroConfigDefaultWithLicense,
		},
		{
			name: "OpenAI completions completions",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Completions: &schema.Completions{
					Provider:    "openai",
					AccessToken: "asdf",
				},
			},
			wantConfig: &conftypes.CompletionsConfig{
				ChatModel:                "gpt-4",
				ChatModelMaxTokens:       7000,
				FastChatModel:            "gpt-3.5-turbo",
				FastChatModelMaxTokens:   4000,
				CompletionModel:          "gpt-3.5-turbo-instruct",
				CompletionModelMaxTokens: 4000,
				AccessToken:              "asdf",
				Provider:                 "openai",
				Endpoint:                 "https://api.openai.com",
			},
		},
		{
			name: "Azure OpenAI completions completions",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Completions: &schema.Completions{
					Provider:        "azure-openai",
					AccessToken:     "asdf",
					Endpoint:        "https://acmecorp.openai.azure.com",
					ChatModel:       "gpt4-deployment",
					FastChatModel:   "gpt35-turbo-deployment",
					CompletionModel: "gpt35-turbo-deployment",
				},
			},
			wantConfig: &conftypes.CompletionsConfig{
				ChatModel:                "gpt4-deployment",
				ChatModelMaxTokens:       7000,
				FastChatModel:            "gpt35-turbo-deployment",
				FastChatModelMaxTokens:   7000,
				CompletionModel:          "gpt35-turbo-deployment",
				CompletionModelMaxTokens: 7000,
				AccessToken:              "asdf",
				Provider:                 "azure-openai",
				Endpoint:                 "https://acmecorp.openai.azure.com",
			},
		},
		{
			name: "Fireworks completions completions",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Completions: &schema.Completions{
					Provider:    "fireworks",
					AccessToken: "asdf",
				},
			},
			wantConfig: &conftypes.CompletionsConfig{
				ChatModel:                "accounts/fireworks/models/llama-v2-7b",
				ChatModelMaxTokens:       3000,
				FastChatModel:            "accounts/fireworks/models/llama-v2-7b",
				FastChatModelMaxTokens:   3000,
				CompletionModel:          "starcoder",
				CompletionModelMaxTokens: 6000,
				AccessToken:              "asdf",
				Provider:                 "fireworks",
				Endpoint:                 "https://api.fireworks.ai/inference/v1/completions",
			},
		},
		{
			name: "AWS Bedrock completions completions",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Completions: &schema.Completions{
					Provider: "aws-bedrock",
					Endpoint: "us-west-2",
				},
			},
			wantConfig: &conftypes.CompletionsConfig{
				ChatModel:                "anthropic.claude-v2",
				ChatModelMaxTokens:       12000,
				FastChatModel:            "anthropic.claude-instant-v1",
				FastChatModelMaxTokens:   9000,
				CompletionModel:          "anthropic.claude-instant-v1",
				CompletionModelMaxTokens: 9000,
				AccessToken:              "",
				Provider:                 "aws-bedrock",
				Endpoint:                 "us-west-2",
			},
		},
		{
			name: "zero-config cody gateway completions without license key",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  "",
			},
			wantDisabled: true,
		},
		{
			name: "zero-config cody gateway completions with license key",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
			},
			wantConfig: zeroConfigDefaultWithLicense,
		},
		{
			name: "zero-config cody gateway completions without provider",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Completions: &schema.Completions{
					ChatModel:       "anthropic/claude-v1.3",
					FastChatModel:   "anthropic/claude-instant-1.3",
					CompletionModel: "anthropic/claude-instant-1.3",
				},
			},
			wantConfig: &conftypes.CompletionsConfig{
				ChatModel:                "anthropic/claude-v1.3",
				ChatModelMaxTokens:       9000,
				FastChatModel:            "anthropic/claude-instant-1.3",
				FastChatModelMaxTokens:   9000,
				CompletionModel:          "anthropic/claude-instant-1.3",
				CompletionModelMaxTokens: 9000,
				AccessToken:              licenseAccessToken,
				Provider:                 "sourcegraph",
				Endpoint:                 "https://cody-gateway.sourcegraph.com",
			},
		},
		{
			// Legacy support for completions.enabled
			name: "legacy field completions.enabled: zero-config cody gateway completions without license key",
			siteConfig: schema.SiteConfiguration{
				Completions: &schema.Completions{Enabled: pointers.Ptr(true)},
				LicenseKey:  "",
			},
			wantDisabled: true,
		},
		{
			name: "legacy field completions.enabled: zero-config cody gateway completions with license key",
			siteConfig: schema.SiteConfiguration{
				Completions: &schema.Completions{
					Enabled: pointers.Ptr(true),
				},
				LicenseKey: licenseKey,
			},
			// Not supported, zero-config is new and should be using the new
			// config.
			wantDisabled: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defaultDeploy := deploy.Type()
			if tc.deployType != "" {
				deploy.Mock(tc.deployType)
			}
			t.Cleanup(func() {
				deploy.Mock(defaultDeploy)
			})
			conf := GetCompletionsConfig(tc.siteConfig)
			if tc.wantDisabled {
				if conf != nil {
					t.Fatalf("expected nil config but got non-nil: %+v", conf)
				}
			} else {
				if conf == nil {
					t.Fatal("unexpected nil config returned")
				}
				if diff := cmp.Diff(tc.wantConfig, conf); diff != "" {
					t.Fatalf("unexpected config computed: %s", diff)
				}
			}
		})
	}
}

func TestGetFeaturesConfig(t *testing.T) {
	zeroConfigDefaultWithLicense := &conftypes.ConfigFeatures{
		Chat:         true,
		AutoComplete: true,
		Commands:     true,
	}

	testCases := []struct {
		name         string
		siteConfig   schema.SiteConfiguration
		deployType   string
		wantConfig   *conftypes.ConfigFeatures
		wantDisabled bool
	}{
		{
			name: "Only Chat enabled",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				ConfigFeatures: &schema.ConfigFeatures{
					Chat: true,
				},
			},
			wantConfig: &conftypes.ConfigFeatures{
				Chat:         true,
				AutoComplete: false,
				Commands:     false,
			},
		},
		{
			name: "Only AutoComplete enabled",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				ConfigFeatures: &schema.ConfigFeatures{
					AutoComplete: true,
				},
			},
			wantConfig: &conftypes.ConfigFeatures{
				Chat:         false,
				AutoComplete: true,
				Commands:     false,
			},
		},
		{
			name: "Only Commands enabled",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				ConfigFeatures: &schema.ConfigFeatures{
					Commands: true,
				},
			},
			wantConfig: &conftypes.ConfigFeatures{
				Chat:         false,
				AutoComplete: false,
				Commands:     true,
			},
		},
		{
			name: "No config given",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled:    pointers.Ptr(true),
				ConfigFeatures: nil,
			},
			wantConfig: &conftypes.ConfigFeatures{
				Chat:         true,
				AutoComplete: true,
				Commands:     true,
			},
		},
		{
			name: "All Config Enabled",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				ConfigFeatures: &schema.ConfigFeatures{
					Commands:     true,
					Chat:         true,
					AutoComplete: true,
				},
			},
			wantConfig: &conftypes.ConfigFeatures{
				Chat:         true,
				AutoComplete: true,
				Commands:     true,
			},
		},
		{
			name: "Commands and Autocomplete Enabled",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				ConfigFeatures: &schema.ConfigFeatures{
					Commands:     true,
					Chat:         false,
					AutoComplete: true,
				},
			},
			wantConfig: &conftypes.ConfigFeatures{
				Chat:         false,
				AutoComplete: true,
				Commands:     true,
			},
		},
	}
	fmt.Println(testCases, zeroConfigDefaultWithLicense, "what is love")

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defaultDeploy := deploy.Type()
			if tc.deployType != "" {
				deploy.Mock(tc.deployType)
			}
			t.Cleanup(func() {
				deploy.Mock(defaultDeploy)
			})
			conf := GetConfigFeatures(tc.siteConfig)
			if tc.wantDisabled {
				if conf != nil {
					t.Fatalf("expected nil config but got non-nil: %+v", conf)
				}
			} else {
				if conf == nil {
					t.Fatal("unexpected nil config returned")
				}
				if diff := cmp.Diff(tc.wantConfig, conf); diff != "" {
					t.Fatalf("unexpected config computed: %s", diff)
				}
			}
		})
	}
}

func TestGetEmbeddingsConfig(t *testing.T) {
	licenseKey := "theasdfkey"
	licenseAccessToken := license.GenerateLicenseKeyBasedAccessToken(licenseKey)
	defaultQdrantConfig := conftypes.QdrantConfig{
		QdrantHNSWConfig: conftypes.QdrantHNSWConfig{
			OnDisk: true,
		},
		QdrantOptimizersConfig: conftypes.QdrantOptimizersConfig{
			IndexingThreshold: 0,
			MemmapThreshold:   100,
		},
		QdrantQuantizationConfig: conftypes.QdrantQuantizationConfig{
			Enabled:  true,
			Quantile: 0.98,
		},
	}
	zeroConfigDefaultWithLicense := &conftypes.EmbeddingsConfig{
		Provider:                   "sourcegraph",
		AccessToken:                licenseAccessToken,
		Model:                      "openai/text-embedding-ada-002",
		Endpoint:                   "https://cody-gateway.sourcegraph.com/v1/embeddings",
		Dimensions:                 1536,
		Incremental:                true,
		MinimumInterval:            24 * time.Hour,
		MaxCodeEmbeddingsPerRepo:   3_072_000,
		MaxTextEmbeddingsPerRepo:   512_000,
		PolicyRepositoryMatchLimit: pointers.Ptr(5000),
		FileFilters: conftypes.EmbeddingsFileFilters{
			MaxFileSizeBytes: 1000000,
		},
		ExcludeChunkOnError: true,
		Qdrant:              defaultQdrantConfig,
	}

	testCases := []struct {
		name         string
		siteConfig   schema.SiteConfiguration
		deployType   string
		wantConfig   *conftypes.EmbeddingsConfig
		dotcom       bool
		wantDisabled bool
	}{
		{
			name: "dotcom Embeddings disabled",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Embeddings: &schema.Embeddings{
					Enabled: pointers.Ptr(false),
				},
			},
			dotcom:       true,
			wantDisabled: true,
		},
		{
			name: "dotcom cody.enabled and empty embeddings object",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Embeddings:  &schema.Embeddings{},
			},
			dotcom:     true,
			wantConfig: zeroConfigDefaultWithLicense,
		},
		{
			name: "dotcom cody.enabled set false",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(false),
				Embeddings:  &schema.Embeddings{},
			},
			dotcom:       true,
			wantDisabled: true,
		},
		{
			name: "dotcom no cody config",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: nil,
				Embeddings:  nil,
			},
			dotcom:       true,
			wantDisabled: true,
		},
		{
			name: "dotcom Invalid provider",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Embeddings: &schema.Embeddings{
					Provider: "invalid",
				},
			},
			dotcom:       true,
			wantDisabled: true,
		},
		{
			name: "dotcom Implicit config with cody.enabled",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
			},
			dotcom:     true,
			wantConfig: zeroConfigDefaultWithLicense,
		},
		{
			name: "dotcom Sourcegraph provider",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Embeddings: &schema.Embeddings{
					Provider: "sourcegraph",
				},
			},
			dotcom:     true,
			wantConfig: zeroConfigDefaultWithLicense,
		},
		{
			name: "dotcom File filters",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Embeddings: &schema.Embeddings{
					Provider: "sourcegraph",
					FileFilters: &schema.FileFilters{
						MaxFileSizeBytes:         200,
						IncludedFilePathPatterns: []string{"*.go"},
						ExcludedFilePathPatterns: []string{"*.java"},
					},
				},
			},
			dotcom: true,
			wantConfig: &conftypes.EmbeddingsConfig{
				Provider:                   "sourcegraph",
				AccessToken:                licenseAccessToken,
				Model:                      "openai/text-embedding-ada-002",
				Endpoint:                   "https://cody-gateway.sourcegraph.com/v1/embeddings",
				Dimensions:                 1536,
				Incremental:                true,
				MinimumInterval:            24 * time.Hour,
				MaxCodeEmbeddingsPerRepo:   3_072_000,
				MaxTextEmbeddingsPerRepo:   512_000,
				PolicyRepositoryMatchLimit: pointers.Ptr(5000),
				FileFilters: conftypes.EmbeddingsFileFilters{
					MaxFileSizeBytes:         200,
					IncludedFilePathPatterns: []string{"*.go"},
					ExcludedFilePathPatterns: []string{"*.java"},
				},
				ExcludeChunkOnError: true,
				Qdrant:              defaultQdrantConfig,
			},
		},
		{
			name: "dotcom File filters w/o MaxFileSizeBytes",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Embeddings: &schema.Embeddings{
					Provider: "sourcegraph",
					FileFilters: &schema.FileFilters{
						IncludedFilePathPatterns: []string{"*.go"},
						ExcludedFilePathPatterns: []string{"*.java"},
					},
				},
			},
			dotcom: true,
			wantConfig: &conftypes.EmbeddingsConfig{
				Provider:                   "sourcegraph",
				AccessToken:                licenseAccessToken,
				Model:                      "openai/text-embedding-ada-002",
				Endpoint:                   "https://cody-gateway.sourcegraph.com/v1/embeddings",
				Dimensions:                 1536,
				Incremental:                true,
				MinimumInterval:            24 * time.Hour,
				MaxCodeEmbeddingsPerRepo:   3_072_000,
				MaxTextEmbeddingsPerRepo:   512_000,
				PolicyRepositoryMatchLimit: pointers.Ptr(5000),
				FileFilters: conftypes.EmbeddingsFileFilters{
					MaxFileSizeBytes:         embeddingsMaxFileSizeBytes,
					IncludedFilePathPatterns: []string{"*.go"},
					ExcludedFilePathPatterns: []string{"*.java"},
				},
				ExcludeChunkOnError: true,
				Qdrant:              defaultQdrantConfig,
			},
		},
		{
			name: "dotcom Disable exclude failed chunk during indexing",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Embeddings: &schema.Embeddings{
					Provider: "sourcegraph",
					FileFilters: &schema.FileFilters{
						MaxFileSizeBytes:         200,
						IncludedFilePathPatterns: []string{"*.go"},
						ExcludedFilePathPatterns: []string{"*.java"},
					},
					ExcludeChunkOnError: pointers.Ptr(false),
				},
			},
			dotcom: true,
			wantConfig: &conftypes.EmbeddingsConfig{
				Provider:                   "sourcegraph",
				AccessToken:                licenseAccessToken,
				Model:                      "openai/text-embedding-ada-002",
				Endpoint:                   "https://cody-gateway.sourcegraph.com/v1/embeddings",
				Dimensions:                 1536,
				Incremental:                true,
				MinimumInterval:            24 * time.Hour,
				MaxCodeEmbeddingsPerRepo:   3_072_000,
				MaxTextEmbeddingsPerRepo:   512_000,
				PolicyRepositoryMatchLimit: pointers.Ptr(5000),
				FileFilters: conftypes.EmbeddingsFileFilters{
					MaxFileSizeBytes:         200,
					IncludedFilePathPatterns: []string{"*.go"},
					ExcludedFilePathPatterns: []string{"*.java"},
				},
				ExcludeChunkOnError: false,
				Qdrant:              defaultQdrantConfig,
			},
		},
		{
			name: "dotcom No provider and no token, assume Sourcegraph",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Embeddings: &schema.Embeddings{
					Model: "openai/text-embedding-bobert-9000",
				},
			},
			dotcom: true,
			wantConfig: &conftypes.EmbeddingsConfig{
				Provider:                   "sourcegraph",
				AccessToken:                licenseAccessToken,
				Model:                      "openai/text-embedding-bobert-9000",
				Endpoint:                   "https://cody-gateway.sourcegraph.com/v1/embeddings",
				Dimensions:                 0, // unknown model used for test case
				Incremental:                true,
				MinimumInterval:            24 * time.Hour,
				MaxCodeEmbeddingsPerRepo:   3_072_000,
				MaxTextEmbeddingsPerRepo:   512_000,
				PolicyRepositoryMatchLimit: pointers.Ptr(5000),
				FileFilters: conftypes.EmbeddingsFileFilters{
					MaxFileSizeBytes: 1000000,
				},
				ExcludeChunkOnError: true,
				Qdrant:              defaultQdrantConfig,
			},
		},
		{
			name: "dotcom Sourcegraph provider without license",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  "",
				Embeddings: &schema.Embeddings{
					Provider: "sourcegraph",
				},
			},
			dotcom:       true,
			wantDisabled: true,
		},
		{
			name: "dotcom OpenAI provider",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Embeddings: &schema.Embeddings{
					Provider:    "openai",
					AccessToken: "asdf",
				},
			},
			dotcom: true,
			wantConfig: &conftypes.EmbeddingsConfig{
				Provider:                   "openai",
				AccessToken:                "asdf",
				Model:                      "text-embedding-ada-002",
				Endpoint:                   "https://api.openai.com/v1/embeddings",
				Dimensions:                 1536,
				Incremental:                true,
				MinimumInterval:            24 * time.Hour,
				MaxCodeEmbeddingsPerRepo:   3_072_000,
				MaxTextEmbeddingsPerRepo:   512_000,
				PolicyRepositoryMatchLimit: pointers.Ptr(5000),
				FileFilters: conftypes.EmbeddingsFileFilters{
					MaxFileSizeBytes: 1000000,
				},
				ExcludeChunkOnError: true,
				Qdrant:              defaultQdrantConfig,
			},
		},
		{
			name: "dotcom OpenAI provider without access token",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Embeddings: &schema.Embeddings{
					Provider: "openai",
				},
			},
			dotcom:       true,
			wantDisabled: true,
		},
		{
			name: "dotcom Azure OpenAI provider",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Embeddings: &schema.Embeddings{
					Provider:    "azure-openai",
					AccessToken: "asdf",
					Endpoint:    "https://acmecorp.openai.azure.com",
					Dimensions:  1536,
					Model:       "the-model",
				},
			},
			dotcom: true,
			wantConfig: &conftypes.EmbeddingsConfig{
				Provider:                   "azure-openai",
				AccessToken:                "asdf",
				Model:                      "the-model",
				Endpoint:                   "https://acmecorp.openai.azure.com",
				Dimensions:                 1536,
				Incremental:                true,
				MinimumInterval:            24 * time.Hour,
				MaxCodeEmbeddingsPerRepo:   3_072_000,
				MaxTextEmbeddingsPerRepo:   512_000,
				PolicyRepositoryMatchLimit: pointers.Ptr(5000),
				FileFilters: conftypes.EmbeddingsFileFilters{
					MaxFileSizeBytes: 1000000,
				},
				ExcludeChunkOnError: true,
				Qdrant:              defaultQdrantConfig,
			},
		},
		{
			name: "Enterprise Implicit config with cody.enabled",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
			},
			dotcom:       false,
			wantDisabled: true,
		},
		{
			name: "Enterprise Sourcegraph provider",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Embeddings: &schema.Embeddings{
					Provider: "sourcegraph",
				},
			},
			dotcom:       false,
			wantDisabled: true,
		},
		{
			name: "Enterprise explict enabled",
			siteConfig: schema.SiteConfiguration{
				CodyEnabled: pointers.Ptr(true),
				LicenseKey:  licenseKey,
				Embeddings: &schema.Embeddings{
					Provider: "sourcegraph",
				},
			},
			dotcom:       false,
			wantDisabled: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defaultDeploy := deploy.Type()
			dotcom.MockSourcegraphDotComMode(t, tc.dotcom)
			if tc.deployType != "" {
				deploy.Mock(tc.deployType)
			}
			t.Cleanup(func() {
				deploy.Mock(defaultDeploy)
			})
			conf := GetEmbeddingsConfig(tc.siteConfig)
			if tc.wantDisabled {
				if conf != nil {
					t.Fatalf("expected nil config but got non-nil: %+v", conf)
				}
			} else {
				if conf == nil {
					t.Fatal("unexpected nil config returned")
				}
				if diff := cmp.Diff(tc.wantConfig, conf); diff != "" {
					t.Fatalf("unexpected config computed: %s", diff)
				}
			}
		})
	}
}

func TestEmailSenderName(t *testing.T) {
	testCases := []struct {
		name       string
		siteConfig schema.SiteConfiguration
		want       string
	}{
		{
			name:       "nothing set",
			siteConfig: schema.SiteConfiguration{},
			want:       "Sourcegraph",
		},
		{
			name: "value set",
			siteConfig: schema.SiteConfiguration{
				EmailSenderName: "Horsegraph",
			},
			want: "Horsegraph",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			Mock(&Unified{SiteConfiguration: tc.siteConfig})
			t.Cleanup(func() { Mock(nil) })

			if got, want := EmailSenderName(), tc.want; got != want {
				t.Fatalf("EmailSenderName() = %v, want %v", got, want)
			}
		})
	}
}

func TestAccessTokenAllowNoExpiration(t *testing.T) {
	testCases := []struct {
		name       string
		siteConfig schema.SiteConfiguration
		want       bool
	}{
		{
			name:       "no accesstoken config set",
			siteConfig: schema.SiteConfiguration{},
			want:       true,
		},
		{
			name: "default value",
			siteConfig: schema.SiteConfiguration{
				AuthAccessTokens: &schema.AuthAccessTokens{
					Allow: string(AccessTokensAll),
				},
			},
			want: true,
		},
		{
			name: "allow no expiration",
			siteConfig: schema.SiteConfiguration{
				AuthAccessTokens: &schema.AuthAccessTokens{
					Allow:             string(AccessTokensAll),
					AllowNoExpiration: pointers.Ptr(true),
				},
			},
			want: true,
		},
		{
			name: "do not allow no expiration",
			siteConfig: schema.SiteConfiguration{
				AuthAccessTokens: &schema.AuthAccessTokens{
					Allow:             string(AccessTokensAll),
					AllowNoExpiration: pointers.Ptr(false),
				},
			},
			want: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			Mock(&Unified{SiteConfiguration: tc.siteConfig})
			t.Cleanup(func() { Mock(nil) })

			if got, want := AccessTokensAllowNoExpiration(), tc.want; got != want {
				t.Fatalf("AccessTokensAllowNoExpiration() = %v, want %v", got, want)
			}
		})
	}
}

func TestAccessTokensExpirationOptions(t *testing.T) {
	testCases := []struct {
		name        string
		siteConfig  schema.SiteConfiguration
		wantDefault int
		wantOptions []int
	}{
		{
			name:        "nil config",
			siteConfig:  schema.SiteConfiguration{},
			wantDefault: 90,
			wantOptions: []int{7, 14, 30, 60, 90},
		},
		{
			name: "empty config",
			siteConfig: schema.SiteConfiguration{
				AuthAccessTokens: &schema.AuthAccessTokens{},
			},
			wantDefault: 90,
			wantOptions: []int{7, 14, 30, 60, 90},
		},
		{
			name: "custom options no default",
			siteConfig: schema.SiteConfiguration{
				AuthAccessTokens: &schema.AuthAccessTokens{
					ExpirationOptionDays: []int{10, 20},
				},
			},
			wantDefault: 90,
			wantOptions: []int{10, 20, 90},
		},
		{
			name: "custom options including default",
			siteConfig: schema.SiteConfiguration{
				AuthAccessTokens: &schema.AuthAccessTokens{
					ExpirationOptionDays:  []int{10, 20},
					DefaultExpirationDays: pointers.Ptr(20),
				},
			},
			wantDefault: 20,
			wantOptions: []int{10, 20},
		},
		{
			name: "ensure options are properly sorted",
			siteConfig: schema.SiteConfiguration{
				AuthAccessTokens: &schema.AuthAccessTokens{
					ExpirationOptionDays:  []int{30, 20, 10},
					DefaultExpirationDays: pointers.Ptr(15),
				},
			},
			wantDefault: 15,
			wantOptions: []int{10, 15, 20, 30},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			Mock(&Unified{
				SiteConfiguration: tc.siteConfig,
			})
			defer Mock(nil)

			defaultDays, options := AccessTokensExpirationOptions()

			assert.Equal(t, tc.wantDefault, defaultDays)
			assert.Equal(t, tc.wantOptions, options)
		})
	}
}
