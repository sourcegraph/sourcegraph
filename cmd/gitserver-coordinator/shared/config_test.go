package shared

import (
	"testing"
)

func TestConfigDefaults(t *testing.T) {
	config := Config{}
	// Assume nothing is set explicitly in the env.
	config.SetMockGetter(mapGetter(nil))
	config.Load()

	if err := config.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %s", err)
	}

	if have, want := config.ListenAddress, "::3178"; have != want {
		t.Errorf("invalid value for ReposDir: have=%s want=%s", have, want)
	}
}

func mapGetter(env map[string]string) func(name, defaultValue, description string) string {
	return func(name, defaultValue, description string) string {
		if v, ok := env[name]; ok {
			return v
		}

		return defaultValue
	}
}
