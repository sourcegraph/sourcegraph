package authz

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/sourcegraph/internal/dotcom"
)

func TestParseAuthorizationHeader(t *testing.T) {
	tests := map[string]struct {
		token    string
		sudoUser string
		err      bool
	}{
		"token tok":                              {token: "tok"},
		"token tok==":                            {token: "tok=="},
		`token token=tok`:                        {token: "tok"},
		`token token="tok=="`:                    {token: "tok=="},
		`token-sudo token="tok==", user="alice"`: {token: "tok==", sudoUser: "alice"},
		`token-sudo token=tok, user="alice"`:     {token: "tok", sudoUser: "alice"},
		`token-sudo token="tok==", user=alice`:   {token: "tok==", sudoUser: "alice"},
		"xyz tok":                                {err: true},
		`token-sudo user="alice"`:                {err: true},
		`token-sudo token="",user="alice"`:       {err: true},
		`token k=v, k=v`:                         {err: true},
	}
	for input, test := range tests {
		t.Run(input, func(t *testing.T) {
			token, sudoUser, err := ParseAuthorizationHeader(input)
			if (err != nil) != test.err {
				t.Errorf("got error %v, want error? %v", err, test.err)
			}
			if err != nil {
				return
			}
			if token != test.token {
				t.Errorf("got token %q, want %q", token, test.token)
			}
			if sudoUser != test.sudoUser {
				t.Errorf("got sudoUser %+v, want %+v", sudoUser, test.sudoUser)
			}
		})
	}

	t.Run("disable sudo token for dotcom", func(t *testing.T) {
		dotcom.MockSourcegraphDotComMode(t, true)

		_, _, err := ParseAuthorizationHeader(`token-sudo token="tok==", user="alice"`)
		got := fmt.Sprintf("%v", err)
		want := "use of access tokens with sudo scope is disabled"
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("Mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("empty token does not raise sudo error on dotcom", func(t *testing.T) {
		dotcom.MockSourcegraphDotComMode(t, true)

		_, _, err := ParseAuthorizationHeader(`token`)
		got := fmt.Sprintf("%v", err)
		want := "no token value in the HTTP Authorization request header"
		if diff := cmp.Diff(want, got); diff != "" {
			t.Fatalf("Mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestParseHTTPCredentials(t *testing.T) {
	tests := map[string]struct {
		scheme  string
		token68 string
		params  map[string]string
		err     bool
	}{
		"scheme v1":                 {scheme: "scheme", token68: "v1"},
		"scheme v1==":               {scheme: "scheme", token68: "v1=="},
		`scheme k1="v1"`:            {scheme: "scheme", params: map[string]string{"k1": "v1"}},
		`scheme-2 k1="v1", k2="v2"`: {scheme: "scheme-2", params: map[string]string{"k1": "v1", "k2": "v2"}},
		`scheme-2 k1=v1, k2="v2"`:   {scheme: "scheme-2", params: map[string]string{"k1": "v1", "k2": "v2"}},
		`scheme k=v, k=v`:           {err: true},
	}
	for input, test := range tests {
		t.Run(input, func(t *testing.T) {
			scheme, token68, params, err := parseHTTPCredentials(input)
			if (err != nil) != test.err {
				t.Errorf("got error %v, want error? %v", err, test.err)
			}
			if err != nil {
				return
			}
			if scheme != test.scheme {
				t.Errorf("got scheme %q, want %q", scheme, test.scheme)
			}
			if token68 != test.token68 {
				t.Errorf("got token68 %q, want %q", token68, test.token68)
			}
			if !reflect.DeepEqual(params, test.params) {
				t.Errorf("got params %+v, want %+v", params, test.params)
			}
		})
	}
}

func TestParseBearerHeader(t *testing.T) {
	tests := map[string]struct {
		token string
		err   bool
	}{
		"Bearer tok":     {token: "tok", err: false},
		"bearer tok":     {token: "tok", err: false},
		"BeARER token":   {token: "token", err: false},
		"Bearer tok tok": {token: "tok tok", err: false},
		"Bearer ":        {token: "", err: false},
		"Bearer":         {token: "", err: true},
		"tok":            {token: "", err: true},
	}
	for input, test := range tests {
		t.Run(input, func(t *testing.T) {
			token, err := ParseBearerHeader(input)
			if test.err {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, test.token, token)
		})
	}
}
