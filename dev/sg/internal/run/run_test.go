package run

import (
	"os"
	"testing"
)

func TestMakeEnvMap(t *testing.T) {
	env1 := map[string]string{
		"SRC_TEST_ENV_NAME": "SRC_TEST_ENV_VALUE",
	}
	env2 := map[string]string{
		"SRC_TEST_ENV_NAME": "SRC_TEST_ENV_VALUE2",
	}
	env3 := map[string]string{
		"SRC_TEST_ENV_NAME": "SRC_TEST_ENV_VALUE3",
	}

	testcases := []struct {
		envs []map[string]string
		want string
	}{
		{
			envs: []map[string]string{env1},
			want: "SRC_TEST_ENV_VALUE",
		},
		{
			envs: []map[string]string{env1, env2},
			want: "SRC_TEST_ENV_VALUE2",
		},
		{
			envs: []map[string]string{env1, env2, env3},
			want: "SRC_TEST_ENV_VALUE3",
		},
	}

	t.Run("rightmost value takes precedence", func(t *testing.T) {
		for _, tc := range testcases {
			if got := makeEnvMap(tc.envs...); got["SRC_TEST_ENV_NAME"] != tc.want {
				t.Errorf("makeEnvMap(%v) = %v, want %v", tc.envs, got, tc.want)
			}
		}
	})

	t.Run("os.Environ() is included and not overwritten", func(t *testing.T) {
		if err := os.Setenv("SRC_TEST_ENV_NAME", "PROCESS_ENV_VALUE"); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			os.Unsetenv("SRC_TEST_ENV_NAME")
		})
		if got := makeEnvMap(env1); got["SRC_TEST_ENV_NAME"] != "PROCESS_ENV_VALUE" {
			t.Errorf("makeEnvMap(%v) = %v, want %v", env1, got, "PROCESS_ENV_VALUE")
		}
	})
}
