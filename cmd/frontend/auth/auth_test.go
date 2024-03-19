package auth

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/internal/auth/userpasswd"
)

func TestNormalizeUsername(t *testing.T) {
	userpasswd.MockAddRandomSuffix = func(s string) (string, error) {
		return fmt.Sprintf("%s-ubioa", s), nil
	}
	t.Cleanup(func() {
		userpasswd.MockAddRandomSuffix = nil
	})

	testCases := []struct {
		in     string
		out    string
		hasErr bool
	}{
		{in: "username", out: "username"},
		{in: "john@gmail.com", out: "john-ubioa"},
		{in: "john.appleseed@gmail.com", out: "john.appleseed-ubioa"},
		{in: "john+test@gmail.com", out: "john-test-ubioa"},
		{in: "this@is@not-an-email", out: "this-is-not-an-email"},
		{in: "user.na$e", out: "user.na-e"},
		{in: "2039f0923f0", out: "2039f0923f0"},
		{in: "john(test)@gmail.com", out: "john-test-ubioa"},
		{in: "bob!", out: "bob-"},
		{in: "john_doe", out: "john_doe"},
		{in: "john__doe", out: "john__doe"},
		{in: "_john", out: "_john"},
		{in: "__john", out: "__john"},
		{in: "bob_", out: "bob_"},
		{in: "bob__", out: "bob__"},
		{in: "user_@name", out: "user_-ubioa"},
		{in: "1", out: "1"},
		{in: "a", out: "a"},
		{in: "a-", out: "a-"},
		{in: "--username-", out: "username-"},
		{in: "bob.!bob", out: "bob-bob"},
		{in: "bob@@bob", out: "bob-bob"},
		{in: "username.", out: "username"},
		{in: ".username", out: "username"},
		{in: "user..name", out: "user-name"},
		{in: "user.-name", out: "user-name"},
		{in: ".", hasErr: true},
		{in: "-", hasErr: true},
	}

	for _, tc := range testCases {
		out, err := NormalizeUsername(tc.in)
		if tc.hasErr {
			if err == nil {
				t.Errorf("Expected error on input %q, but there was none, output was %q", tc.in, out)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error on input %q: %s", tc.in, err)
			} else if out != tc.out {
				t.Errorf("Expected %q to normalize to %q, but got %q", tc.in, tc.out, out)
			}

			if !IsValidUsername(out) {
				t.Errorf("Normalization succeeded, but output %q is still not a valid username", out)
			}
		}
	}
}

func Test_AddRandomSuffixToMakeUnique(t *testing.T) {
	const suffixLength = 5

	testCases := []struct {
		username   string
		wantLength int
	}{
		{
			username:   "bob",
			wantLength: 3 + 1 + suffixLength,
		},
		{
			username:   "bob-",
			wantLength: 4 + suffixLength,
		},
		{
			username:   "",
			wantLength: suffixLength,
		},
	}

	rand.Seed(0)
	for _, tc := range testCases {
		// Run a bunch of times to see we're getting consistent results
		for range 100 {
			out, err := userpasswd.AddRandomSuffix(tc.username)
			assert.NoError(t, err, tc.username)
			assert.Len(t, out, tc.wantLength)
			assert.True(t, IsValidUsername(out))
		}
	}
}

func Test_IsValidUsername(t *testing.T) {
	// generate a string of the length 255, with all "a"s
	username255 := string(make([]byte, 255))
	for i := range username255 {
		username255 = username255[:i] + "a" + username255[i+1:]
	}

	testCases := []struct {
		username string
		want     bool
	}{
		{username: "username", want: true},
		{username: "user.name", want: true},
		{username: "username-", want: true},
		{username: username255, want: true},
		{username: "", want: false},
		{username: "user@name", want: false},
		{username: "username--", want: false},
		{username: ".username", want: false},
		{username: "user!name", want: false},
		{username: username255 + "a", want: false},
	}

	for _, tc := range testCases {
		assert.Equal(t, tc.want, IsValidUsername(tc.username), tc.username)
	}
}
