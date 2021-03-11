package httpheader

import (
	"testing"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestValidateCustom(t *testing.T) {
	tests := map[string]struct {
		input        conf.Unified
		wantProblems conf.Problems
	}{
		"single": {
			input: conf.Unified{SiteConfiguration: schema.SiteConfiguration{
				AuthProviders: []schema.AuthProviders{
					{HttpHeader: &schema.HTTPHeaderAuthProvider{Type: "http-header"}},
				},
			}},
			wantProblems: nil,
		},
		"multiple": {
			input: conf.Unified{SiteConfiguration: schema.SiteConfiguration{
				AuthProviders: []schema.AuthProviders{
					{HttpHeader: &schema.HTTPHeaderAuthProvider{Type: "http-header"}},
					{HttpHeader: &schema.HTTPHeaderAuthProvider{Type: "http-header"}},
				},
			}},
			wantProblems: conf.NewSiteProblems("at most 1"),
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			conf.TestValidator(t, test.input, validateConfig, test.wantProblems)
		})
	}
}
