package executil

import (
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
	"github.com/sourcegraph/sourcegraph/schema"
)

// head will return the first n lines of data.
func head(data []byte, n int) []byte {
	for i, b := range data {
		if b == '\n' {
			n--
			if n == 0 {
				return data[:i+1]
			}
		}
	}
	return data
}

func TestGetTlsExternal(t *testing.T) {
	t.Run("test multiple certs", func(t *testing.T) {
		conf.Mock(&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				ExperimentalFeatures: &schema.ExperimentalFeatures{
					TlsExternal: &schema.TlsExternal{
						Certificates: []string{
							"foo",
							"bar",
							"baz",
						},
					},
				},
			},
		})

		tls := getTlsExternalDoNotInvoke()

		if tls.SSLNoVerify {
			t.Error("expected SSLNoVerify to be false, but got true")
		}

		got, err := os.ReadFile(tls.SSLCAInfo)
		if err != nil {
			t.Fatal(err)
		}
		// We also include system certificates, so when comparing only compare
		// the first 3 lines.
		got = head(got, 3)

		want := `foo
bar
baz
`

		if diff := cmp.Diff(want, string(got)); diff != "" {
			t.Errorf("mismatch in contenst of SSLCAInfo file (-want +got):\n%s", diff)
		}
	})
}

func TestConfigureRemoteGitCommand(t *testing.T) {
	expectedEnv := []string{
		"GIT_ASKPASS=true",
		"GIT_SSH_COMMAND=ssh -o BatchMode=yes -o ConnectTimeout=30",
		"GIT_HTTP_USER_AGENT=git/Sourcegraph-Bot",
	}
	remoteURL, err := vcs.ParseURL("https://foo:bar@example.com/foo.git")
	require.NoError(t, err)
	tests := []struct {
		input        *exec.Cmd
		tlsConfig    *tlsConfig
		expectedEnv  []string
		expectedArgs []string
	}{
		{
			input:        exec.Command("git", "fetch", remoteURL.String()),
			expectedEnv:  append(expectedEnv, "GIT_SG_USERNAME=foo", "GIT_SG_PASSWORD=bar"),
			expectedArgs: []string{"git", "-c", "credential.helper=", "-c", "credential.helper=!f() { echo \"username=$GIT_SG_USERNAME\npassword=$GIT_SG_PASSWORD\"; }; f", "-c", "protocol.version=2", "fetch", "https://example.com/foo.git"},
		},
		{
			input:       exec.Command("git", "ls-remote", remoteURL.String()),
			expectedEnv: append(expectedEnv, "GIT_SG_USERNAME=foo", "GIT_SG_PASSWORD=bar"),

			// Don't use protocol.version=2 for ls-remote because it hurts perf.
			expectedArgs: []string{"git", "-c", "credential.helper=", "-c", "credential.helper=!f() { echo \"username=$GIT_SG_USERNAME\npassword=$GIT_SG_PASSWORD\"; }; f", "ls-remote", "https://example.com/foo.git"},
		},

		{
			// Without remote URL
			input:       exec.Command("git", "ls-remote"),
			expectedEnv: expectedEnv,
			// Don't use protocol.version=2 for ls-remote because it hurts perf.
			expectedArgs: []string{"git", "-c", "credential.helper=", "ls-remote"},
		},

		// tlsConfig tests
		{
			input: exec.Command("git", "ls-remote"),
			tlsConfig: &tlsConfig{
				SSLNoVerify: true,
			},
			expectedEnv:  append(expectedEnv, "GIT_SSL_NO_VERIFY=true"),
			expectedArgs: []string{"git", "-c", "credential.helper=", "ls-remote"},
		},
		{
			input: exec.Command("git", "ls-remote"),
			tlsConfig: &tlsConfig{
				SSLCAInfo: "/tmp/foo.certs",
			},
			expectedEnv:  append(expectedEnv, "GIT_SSL_CAINFO=/tmp/foo.certs"),
			expectedArgs: []string{"git", "-c", "credential.helper=", "ls-remote"},
		},
		// Allow absolute git commands
		{
			input: exec.Command("/foo/bar/git", "ls-remote"),
			tlsConfig: &tlsConfig{
				SSLCAInfo: "/tmp/foo.certs",
			},
			expectedEnv:  append(expectedEnv, "GIT_SSL_CAINFO=/tmp/foo.certs"),
			expectedArgs: []string{"/foo/bar/git", "-c", "credential.helper=", "ls-remote"},
		},
	}

	for _, test := range tests {
		t.Run(strings.Join(test.input.Args, " "), func(t *testing.T) {
			config := test.tlsConfig
			if config == nil {
				config = &tlsConfig{}
			}
			configureRemoteGitCommand(test.input, remoteURL, config)
			assert.Equal(t, test.expectedEnv, test.input.Env)
			assert.Equal(t, test.expectedArgs, test.input.Args)
		})
	}
}

func TestConfigureRemoteP4FusionCommandWithoutCArgs(t *testing.T) {
	expectedEnv := []string{
		"GIT_ASKPASS=true",
		"GIT_SSH_COMMAND=ssh -o BatchMode=yes -o ConnectTimeout=30",
		"GIT_HTTP_USER_AGENT=git/Sourcegraph-Bot",
	}
	input := exec.Command("p4-fusion", "--path", "some_path", "--client", "some_client", "--user", "some_user")
	expectedArgs := []string{"p4-fusion", "--path", "some_path", "--client", "some_client", "--user", "some_user"}

	remoteURL, err := vcs.ParseURL("https://example.com/foo.git")
	require.NoError(t, err)
	configureRemoteGitCommand(input, remoteURL, &tlsConfig{})
	assert.Equal(t, expectedEnv, input.Env)
	assert.Equal(t, expectedArgs, input.Args)
}

func TestRemoveUnsupportedP4Args(t *testing.T) {
	tests := []struct {
		name         string
		input        []string
		expectedArgs []string
	}{
		{
			name:         "empty args",
			input:        []string{},
			expectedArgs: []string{},
		},
		{
			name:         "single -c token without a follow-up, removed",
			input:        []string{"-c"},
			expectedArgs: []string{},
		},
		{
			name:         "no -c args, nothing removed",
			input:        []string{"normal", "args"},
			expectedArgs: []string{"normal", "args"},
		},
		{
			name:         "single -c arg removed",
			input:        []string{"normal", "args", "-c", "oops", "normal_again"},
			expectedArgs: []string{"normal", "args", "normal_again"},
		},
		{
			name:         "multiple -c args removed",
			input:        []string{"normal", "args", "-c", "oops", "normal_again", "-c", "oops2"},
			expectedArgs: []string{"normal", "args", "normal_again"},
		},
		{
			name:         "repeated -c token",
			input:        []string{"-c", "-c", "-c", "not_good", "normal", "args"},
			expectedArgs: []string{"normal", "args"},
		},
		{
			name:         "only -c args, everything removed",
			input:        []string{"-c", "oops", "-c", "-c", "not_good"},
			expectedArgs: []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualArgs := removeUnsupportedP4Args(test.input)
			assert.Equal(t, test.expectedArgs, actualArgs)
		})
	}
}

func TestConfigureRemoteGitCommand_tls(t *testing.T) {
	baseEnv := []string{
		"GIT_ASKPASS=true",
		"GIT_SSH_COMMAND=ssh -o BatchMode=yes -o ConnectTimeout=30",
		"GIT_HTTP_USER_AGENT=git/Sourcegraph-Bot",
	}

	cases := []struct {
		conf *tlsConfig
		want []string
	}{{
		conf: &tlsConfig{},
		want: nil,
	}, {
		conf: &tlsConfig{
			SSLNoVerify: true,
		},
		want: []string{
			"GIT_SSL_NO_VERIFY=true",
		},
	}}
	remoteURL, err := vcs.ParseURL("https://example.com/foo.git")
	require.NoError(t, err)
	for _, tc := range cases {
		cmd := exec.Command("git", "clone")
		configureRemoteGitCommand(cmd, remoteURL, tc.conf)
		want := append(baseEnv, tc.want...)
		assert.Equal(t, want, cmd.Env)
	}
}
