package extsvc

import (
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"

	"github.com/sourcegraph/sourcegraph/schema"
)

func TestVariantConfigPrototypePointers(t *testing.T) {
	// every call to `Variant::ConfigPrototype` should return a new instance of the prototype type
	// or nil if there is no prototype defined for that variant
	for variant := range variantValuesMap {
		x := variant.ConfigPrototype()
		y := variant.ConfigPrototype()
		if x != nil && x == y {
			t.Errorf("%s pointers are the same: %p == %p", variant.AsKind(), x, y)
		}
	}
	// check all of the current prototypes, thanks to Cody generating this code for me!
	if y, ok := VariantAWSCodeCommit.ConfigPrototype().(*schema.AWSCodeCommitConnection); !ok {
		t.Errorf("wrong type for AWS CodeCommit configuration prototype: %T", y)
	}
	if y, ok := VariantAzureDevOps.ConfigPrototype().(*schema.AzureDevOpsConnection); !ok {
		t.Errorf("wrong type for Azure DevOps configuration prototype: %T", y)
	}
	if y, ok := VariantBitbucketCloud.ConfigPrototype().(*schema.BitbucketCloudConnection); !ok {
		t.Errorf("wrong type for Bitbucket Cloud configuration prototype: %T", y)
	}
	if y, ok := VariantBitbucketServer.ConfigPrototype().(*schema.BitbucketServerConnection); !ok {
		t.Errorf("wrong type for Bitbucket Server configuration prototype: %T", y)
	}
	if y, ok := VariantGerrit.ConfigPrototype().(*schema.GerritConnection); !ok {
		t.Errorf("wrong type for Gerrit configuration prototype: %T", y)
	}
	if y, ok := VariantGitHub.ConfigPrototype().(*schema.GitHubConnection); !ok {
		t.Errorf("wrong type for GitHub configuration prototype: %T", y)
	}
	if y, ok := VariantGitLab.ConfigPrototype().(*schema.GitLabConnection); !ok {
		t.Errorf("wrong type for GitLab configuration prototype: %T", y)
	}
	if y, ok := VariantGitolite.ConfigPrototype().(*schema.GitoliteConnection); !ok {
		t.Errorf("wrong type for Gitolite configuration prototype: %T", y)
	}
	if y, ok := VariantGoPackages.ConfigPrototype().(*schema.GoModulesConnection); !ok {
		t.Errorf("wrong type for Go Packages configuration prototype: %T", y)
	}
	if y, ok := VariantJVMPackages.ConfigPrototype().(*schema.JVMPackagesConnection); !ok {
		t.Errorf("wrong type for JVM Packages configuration prototype: %T", y)
	}
	if y, ok := VariantNpmPackages.ConfigPrototype().(*schema.NpmPackagesConnection); !ok {
		t.Errorf("wrong type for NPM Packages configuration prototype: %T", y)
	}
	if y, ok := VariantOther.ConfigPrototype().(*schema.OtherExternalServiceConnection); !ok {
		t.Errorf("wrong type for Other configuration prototype: %T", y)
	}
	if y, ok := VariantPagure.ConfigPrototype().(*schema.PagureConnection); !ok {
		t.Errorf("wrong type for Pagure configuration prototype: %T", y)
	}
	if y, ok := VariantPerforce.ConfigPrototype().(*schema.PerforceConnection); !ok {
		t.Errorf("wrong type for Perforce configuration prototype: %T", y)
	}
	if y, ok := VariantPhabricator.ConfigPrototype().(*schema.PhabricatorConnection); !ok {
		t.Errorf("wrong type for Phabricator configuration prototype: %T", y)
	}
	if y, ok := VariantPythonPackages.ConfigPrototype().(*schema.PythonPackagesConnection); !ok {
		t.Errorf("wrong type for Python Packages configuration prototype: %T", y)
	}
	if y, ok := VariantRubyPackages.ConfigPrototype().(*schema.RubyPackagesConnection); !ok {
		t.Errorf("wrong type for Ruby Packages configuration prototype: %T", y)
	}
	if y, ok := VariantRustPackages.ConfigPrototype().(*schema.RustPackagesConnection); !ok {
		t.Errorf("wrong type for Rust Packages configuration prototype: %T", y)
	}
}

func TestExtractToken(t *testing.T) {
	for _, tc := range []struct {
		config string
		kind   string
		want   string
	}{
		{
			config: `{"token": "deadbeef"}`,
			kind:   KindGitLab,
			want:   "deadbeef",
		},
		{
			config: `{"token": "deadbeef"}`,
			kind:   KindGitHub,
			want:   "deadbeef",
		},
		{
			config: `{"token": "deadbeef"}`,
			kind:   KindAzureDevOps,
			want:   "deadbeef",
		},
		{
			config: `{"token": "deadbeef"}`,
			kind:   KindBitbucketServer,
			want:   "deadbeef",
		},
		{
			config: `{"token": "deadbeef"}`,
			kind:   KindPhabricator,
			want:   "deadbeef",
		},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			have, err := ExtractToken(tc.config, tc.kind)
			if err != nil {
				t.Fatal(err)
			}
			if have != tc.want {
				t.Errorf("Want %q, have %q", tc.want, have)
			}
		})
	}

	t.Run("fails for unsupported kind", func(t *testing.T) {
		_, err := ExtractToken(`{}`, KindGitolite)
		if err == nil {
			t.Fatal("expected an error for unsupported kind")
		}
	})
}

func TestExtractRateLimitConfig(t *testing.T) {
	for _, tc := range []struct {
		name          string
		config        string
		kind          string
		want          rate.Limit
		expectDefault bool
	}{
		{
			name:          "GitLab default",
			config:        `{"url": "https://example.com/"}`,
			kind:          KindGitLab,
			want:          rate.Inf,
			expectDefault: true,
		},
		{
			name:          "GitHub default",
			config:        `{"url": "https://example.com/"}`,
			kind:          KindGitHub,
			want:          rate.Inf,
			expectDefault: true,
		},
		{
			name:          "Bitbucket Server default",
			config:        `{"url": "https://example.com/"}`,
			kind:          KindBitbucketServer,
			want:          8.0,
			expectDefault: true,
		},
		{
			name:          "Bitbucket Cloud default",
			config:        `{"url": "https://example.com/"}`,
			kind:          KindBitbucketCloud,
			want:          2.0,
			expectDefault: true,
		},
		{
			name:          "GitLab non-default",
			config:        `{"url": "https://example.com/", "rateLimit": {"enabled": true, "requestsPerHour": 3600}}`,
			kind:          KindGitLab,
			want:          1.0,
			expectDefault: false,
		},
		{
			name:          "GitHub non-default",
			config:        `{"url": "https://example.com/", "rateLimit": {"enabled": true, "requestsPerHour": 3600}}`,
			kind:          KindGitHub,
			want:          1.0,
			expectDefault: false,
		},
		{
			name:          "Bitbucket Server non-default",
			config:        `{"url": "https://example.com/", "rateLimit": {"enabled": true, "requestsPerHour": 3600}}`,
			kind:          KindBitbucketServer,
			want:          1.0,
			expectDefault: false,
		},
		{
			name:   "Bitbucket Cloud non-default",
			config: `{"url": "https://example.com/", "rateLimit": {"enabled": true, "requestsPerHour": 3600}}`,
			kind:   KindBitbucketCloud,
			want:   1.0,
		},
		{
			name:          "NPM default",
			config:        `{"registry": "https://registry.npmjs.org"}`,
			kind:          KindNpmPackages,
			want:          6000.0 / 3600.0,
			expectDefault: true,
		},
		{
			name:          "NPM non-default",
			config:        `{"registry": "https://registry.npmjs.org", "rateLimit": {"enabled": true, "requestsPerHour": 3600}}`,
			kind:          KindNpmPackages,
			want:          1.0,
			expectDefault: false,
		},
		{
			name:          "Go mod default",
			config:        `{"urls": ["https://example.com"]}`,
			kind:          KindGoPackages,
			want:          57600.0 / 3600.0,
			expectDefault: true,
		},
		{
			name:          "Go mod non-default",
			config:        `{"urls": ["https://example.com"], "rateLimit": {"enabled": true, "requestsPerHour": 3600}}`,
			kind:          KindNpmPackages,
			want:          1.0,
			expectDefault: false,
		},
		{
			name:          "No trailing slash",
			config:        `{"url": "https://example.com", "rateLimit": {"enabled": true, "requestsPerHour": 3600}}`,
			kind:          KindBitbucketCloud,
			want:          1.0,
			expectDefault: false,
		},
		{
			name:          "Empty JVM config",
			config:        "",
			kind:          KindJVMPackages,
			want:          2.0,
			expectDefault: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rlc, isDefault, err := ExtractRateLimit(tc.config, tc.kind)
			if err != nil {
				t.Fatal(err)
			}
			if isDefault != tc.expectDefault {
				t.Fatalf("expected default value: %+v, got: %+v", tc.expectDefault, isDefault)
			}
			if diff := cmp.Diff(tc.want, rlc); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestEncodeURN(t *testing.T) {
	tests := []struct {
		desc    string
		kind    string
		id      int64
		wantURN string
	}{
		{
			desc:    "An empty kind and ID",
			kind:    "",
			id:      0,
			wantURN: "extsvc::0",
		},
		{
			desc:    "A valid kind and ID",
			kind:    "github.com",
			id:      1,
			wantURN: "extsvc:github.com:1",
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			urn := URN(test.kind, test.id)
			if urn != test.wantURN {
				t.Fatalf("got urn %q, want %q", urn, test.wantURN)
			}
		})
	}
}

func TestDecodeURN(t *testing.T) {
	tests := []struct {
		desc     string
		urn      string
		wantKind string
		wantID   int64
	}{
		{
			desc:     "An empty string",
			urn:      "",
			wantKind: "",
			wantID:   0,
		},
		{
			desc:     "An incomplete URN",
			urn:      "extsvc:",
			wantKind: "",
			wantID:   0,
		},
		{
			desc:     "A valid complete URN",
			urn:      "extsvc:github.com:1",
			wantKind: "github.com",
			wantID:   1,
		},
		{
			desc:     "A valid URN with no kind",
			urn:      "extsvc::1",
			wantKind: "",
			wantID:   1,
		},
		{
			desc:     "A URN with floating-point ID",
			urn:      "extsvc:github.com:1.0",
			wantKind: "",
			wantID:   0,
		},
		{
			desc:     "A URN with string ID",
			urn:      "extsvc:github.com:fake",
			wantKind: "",
			wantID:   0,
		},
	}

	for _, test := range tests {
		t.Run(test.desc, func(t *testing.T) {
			kind, id := DecodeURN(test.urn)
			if kind != test.wantKind {
				t.Errorf("got kind %q, want %q", kind, test.wantKind)
			}
			if id != test.wantID {
				t.Errorf("got id %d, want %d", id, test.wantID)
			}
		})
	}
}

func TestUniqueCodeHostIdentifier(t *testing.T) {
	for _, tc := range []struct {
		config string
		kind   string
		want   string
	}{
		{
			kind:   KindGitLab,
			config: `{"url": "https://example.com"}`,
			want:   "https://example.com/",
		},
		{
			kind:   KindGitHub,
			config: `{"url": "https://github.com"}`,
			want:   "https://github.com/",
		},
		{
			kind:   KindGitHub,
			config: `{"url": "https://github.example.com/"}`,
			want:   "https://github.example.com/",
		},
		{
			kind: KindAWSCodeCommit,
			config: `{
				"region": "eu-west-1",
				"accessKeyID": "accesskey",
				"secretAccessKey": "secretaccesskey",
				"gitCredentials": {
					"username": "my-user",
					"password": "my-password"
				}
			}`,
			want: "eu-west-1:accesskey",
		},
		{
			kind:   KindGerrit,
			config: `{"url": "https://example.com"}`,
			want:   "https://example.com/",
		},
		{
			kind:   KindBitbucketServer,
			config: `{"url": "https://bitbucket.sgdev.org/"}`,
			want:   "https://bitbucket.sgdev.org/",
		},

		{
			kind:   KindBitbucketCloud,
			config: `{"url": "https://bitbucket.org/"}`,
			want:   "https://bitbucket.org/",
		},

		{
			kind:   KindGitolite,
			config: `{"host": "ssh://git@gitolite.example.com:2222/"}`,
			want:   "ssh://git@gitolite.example.com:2222/",
		},
		{
			kind:   KindGitolite,
			config: `{"host": "git@gitolite.example.com"}`,
			want:   "git@gitolite.example.com/",
		},
		{
			kind:   KindPerforce,
			config: `{"p4.port": "ssl:111.222.333.444:1666"}`,
			want:   "ssl:111.222.333.444:1666",
		},
		{
			kind:   KindPhabricator,
			config: `{"url": "https://phabricator.sgdev.org/"}`,
			want:   "https://phabricator.sgdev.org/",
		},
		{
			kind:   KindOther,
			config: `{"url": "ssh://user@host.xz:2333/"}`,
			want:   "ssh://user@host.xz:2333/",
		},
	} {
		t.Run(tc.kind, func(t *testing.T) {
			have, err := UniqueCodeHostIdentifier(tc.kind, tc.config)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(tc.want, have); diff != "" {
				t.Fatal(diff)
			}
		})
	}
}

func TestWebhookURL(t *testing.T) {
	const externalServiceID = 42
	const externalURLStr = "https://sourcegraph.com"
	externalURL, err := url.Parse(externalURLStr)
	require.NoError(t, err)

	t.Run("unknown kind", func(t *testing.T) {
		u, err := WebhookURL(KindOther, externalServiceID, nil, externalURL)
		assert.Nil(t, err)
		assert.Equal(t, u, "")
	})

	t.Run("basic kinds", func(t *testing.T) {
		for kind, want := range map[string]string{
			KindGitHub:          externalURLStr + "/.api/github-webhooks?externalServiceID=42",
			KindBitbucketServer: externalURLStr + "/.api/bitbucket-server-webhooks?externalServiceID=42",
			KindGitLab:          externalURLStr + "/.api/gitlab-webhooks?externalServiceID=42",
		} {
			t.Run(kind, func(t *testing.T) {
				// Note the use of a nil configuration here: these kinds do not
				// depend on the configuration being passed in or valid.
				have, err := WebhookURL(kind, externalServiceID, nil, externalURL)
				assert.Nil(t, err)
				assert.Equal(t, want, have)
			})
		}
	})

	t.Run("Bitbucket Cloud", func(t *testing.T) {
		t.Run("invalid configurations", func(t *testing.T) {
			for name, cfg := range map[string]any{
				"nil":               nil,
				"GitHub connection": &schema.GitHubConnection{},
			} {
				t.Run(name, func(t *testing.T) {
					_, err := WebhookURL(KindBitbucketCloud, externalServiceID, cfg, externalURL)
					assert.NotNil(t, err)
				})
			}
		})

		t.Run("valid configuration", func(t *testing.T) {
			have, err := WebhookURL(
				KindBitbucketCloud, externalServiceID,
				&schema.BitbucketCloudConnection{
					WebhookSecret: "foo bar",
				},
				externalURL,
			)
			assert.Nil(t, err)
			assert.Equal(t, externalURLStr+"/.api/bitbucket-cloud-webhooks?externalServiceID=42&secret=foo+bar", have)
		})
	})
}

func TestCodeHostURN(t *testing.T) {
	t.Run("normalize URL", func(t *testing.T) {
		const url = "https://github.com"
		urn, err := NewCodeHostBaseURL(url)
		require.NoError(t, err)

		assert.Equal(t, "https://github.com/", urn.String())
	})

	t.Run(`empty CodeHostURN.String() returns ""`, func(t *testing.T) {
		urn := CodeHostBaseURL{}
		assert.Equal(t, "", urn.String())
	})
}
