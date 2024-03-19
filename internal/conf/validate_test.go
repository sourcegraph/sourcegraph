package conf

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/schema"
)

const (
	executorsAccessToken                        = "executorsAccessToken"
	authOpenIDClientSecret                      = "authOpenIDClientSecret"
	authGitHubClientSecret                      = "authGitHubClientSecret"
	authGitLabClientSecret                      = "authGitLabClientSecret"
	authAzureDevOpsClientSecret                 = "authAzureDevOpsClientSecret"
	emailSMTPPassword                           = "emailSMTPPassword"
	organizationInvitationsSigningKey           = "organizationInvitationsSigningKey"
	githubClientSecret                          = "githubClientSecret"
	dotcomGitHubAppCloudClientSecret            = "dotcomGitHubAppCloudClientSecret"
	dotcomGitHubAppCloudPrivateKey              = "dotcomGitHubAppCloudPrivateKey"
	authUnlockAccountLinkSigningKey             = "authUnlockAccountLinkSigningKey"
	dotcomSrcCliVersionCacheGitHubToken         = "dotcomSrcCliVersionCacheGitHubToken"
	dotcomSrcCliVersionCacheGitHubWebhookSecret = "dotcomSrcCliVersionCacheGitHubWebhookSecret"
	dotcomSAMSClientSecret                      = "dotcomSAMSClientSecret"
)

func TestValidate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		res, err := validate([]byte(schema.SiteSchemaJSON), []byte(`{"maxReposToSearch":123}`))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Errors()) != 0 {
			t.Errorf("errors: %v", res.Errors())
		}
	})

	t.Run("valid with additionalProperties", func(t *testing.T) {
		res, err := validate([]byte(schema.SiteSchemaJSON), []byte(`{"a":123}`))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Errors()) != 0 {
			t.Errorf("errors: %v", res.Errors())
		}
	})

	t.Run("invalid", func(t *testing.T) {
		res, err := validate([]byte(schema.SiteSchemaJSON), []byte(`{"maxReposToSearch":true}`))
		if err != nil {
			t.Fatal(err)
		}
		if len(res.Errors()) == 0 {
			t.Error("want invalid")
		}
	})
}

func TestValidateCustom(t *testing.T) {
	tests := map[string]struct {
		raw         string
		wantProblem string
		wantErr     string
	}{
		"unrecognized auth.providers": {
			raw:     `{"auth.providers":[{"type":"asdf"}]}`,
			wantErr: "tagged union type must have a",
		},
		"valid externalURL": {
			raw: `{"externalURL":"http://example.com"}`,
		},
		"valid externalURL ending with slash": {
			raw: `{"externalURL":"http://example.com/"}`,
		},
		"non-root externalURL": {
			raw:         `{"externalURL":"http://example.com/sourcegraph"}`,
			wantProblem: "externalURL must not be a non-root URL",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			problems, err := validateCustomRaw(conftypes.RawUnified{Site: test.raw})
			if err != nil {
				if test.wantErr == "" {
					t.Fatalf("got unexpected error: %v", err)
				}
				if !strings.Contains(err.Error(), test.wantErr) {
					t.Fatal(err)
				}
				return
			}
			if test.wantProblem == "" {
				if len(problems) > 0 {
					t.Fatalf("unexpected problems: %v", problems)
				}
				return
			}
			for _, p := range problems {
				if strings.Contains(p.String(), test.wantProblem) {
					return
				}
			}
			t.Fatalf("could not find problem %q in %v", test.wantProblem, problems)
		})
	}
}

func TestValidateSettings(t *testing.T) {
	tests := map[string]struct {
		input        string
		wantProblems []string
	}{
		"valid": {
			input:        `{}`,
			wantProblems: []string{},
		},
		"comment only": {
			input:        `// a`,
			wantProblems: []string{"must be a JSON object (use {} for empty)"},
		},
		"invalid per JSON Schema": {
			input:        `{"experimentalFeatures":123}`,
			wantProblems: []string{"experimentalFeatures: Invalid type. Expected: object, given: integer"},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			problems := ValidateSettings(test.input)
			if !reflect.DeepEqual(problems, test.wantProblems) {
				t.Errorf("got problems %v, want %v", problems, test.wantProblems)
			}
		})
	}
}

func TestDoValidate(t *testing.T) {
	siteSchemaJSON := schema.SiteSchemaJSON

	tests := map[string]struct {
		input        string
		wantProblems []string
	}{
		"valid": {
			input:        `{}`,
			wantProblems: []string{},
		},
		"invalid root": {
			input:        `null`,
			wantProblems: []string{`must be a JSON object (use {} for empty)`},
		},
		"invalid per JSON Schema": {
			input:        `{"externalURL":123}`,
			wantProblems: []string{"externalURL: Invalid type. Expected: string, given: integer"},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			problems := doValidate([]byte(test.input), siteSchemaJSON)
			if !reflect.DeepEqual(problems, test.wantProblems) {
				t.Errorf("got problems %v, want %v", problems, test.wantProblems)
			}
		})
	}
}

func TestProblems(t *testing.T) {
	siteProblems := NewSiteProblems(
		"siteProblem1",
		"siteProblem2",
		"siteProblem3",
	)
	externalServiceProblems := NewExternalServiceProblems(
		"externalServiceProblem1",
		"externalServiceProblem2",
		"externalServiceProblem3",
	)

	var problems Problems
	problems = append(problems, siteProblems...)
	problems = append(problems, externalServiceProblems...)

	{
		messages := make([]string, 0, len(problems))
		messages = append(messages, siteProblems.Messages()...)
		messages = append(messages, externalServiceProblems.Messages()...)

		want := strings.Join(messages, "\n")
		got := strings.Join(problems.Messages(), "\n")
		if want != got {
			t.Errorf("got %q, want %q", got, want)
		}
	}

	{
		want := strings.Join(siteProblems.Messages(), "\n")
		got := strings.Join(problems.Site().Messages(), "\n")
		if want != got {
			t.Errorf("got %q, want %q", got, want)
		}
	}

	{
		want := strings.Join(externalServiceProblems.Messages(), "\n")
		got := strings.Join(problems.ExternalService().Messages(), "\n")
		if want != got {
			t.Errorf("got %q, want %q", got, want)
		}
	}
}

func TestRedactSecrets(t *testing.T) {
	redacted, err := RedactSecrets(
		conftypes.RawUnified{
			Site: getTestSiteWithSecrets(
				testSecrets{
					executorsAccessToken:                        executorsAccessToken,
					authOpenIDClientSecret:                      authOpenIDClientSecret,
					authGitLabClientSecret:                      authGitLabClientSecret,
					authGitHubClientSecret:                      authGitHubClientSecret,
					authAzureDevOpsClientSecret:                 authAzureDevOpsClientSecret,
					emailSMTPPassword:                           emailSMTPPassword,
					organizationInvitationsSigningKey:           organizationInvitationsSigningKey,
					githubClientSecret:                          githubClientSecret,
					dotcomGitHubAppCloudClientSecret:            dotcomGitHubAppCloudClientSecret,
					dotcomGitHubAppCloudPrivateKey:              dotcomGitHubAppCloudPrivateKey,
					dotcomSrcCliVersionCacheGitHubToken:         dotcomSrcCliVersionCacheGitHubToken,
					dotcomSrcCliVersionCacheGitHubWebhookSecret: dotcomSrcCliVersionCacheGitHubWebhookSecret,
					dotcomSAMSClientSecret:                      dotcomSAMSClientSecret,
					authUnlockAccountLinkSigningKey:             authUnlockAccountLinkSigningKey,
				},
			),
		},
	)
	require.NoError(t, err)

	want := getTestSiteWithRedactedSecrets()
	assert.Equal(t, want, redacted.Site)
}

func TestRedactConfSecrets(t *testing.T) {
	conf := `{
  "auth.providers": [
    {
      "clientID": "sourcegraph-client-openid",
      "clientSecret": "strongsecret",
      "displayName": "Keycloak local OpenID Connect #1 (dev)",
      "issuer": "http://localhost:3220/auth/realms/master",
      "type": "openidconnect"
    }
  ]
}`

	want := `{
  "auth.providers": [
    {
      "clientID": "sourcegraph-client-openid",
      "clientSecret": "%s",
      "displayName": "Keycloak local OpenID Connect #1 (dev)",
      "issuer": "http://localhost:3220/auth/realms/master",
      "type": "openidconnect"
    }
  ]
}`

	testCases := []struct {
		name           string
		hashSecrets    bool
		redactedFmtStr string
	}{
		{
			name:        "hashSecrets true",
			hashSecrets: true,
			// This is the first 10 chars of the SHA256 of "strongsecret". See this go playground to
			// verify: https://go.dev/play/p/N-4R4_fO9XI.
			redactedFmtStr: "REDACTED-DATA-CHUNK-f434ecc765",
		},
		{
			name:           "hashSecrets false",
			hashSecrets:    false,
			redactedFmtStr: "REDACTED",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			redacted, err := redactConfSecrets(conftypes.RawUnified{Site: conf}, tc.hashSecrets)
			require.NoError(t, err)

			want := fmt.Sprintf(want, tc.redactedFmtStr)
			assert.Equal(t, want, redacted.Site)
		})
	}
}

func TestReturnSafeConfig(t *testing.T) {
	conf := `{
  "executors.frontendURL": "http://host.docker.internal:3082",
  "batchChanges.rolloutWindows": [{"rate": "unlimited"}]
}`

	want := `{"batchChanges.rolloutWindows":[{"rate":"unlimited"}]}`

	redacted, err := ReturnSafeConfigs(conftypes.RawUnified{Site: conf})
	require.NoError(t, err)

	assert.Equal(t, want, redacted.Site)
}

func TestRedactConfSecretsWithCommentedOutSecret(t *testing.T) {
	conf := `{
  // "executors.accessToken": "supersecret",
  "executors.frontendURL": "http://host.docker.internal:3082"
}`

	want := `{
  // "executors.accessToken": "supersecret",
  "executors.frontendURL": "http://host.docker.internal:3082"
}`

	testCases := []struct {
		name        string
		hashSecrets bool
	}{
		{
			name:        "hashSecrets true",
			hashSecrets: true,
		},
		{
			name:        "hashSecrets false",
			hashSecrets: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			redacted, err := redactConfSecrets(conftypes.RawUnified{Site: conf}, tc.hashSecrets)
			require.NoError(t, err)

			assert.Equal(t, want, redacted.Site)
		})
	}
}

func TestRedactSecrets_AuthProvidersSectionNotAdded(t *testing.T) {
	const cfgWithoutAuthProviders = `{
  "executors.accessToken": "%s"
}`
	redacted, err := RedactSecrets(
		conftypes.RawUnified{
			Site: fmt.Sprintf(cfgWithoutAuthProviders, executorsAccessToken),
		},
	)
	require.NoError(t, err)

	want := fmt.Sprintf(cfgWithoutAuthProviders, "REDACTED")
	assert.Equal(t, want, redacted.Site)
}

func TestUnredactSecrets(t *testing.T) {
	previousSite := getTestSiteWithSecrets(
		testSecrets{
			executorsAccessToken:                        executorsAccessToken,
			authOpenIDClientSecret:                      authOpenIDClientSecret,
			authGitLabClientSecret:                      authGitLabClientSecret,
			authGitHubClientSecret:                      authGitHubClientSecret,
			authAzureDevOpsClientSecret:                 authAzureDevOpsClientSecret,
			emailSMTPPassword:                           emailSMTPPassword,
			organizationInvitationsSigningKey:           organizationInvitationsSigningKey,
			githubClientSecret:                          githubClientSecret,
			dotcomGitHubAppCloudClientSecret:            dotcomGitHubAppCloudClientSecret,
			dotcomGitHubAppCloudPrivateKey:              dotcomGitHubAppCloudPrivateKey,
			dotcomSrcCliVersionCacheGitHubToken:         dotcomSrcCliVersionCacheGitHubToken,
			dotcomSrcCliVersionCacheGitHubWebhookSecret: dotcomSrcCliVersionCacheGitHubWebhookSecret,
			authUnlockAccountLinkSigningKey:             authUnlockAccountLinkSigningKey,
		},
	)

	t.Run("replaces REDACTED with corresponding secret", func(t *testing.T) {
		input := getTestSiteWithRedactedSecrets()
		unredactedSite, err := UnredactSecrets(input, conftypes.RawUnified{Site: previousSite})
		require.NoError(t, err)
		assert.NotContains(t, unredactedSite, redactedSecret)
		assert.Equal(t, previousSite, unredactedSite)
	})

	t.Run("unredacts secrets AND respects specified edits to secret", func(t *testing.T) {
		input := getTestSiteWithSecrets(
			testSecrets{
				executorsAccessToken:                        "new" + executorsAccessToken,
				authOpenIDClientSecret:                      redactedSecret,
				authGitLabClientSecret:                      "new" + authGitLabClientSecret,
				authGitHubClientSecret:                      redactedSecret,
				authAzureDevOpsClientSecret:                 redactedSecret,
				emailSMTPPassword:                           redactedSecret,
				organizationInvitationsSigningKey:           redactedSecret,
				githubClientSecret:                          redactedSecret,
				dotcomGitHubAppCloudClientSecret:            redactedSecret,
				dotcomGitHubAppCloudPrivateKey:              redactedSecret,
				dotcomSrcCliVersionCacheGitHubToken:         redactedSecret,
				dotcomSrcCliVersionCacheGitHubWebhookSecret: redactedSecret,
				authUnlockAccountLinkSigningKey:             redactedSecret,
			},
		)
		unredactedSite, err := UnredactSecrets(input, conftypes.RawUnified{Site: previousSite})
		require.NoError(t, err)

		// Expect to have newly-specified secrets and to fill in "REDACTED" secrets with secrets from previous site
		want := getTestSiteWithSecrets(
			testSecrets{
				executorsAccessToken:                        "new" + executorsAccessToken,
				authOpenIDClientSecret:                      authOpenIDClientSecret,
				authGitLabClientSecret:                      "new" + authGitLabClientSecret,
				authGitHubClientSecret:                      authGitHubClientSecret,
				authAzureDevOpsClientSecret:                 authAzureDevOpsClientSecret,
				emailSMTPPassword:                           emailSMTPPassword,
				organizationInvitationsSigningKey:           organizationInvitationsSigningKey,
				githubClientSecret:                          githubClientSecret,
				dotcomGitHubAppCloudClientSecret:            dotcomGitHubAppCloudClientSecret,
				dotcomGitHubAppCloudPrivateKey:              dotcomGitHubAppCloudPrivateKey,
				dotcomSrcCliVersionCacheGitHubToken:         dotcomSrcCliVersionCacheGitHubToken,
				dotcomSrcCliVersionCacheGitHubWebhookSecret: dotcomSrcCliVersionCacheGitHubWebhookSecret,
				authUnlockAccountLinkSigningKey:             authUnlockAccountLinkSigningKey,
			},
		)
		assert.Equal(t, want, unredactedSite)
	})

	t.Run("unredacts secrets and respects edits to config", func(t *testing.T) {
		const newEmail = "new_email@example.com"
		input := getTestSiteWithSecrets(
			testSecrets{
				executorsAccessToken:                        "new" + executorsAccessToken,
				authOpenIDClientSecret:                      redactedSecret,
				authGitLabClientSecret:                      "new" + authGitLabClientSecret,
				authGitHubClientSecret:                      redactedSecret,
				authAzureDevOpsClientSecret:                 redactedSecret,
				emailSMTPPassword:                           redactedSecret,
				organizationInvitationsSigningKey:           redactedSecret,
				githubClientSecret:                          redactedSecret,
				dotcomGitHubAppCloudClientSecret:            redactedSecret,
				dotcomGitHubAppCloudPrivateKey:              redactedSecret,
				dotcomSrcCliVersionCacheGitHubToken:         redactedSecret,
				dotcomSrcCliVersionCacheGitHubWebhookSecret: redactedSecret,
				authUnlockAccountLinkSigningKey:             redactedSecret,
			},
			newEmail,
		)
		unredactedSite, err := UnredactSecrets(input, conftypes.RawUnified{Site: previousSite})
		require.NoError(t, err)

		// Expect new secrets and new email to show up in the unredacted version
		want := getTestSiteWithSecrets(
			testSecrets{
				executorsAccessToken:                        "new" + executorsAccessToken,
				authOpenIDClientSecret:                      authOpenIDClientSecret,
				authGitLabClientSecret:                      "new" + authGitLabClientSecret,
				authGitHubClientSecret:                      authGitHubClientSecret,
				authAzureDevOpsClientSecret:                 authAzureDevOpsClientSecret,
				emailSMTPPassword:                           emailSMTPPassword,
				organizationInvitationsSigningKey:           organizationInvitationsSigningKey,
				githubClientSecret:                          githubClientSecret,
				dotcomGitHubAppCloudClientSecret:            dotcomGitHubAppCloudClientSecret,
				dotcomGitHubAppCloudPrivateKey:              dotcomGitHubAppCloudPrivateKey,
				dotcomSrcCliVersionCacheGitHubToken:         dotcomSrcCliVersionCacheGitHubToken,
				dotcomSrcCliVersionCacheGitHubWebhookSecret: dotcomSrcCliVersionCacheGitHubWebhookSecret,
				authUnlockAccountLinkSigningKey:             authUnlockAccountLinkSigningKey,
			},
			newEmail,
		)
		assert.Equal(t, want, unredactedSite)
	})
}

func getTestSiteWithRedactedSecrets() string {
	return getTestSiteWithSecrets(
		testSecrets{
			executorsAccessToken:                        redactedSecret,
			authOpenIDClientSecret:                      redactedSecret,
			authGitLabClientSecret:                      redactedSecret,
			authGitHubClientSecret:                      redactedSecret,
			authAzureDevOpsClientSecret:                 redactedSecret,
			emailSMTPPassword:                           redactedSecret,
			organizationInvitationsSigningKey:           redactedSecret,
			githubClientSecret:                          redactedSecret,
			dotcomGitHubAppCloudClientSecret:            redactedSecret,
			dotcomGitHubAppCloudPrivateKey:              redactedSecret,
			dotcomSrcCliVersionCacheGitHubToken:         redactedSecret,
			dotcomSrcCliVersionCacheGitHubWebhookSecret: redactedSecret,
			dotcomSAMSClientSecret:                      redactedSecret,
			authUnlockAccountLinkSigningKey:             redactedSecret,
		},
	)
}

type testSecrets struct {
	executorsAccessToken                        string
	authOpenIDClientSecret                      string
	authGitHubClientSecret                      string
	authGitLabClientSecret                      string
	authAzureDevOpsClientSecret                 string
	emailSMTPPassword                           string
	organizationInvitationsSigningKey           string
	githubClientSecret                          string
	dotcomGitHubAppCloudClientSecret            string
	dotcomGitHubAppCloudPrivateKey              string
	dotcomSrcCliVersionCacheGitHubToken         string
	dotcomSrcCliVersionCacheGitHubWebhookSecret string
	dotcomSAMSClientSecret                      string
	authUnlockAccountLinkSigningKey             string
}

func getTestSiteWithSecrets(testSecrets testSecrets, optionalEdit ...string) string {
	email := "noreply+dev@sourcegraph.com"
	if len(optionalEdit) > 0 {
		email = optionalEdit[0]
	}
	return fmt.Sprintf(`{
  "repoListUpdateInterval": 1,
  "email.address": "%s",
  "executors.accessToken": "%s",
  "externalURL": "https://sourcegraph.test:3443",
  "update.channel": "release",
  "auth.providers": [
    {
      "allowSignup": true,
      "type": "builtin"
    },
    {
      "clientID": "sourcegraph-client-openid",
      "clientSecret": "%s",
      "displayName": "Keycloak local OpenID Connect #1 (dev)",
      "issuer": "http://localhost:3220/auth/realms/master",
      "type": "openidconnect"
    },
    {
      "clientID": "sourcegraph-client-github",
      "clientSecret": "%s",
      "displayName": "GitHub.com (dev)",
      "type": "github"
    },
    {
      "clientID": "sourcegraph-client-gitlab",
      "clientSecret": "%s",
      "displayName": "GitLab.com",
      "type": "gitlab",
      "url": "https://gitlab.com"
    },
    {
      "apiScope": "vso.code,vso.identity,vso.project,vso.work",
      "clientID": "sourcegraph-client-azuredevops",
      "clientSecret": "%s",
      "displayName": "Azure DevOps",
      "type": "azureDevOps"
    }
  ],
  "observability.tracing": {
    "sampling": "selective"
  },
  "externalService.userMode": "all",
  "email.smtp": {
    "username": "%s",
    "password": "%s"
  },
  "organizationInvitations": {
    "signingKey": "%s"
  },
  "githubClientSecret": "%s",
  "dotcom": {
    "githubApp.cloud": {
      "clientSecret": "%s",
      "privateKey": "%s"
    },
    "srcCliVersionCache": {
      "github": {
        "token": "%s",
        "webhookSecret": "%s"
      }
    },
    "sams.clientSecret": "%s"
  },
  "auth.unlockAccountLinkSigningKey": "%s",
}`,
		email,
		testSecrets.executorsAccessToken,
		testSecrets.authOpenIDClientSecret,
		testSecrets.authGitHubClientSecret,
		testSecrets.authGitLabClientSecret,
		testSecrets.authAzureDevOpsClientSecret,
		testSecrets.emailSMTPPassword, // used again as username
		testSecrets.emailSMTPPassword,
		testSecrets.organizationInvitationsSigningKey,
		testSecrets.githubClientSecret,
		testSecrets.dotcomGitHubAppCloudClientSecret,
		testSecrets.dotcomGitHubAppCloudPrivateKey,
		testSecrets.dotcomSrcCliVersionCacheGitHubToken,
		testSecrets.dotcomSrcCliVersionCacheGitHubWebhookSecret,
		testSecrets.dotcomSAMSClientSecret,
		testSecrets.authUnlockAccountLinkSigningKey,
	)
}
