// Package security implements a configurable password policy
// This package may eventually get broken up as other packages are added.
package security

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var userRegex = lazyregexp.New("^[a-zA-Z0-9]+$")

// ValidateRemoteAddr validates if the input is a valid IP or a valid hostname.
// It validates the hostname by attempting to resolve it.
func ValidateRemoteAddr(raddr string) bool {
	host, port, err := net.SplitHostPort(raddr)

	if err == nil {
		raddr = host
		_, err := strconv.Atoi(port)
		// return false if port is not an int
		if err != nil {
			return false
		}
	}

	// Check if the string contains a username (e.g. git@example.com); if so validate username
	fragments := strings.Split(raddr, "@")
	// raddr contains more than one `@`
	if len(fragments) > 2 {
		return false
	}
	// raddr contains exactly one `@`
	if len(fragments) == 2 {
		user := fragments[0]

		if match := userRegex.MatchString(user); !match {
			return false
		}

		// Set raddr to host minus the user
		raddr = fragments[1]
	}

	validIP := net.ParseIP(raddr) != nil
	validHost := true

	_, err = net.LookupHost(raddr)

	if err != nil {
		// we cannot resolve the addr
		validHost = false
	}

	return validIP || validHost
}

// maxPasswordRunes is the maximum number of UTF-8 runes that a password can contain.
// This safety limit is to protect us from a DDOS attack caused by hashing very large passwords on Sourcegraph.com.
const maxPasswordRunes = 256

// ValidatePassword: Validates that a password meets the required criteria
func ValidatePassword(passwd string) error {
	if conf.PasswordPolicyEnabled() {
		return validatePasswordUsingPolicy(passwd)
	}

	return validatePasswordUsingDefaultMethod(passwd)
}

// This is the default method using our current standard
func validatePasswordUsingDefaultMethod(passwd string) error {
	// Check for blank password
	if passwd == "" {
		return errcode.NewPresentationError("Your password may not be empty.")
	}

	// Check for minimum/maximum length only
	pwLen := utf8.RuneCountInString(passwd)
	minPasswordRunes := conf.AuthMinPasswordLength()

	if pwLen < minPasswordRunes ||
		pwLen > maxPasswordRunes {
		return errcode.NewPresentationError(fmt.Sprintf("Your password may not be less than %d or be more than %d characters.", minPasswordRunes, maxPasswordRunes))
	}

	return nil
}

// This validates the password using the Password Policy configured
func validatePasswordUsingPolicy(passwd string) error {
	chars := 0
	numbers := false
	upperCase := false
	special := 0

	for _, c := range passwd {
		switch {
		case unicode.IsNumber(c):
			numbers = true
			chars++
		case unicode.IsUpper(c):
			upperCase = true
			chars++
		case unicode.IsPunct(c) || unicode.IsSymbol(c):
			special++
			chars++
		case unicode.IsLetter(c) || c == ' ':
			chars++
		default:
			// ignore
		}
	}
	// Check for blank password
	if chars == 0 {
		return errors.New("password empty")
	}

	// Get a reference to the password policy
	policy := conf.AuthPasswordPolicy()

	// Minimum Length Check
	if chars < policy.MinimumLength {
		return errcode.NewPresentationError(fmt.Sprintf("Your password may not be less than %d characters.", policy.MinimumLength))
	}

	// Maximum Length Check
	if chars > maxPasswordRunes {
		return errcode.NewPresentationError(fmt.Sprintf("Your password may not be more than %d characters.", maxPasswordRunes))
	}

	// Numeric Check
	if policy.RequireAtLeastOneNumber {
		if !numbers {
			return errcode.NewPresentationError("Your password must include one number.")
		}
	}

	// Mixed case check
	if policy.RequireUpperandLowerCase {
		if !upperCase {
			return errcode.NewPresentationError("Your password must include one uppercase letter.")
		}
	}

	// Special Character Check
	if policy.NumberOfSpecialCharacters > 0 {
		if special < policy.NumberOfSpecialCharacters {
			return errcode.NewPresentationError(fmt.Sprintf("Your password must include at least %d special character(s).", policy.NumberOfSpecialCharacters))
		}
	}

	// All good return
	return nil
}
