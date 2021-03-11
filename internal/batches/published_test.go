package batches

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v2"
)

func TestPublishedValue(t *testing.T) {
	tests := []struct {
		name    string
		val     interface{}
		True    bool
		False   bool
		Draft   bool
		Invalid bool
	}{
		{name: "True", val: true, True: true},
		{name: "False", val: false, False: true},
		{name: "Draft", val: "draft", Draft: true},
		{name: "Invalid", val: "invalid", Invalid: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := PublishedValue{Val: tc.val}
			if have, want := p.True(), tc.True; have != want {
				t.Fatalf("invalid `true` value: want=%t have=%t", want, have)
			}
			if have, want := p.False(), tc.False; have != want {
				t.Fatalf("invalid `false` value: want=%t have=%t", want, have)
			}
			if have, want := p.Draft(), tc.Draft; have != want {
				t.Fatalf("invalid `draft` value: want=%t have=%t", want, have)
			}
			if have, want := p.Valid(), !tc.Invalid; have != want {
				t.Fatalf("invalid `valid` value: want=%t have=%t", want, have)
			}
		})
	}
	t.Run("JSON marshal", func(t *testing.T) {
		tests := []struct {
			name     string
			val      interface{}
			expected string
		}{
			{name: "true", val: true, expected: "true"},
			{name: "false", val: false, expected: "false"},
			{name: "draft", val: "draft", expected: `"draft"`},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				p := PublishedValue{Val: tc.val}
				j, err := json.Marshal(p)
				if err != nil {
					t.Fatal(err)
				}
				if have, want := string(j), tc.expected; have != want {
					t.Fatalf("invalid JSON generated: want=%q have=%q", want, have)
				}
			})
		}
	})
	t.Run("JSON unmarshal", func(t *testing.T) {
		tests := []struct {
			name     string
			val      string
			expected interface{}
		}{
			{name: "true", val: "true", expected: true},
			{name: "false", val: "false", expected: false},
			{name: "draft", val: `"draft"`, expected: "draft"},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				var p PublishedValue
				if err := json.Unmarshal([]byte(tc.val), &p); err != nil {
					t.Fatal(err)
				}
				if have, want := p.Value(), tc.expected; have != want {
					t.Fatalf("invalid value parsed: want=%q have=%q", want, have)
				}
			})
		}
	})
	t.Run("YAML unmarshal", func(t *testing.T) {
		tests := []struct {
			name     string
			val      string
			expected interface{}
		}{
			{name: "true", val: "true", expected: true},
			{name: "true", val: "yes", expected: true},
			{name: "false", val: "false", expected: false},
			{name: "false", val: "no", expected: false},
			{name: "draft", val: "draft", expected: "draft"},
			{name: "draft", val: `"draft"`, expected: "draft"},
		}
		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				var p PublishedValue
				if err := yaml.Unmarshal([]byte(tc.val), &p); err != nil {
					t.Fatal(err)
				}
				if have, want := p.Value(), tc.expected; have != want {
					t.Fatalf("invalid value parsed: want=%q have=%q", want, have)
				}
			})
		}
	})
}
