package security

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/schema"
)

// mockPolicyOpts configurable options for the mock password policy
type mockPolicyOpts struct {
	policyEnabled, authPolicyEnabled, reqNumber, reqCase bool
	minPasswordLength, specialChars                      int
}

type passwordTest struct {
	password string
	errorStr string
}

type addrTest struct {
	addr string
	pass bool
}

// setMockPasswordPolicyConfig helper for returning a customized mock config
func setMockPasswordPolicyConfig(opts mockPolicyOpts) {
	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			AuthMinPasswordLength: opts.minPasswordLength,
			ExperimentalFeatures: &schema.ExperimentalFeatures{
				PasswordPolicy: &schema.PasswordPolicy{
					Enabled:                   opts.policyEnabled,
					NumberOfSpecialCharacters: 3,
					// invert reqNumber and reqCase so it differs AuthPasswordPolicy
					RequireUpperandLowerCase: !opts.reqNumber,
					RequireAtLeastOneNumber:  !opts.reqCase,
				},
			},
			AuthPasswordPolicy: &schema.AuthPasswordPolicy{
				Enabled:                   opts.authPolicyEnabled,
				NumberOfSpecialCharacters: opts.specialChars,
				RequireAtLeastOneNumber:   opts.reqNumber,
				RequireUpperandLowerCase:  opts.reqCase,
			},
		},
	})
}

func TestGetPasswordPolicy(t *testing.T) {
	t.Run("fetch correct policy", func(t *testing.T) {
		setMockPasswordPolicyConfig(mockPolicyOpts{policyEnabled: true, authPolicyEnabled: true,
			minPasswordLength: 15, specialChars: 2, reqNumber: true, reqCase: true})
		p := conf.AuthPasswordPolicy()

		assert.True(t, p.Enabled)
		assert.Equal(t, p.MinimumLength, 15)
		assert.Equal(t, p.RequireUpperandLowerCase, true)

		// create experimental policy for testing backwards compatability
		conf.Mock(&conf.Unified{
			SiteConfiguration: schema.SiteConfiguration{
				AuthMinPasswordLength: 15,
				ExperimentalFeatures: &schema.ExperimentalFeatures{
					PasswordPolicy: &schema.PasswordPolicy{
						Enabled:                   true,
						NumberOfSpecialCharacters: 2,
						RequireUpperandLowerCase:  true,
						RequireAtLeastOneNumber:   true,
					},
				},
			},
		})

		p = conf.AuthPasswordPolicy()

		assert.True(t, p.Enabled)
		assert.Equal(t, p.MinimumLength, 15)
		assert.Equal(t, p.RequireUpperandLowerCase, true)
		assert.Equal(t, p.NumberOfSpecialCharacters, 2)
	})
}

func TestFetchPasswordPolicyReturnsNil(t *testing.T) {
	conf.Mock(&conf.Unified{
		SiteConfiguration: schema.SiteConfiguration{
			AuthMinPasswordLength: 9,
		},
	})

	t.Run("When no policy is defined, only check password length ", func(t *testing.T) {
		p := conf.AuthPasswordPolicy()
		assert.False(t, p.Enabled)

		assert.Nil(t, ValidatePassword("idontneedanythingspecial"))
		assert.ErrorContains(t, ValidatePassword("abshort"), "Your password may not be less than 9 or be more than 256 characters.")
	})
}
func TestPasswordPolicy(t *testing.T) {
	var passwordTests = []passwordTest{
		{"Sup3rstr0ngbutn0teno0ugh", "Your password must include at least 2 special character(s)."},
		{"id0hav3symb0lsn0w!!works?", "Your password must include one uppercase letter."},
		{"Andn0w?!!", "Your password may not be less than 15 characters."},
		{strings.Repeat("A", 259), "Your password may not be more than 256 characters."},
	}

	t.Run("correctly detects deviating passwords", func(t *testing.T) {
		setMockPasswordPolicyConfig(mockPolicyOpts{policyEnabled: false, minPasswordLength: 15,
			authPolicyEnabled: true, specialChars: 2, reqNumber: true, reqCase: true})

		for _, p := range passwordTests {
			assert.ErrorContains(t, ValidatePassword(p.password), p.errorStr)
		}
	})

	t.Run("detects correct passwords", func(t *testing.T) {
		// test with all options enabled and a length limit
		setMockPasswordPolicyConfig(mockPolicyOpts{policyEnabled: false, minPasswordLength: 12,
			authPolicyEnabled: true, specialChars: 2,
			reqNumber: true, reqCase: true})
		password := "tH1smustCert@!inlybe0kthen?"
		assert.Nil(t, ValidatePassword(password))

		// test with only a password length limit
		setMockPasswordPolicyConfig(mockPolicyOpts{policyEnabled: false, minPasswordLength: 15,
			authPolicyEnabled: true, specialChars: 0,
			reqNumber: false, reqCase: false})
		password = "thisshouldnowpassaswell"
		assert.Nil(t, ValidatePassword(password))
	})
}

func TestAddrValidation(t *testing.T) {
	var addrTests = []addrTest{
		{"127/0.0.1", false},
		{"-oFooBaz", false},
		{"sourcegraph com", false},
		{"127.0.0.1", true},
		{"127.0.0.1:80", true},
		{"127.0.0.1:foo", false},
		{"sourcegraph.com", true},
		{"sourcegraph.com:443", true},
		{"sourcegraph.com:-baz", false},
		{"git123@sourcegraph.com", true},
		{"git123@127.0.0.1:80", true},
		{"git123@git456@sourcegraph.com", false},
		{"git-123@sourcegraph.com", false},
		{"git-123@sourcegraph.com:foo", false},
		{"git@sourcegraph.com", true},
		{"thissubdomaindoesnotexist.sourcegraph.com", false},
	}

	for _, a := range addrTests {
		t.Run(a.addr, func(t *testing.T) {
			assert.True(t, ValidateRemoteAddr(a.addr) == a.pass)
		})
	}

}
