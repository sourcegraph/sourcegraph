package shared

import (
	"strconv"
	"testing"
	"time"
)

func TestConfigDefaults(t *testing.T) {
	config := Config{}
	// Assume nothing is set explicitly in the env.
	config.SetMockGetter(mapGetter(nil))
	config.Load()

	if err := config.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %s", err)
	}

	if have, want := config.ReposDir, "/data/repos"; have != want {
		t.Errorf("invalid value for ReposDir: have=%s want=%s", have, want)
	}
	if have, want := config.CoursierCacheDir, "/data/repos/coursier"; have != want {
		t.Errorf("invalid value for CoursierCacheDir: have=%s want=%s", have, want)
	}
	if have, want := config.SyncRepoStateInterval, 10*time.Minute; have != want {
		t.Errorf("invalid value for SyncRepoStateInterval: have=%s want=%s", have, want)
	}
	if have, want := config.SyncRepoStateBatchSize, 500; have != want {
		t.Errorf("invalid value for SyncRepoStateBatchSize: have=%d want=%d", have, want)
	}
	if have, want := config.SyncRepoStateUpdatePerSecond, 500; have != want {
		t.Errorf("invalid value for SyncRepoStateUpdatePerSecond: have=%d want=%d", have, want)
	}
	if have, want := config.JanitorReposDesiredPercentFree, 10; have != want {
		t.Errorf("invalid value for JanitorReposDesiredPercentFree: have=%d want=%d", have, want)
	}
	if have, want := config.JanitorInterval, time.Minute; have != want {
		t.Errorf("invalid value for JanitorInterval: have=%s want=%s", have, want)
	}
	if have, want := config.JanitorDisableDeleteReposOnWrongShard, false; have != want {
		t.Errorf("invalid value for JanitorDisableDeleteReposOnWrongShard: have=%t want=%t", have, want)
	}
	if have, want := config.EnableExperimentalJanitor, false; have != want {
		t.Errorf("invalid value for EnableExperimentalJanitor: have=%t want=%t", have, want)
	}
	if have, want := config.ExperimentalJanitorConcurrency, 1; have != want {
		t.Errorf("invalid value for ExperimentalJanitorConcurrency: have=%t want=%t", have, want)
	}
}

func TestConfig_PercentFree(t *testing.T) {
	tests := []struct {
		i       int
		want    int
		wantErr bool
	}{
		{i: -1, wantErr: true},
		{i: -4, wantErr: true},
		{i: 300, wantErr: true},
		{i: 0, want: 0},
		{i: 50, want: 50},
		{i: 100, want: 100},
	}
	for i, tt := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			config := Config{}
			config.SetMockGetter(mapGetter(map[string]string{"SRC_REPOS_DESIRED_PERCENT_FREE": strconv.Itoa(tt.i)}))
			config.Load()

			err := config.Validate()

			if err != nil {
				if !tt.wantErr {
					t.Fatalf("unexpected validation error: %s", err)
				} else {
					// An error was expected and it was returned, so we can end the test here.
					return
				}
			}

			if tt.wantErr && err == nil {
				t.Fatal("unexpected nil validation error")
			}

			if have, want := config.JanitorReposDesiredPercentFree, tt.want; have != want {
				t.Errorf("invalid value for JanitorReposDesiredPercentFree: have=%d want=%d", have, want)
			}
		})
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
