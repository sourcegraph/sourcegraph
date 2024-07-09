package analytics

import (
	"context"
	"encoding/json"
	"os"
	"path"
	"testing"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/dev/sg/root"
)

func TestInitIdentity(t *testing.T) {
	tmp := t.TempDir()
	if err := os.Setenv("HOME", tmp); err != nil {
		t.Fatal(err)
	}

	secretStore := NewMockSecretStore()

	sghome, err := root.GetSGHomePath()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("no whoami", func(t *testing.T) {
		if err := InitIdentity(context.Background(), std.NewOutput(os.Stderr, false), secretStore); err == nil {
			t.Fatal("expected error when attempting to fetch external secrets when whoami.json is missing but got none")
		}
	})

	t.Run("empty whoami", func(t *testing.T) {
		if err := os.WriteFile(path.Join(sghome, "whoami.json"), []byte(``), 0o700); err != nil {
			t.Fatal(err)
		}

		if err := InitIdentity(context.Background(), std.NewOutput(os.Stderr, false), secretStore); err == nil {
			t.Fatal("expected error when attempting to fetch external secrets when whoami.json is empty but got none")
		}
	})

	t.Run("misformated whoami", func(t *testing.T) {
		if err := os.WriteFile(path.Join(sghome, "whoami.json"), []byte(`{`), 0o700); err != nil {
			t.Fatal(err)
		}

		if err := InitIdentity(context.Background(), std.NewOutput(os.Stderr, false), secretStore); err == nil {
			t.Fatal("expected error when attempting to fetch external secrets when whoami.json is misformated but got none")
		}
	})

	t.Run("empty email", func(t *testing.T) {
		if err := os.WriteFile(path.Join(sghome, "whoami.json"), []byte(`{"email":""}`), 0o700); err != nil {
			t.Fatal(err)
		}

		if err := InitIdentity(context.Background(), std.NewOutput(os.Stderr, false), secretStore); err == nil {
			t.Fatal("expected error when attempting to fetch external secrets when whoami.json email is empty but got none")
		}
	})

	t.Run("ci@sourcegraph.com when CI=true", func(t *testing.T) {
		os.Setenv("CI", "true")
		t.Cleanup(func() {
			os.Unsetenv("CI")
		})
		if err := InitIdentity(context.Background(), std.NewOutput(os.Stderr, false), secretStore); err != nil {
			t.Fatal("expected error when attempting to fetch external secrets when whoami.json is missing but got none")
		}
		fd, err := os.Open(path.Join(sghome, "whoami.json"))
		if err != nil {
			t.Fatal(err)
		}

		var whoami struct {
			Email string `json:"email"`
		}
		if err := json.NewDecoder(fd).Decode(&whoami); err != nil {
			t.Fatalf("failed to decode ci whoami json: %v", err)
		}

		if whoami.Email != CIIdentity {
			t.Errorf("expected idenity to be %q got %q", CIIdentity, whoami.Email)
		}
	})

	t.Run("well formed", func(t *testing.T) {
		if err := os.WriteFile(path.Join(sghome, "whoami.json"), []byte(`{"email":"bananaphone"}`), 0o700); err != nil {
			t.Fatal(err)
		}

		if err := InitIdentity(context.Background(), std.NewOutput(os.Stderr, false), secretStore); err != nil {
			t.Fatalf("expected no error when whoami is well formatted but got %v", err)
		}
	})
}
