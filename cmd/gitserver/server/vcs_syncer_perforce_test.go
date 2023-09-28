package server

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/internal/vcs"
)

func TestDecomposePerforceRemoteURL(t *testing.T) {
	t.Run("not a perforce scheme", func(t *testing.T) {
		remoteURL, _ := vcs.ParseURL("https://www.google.com")
		_, _, _, _, _, err := decomposePerforceRemoteURL(remoteURL)
		assert.Error(t, err)
	})

	// Tests are driven from "Examples" from the page:
	// https://www.perforce.com/manuals/cmdref/Content/CmdRef/P4PORT.html
	tests := []struct {
		cloneURL     string
		wantHost     string
		wantUsername string
		wantPassword string
		wantDepot    string
	}{
		{
			cloneURL:     "perforce://admin:password@ssl:111.222.333.444:1666//Sourcegraph/",
			wantHost:     "ssl:111.222.333.444:1666",
			wantUsername: "admin",
			wantPassword: "password",
			wantDepot:    "//Sourcegraph/",
		},
		{
			cloneURL:     "perforce://admin@ssl:111.222.333.444:1666//Sourcegraph/",
			wantHost:     "ssl:111.222.333.444:1666",
			wantUsername: "admin",
			wantDepot:    "//Sourcegraph/",
		},
		{
			cloneURL:  "perforce://ssl:111.222.333.444:1666//Sourcegraph/",
			wantHost:  "ssl:111.222.333.444:1666",
			wantDepot: "//Sourcegraph/",
		},
		{
			cloneURL: "perforce://ssl:111.222.333.444:1666",
			wantHost: "ssl:111.222.333.444:1666",
		},

		{
			cloneURL:     "perforce://admin:password@ssl6:[::]:1818ssl64:[::]:1818//Sourcegraph/",
			wantHost:     "ssl6:[::]:1818ssl64:[::]:1818",
			wantUsername: "admin",
			wantPassword: "password",
			wantDepot:    "//Sourcegraph/",
		},
		{
			cloneURL:     "perforce://admin:password@tcp6:[2001:db8::123]:1818//Sourcegraph/Cloud/",
			wantHost:     "tcp6:[2001:db8::123]:1818",
			wantUsername: "admin",
			wantPassword: "password",
			wantDepot:    "//Sourcegraph/Cloud/",
		},
	}
	for _, test := range tests {
		t.Run(test.cloneURL, func(t *testing.T) {
			remoteURL, _ := vcs.ParseURL(test.cloneURL)
			username, password, host, depot, _, err := decomposePerforceRemoteURL(remoteURL)
			if err != nil {
				t.Fatal(err)
			}

			if host != test.wantHost {
				t.Fatalf("Host: want %q but got %q", test.wantHost, host)
			}
			if username != test.wantUsername {
				t.Fatalf("Username: want %q but got %q", test.wantUsername, username)
			}
			if password != test.wantPassword {
				t.Fatalf("Password: want %q but got %q", test.wantPassword, password)
			}
			if depot != test.wantDepot {
				t.Fatalf("Depot: want %q but got %q", test.wantDepot, depot)
			}
		})
	}
}

func TestSpecifyCommandInErrorMessage(t *testing.T) {
	tests := []struct {
		name        string
		errorMsg    string
		command     *exec.Cmd
		expectedMsg string
	}{
		{
			name:     "empty error message",
			errorMsg: "",
			command: &exec.Cmd{
				Args: []string{"p4", "login", "-s"},
			},
			expectedMsg: "",
		},
		{
			name:     "error message without phrase to replace",
			errorMsg: "Some error",
			command: &exec.Cmd{
				Args: []string{"p4", "login", "-s"},
			},
			expectedMsg: "Some error",
		},
		{
			name:        "error message with phrase to replace, nil input Cmd",
			errorMsg:    "Some error",
			command:     nil,
			expectedMsg: "Some error",
		},
		{
			name:        "error message with phrase to replace, empty input Cmd",
			errorMsg:    "Some error",
			command:     &exec.Cmd{},
			expectedMsg: "Some error",
		},
		{
			name:     "error message with phrase to replace, valid input Cmd",
			errorMsg: "error cloning repo: repo perforce/path/to/depot not cloneable: exit status 1 (output follows)\n\nPerforce password (P4PASSWD) invalid or unset.",
			command: &exec.Cmd{
				Args: []string{"p4", "login", "-s"},
			},
			expectedMsg: "error cloning repo: repo perforce/path/to/depot not cloneable: exit status 1 (output follows)\n\nPerforce password (P4PASSWD) invalid or unset.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actualMsg := specifyCommandInErrorMessage(test.errorMsg, test.command)
			assert.Equal(t, test.expectedMsg, actualMsg)
		})
	}
}

func TestP4DepotSyncer_p4CommandEnv(t *testing.T) {
	syncer := &PerforceDepotSyncer{
		Client: "client",
		P4Home: "p4home",
	}
	vars := syncer.p4CommandEnv("host", "username", "password")
	assertEnv := func(key, value string) {
		var match string
		for _, s := range vars {
			parts := strings.SplitN(s, "=", 2)
			if len(parts) != 2 {
				t.Errorf("Expected 2 parts, got %d in %q", len(parts), s)
				continue
			}
			if parts[0] != key {
				continue
			}
			// Last match wins
			if parts[1] == value {
				match = parts[1]
			}
		}
		if match == "" {
			t.Errorf("No match found for %q", key)
		} else if match != value {
			t.Errorf("Want %q, got %q", value, match)
		}
	}
	assertEnv("HOME", "p4home")
	assertEnv("P4CLIENT", "client")
	assertEnv("P4PORT", "host")
	assertEnv("P4USER", "username")
	assertEnv("P4PASSWD", "password")
}
