package licensing

import (
	"fmt"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type Feature interface {
	FeatureName() string
}

// BasicFeature is a product feature that is selectively activated based on the current license key.
type BasicFeature string

func (f BasicFeature) FeatureName() string {
	return string(f)
}

// Check checks whether the feature is activated based on the current license. If it is
// disabled, it returns a non-nil error.
//
// The returned error may implement errcode.PresentationError to indicate that it can be displayed
// directly to the user. Use IsFeatureNotActivated to distinguish between the error reasons.
func Check(feature Feature) error {
	if MockCheckFeature != nil {
		return MockCheckFeature(feature)
	}

	info, err := GetConfiguredProductLicenseInfo()
	if err != nil {
		return errors.WithMessage(err, fmt.Sprintf("checking feature %q activation", feature))
	}
	return checkFeature(info, feature)
}

// Returns whether or not the target feature is activated based on the current license.
// If the target parameter is a pointer, the plan's configured feature will be written
// to the target.
func FeatureAs(target Feature) bool {
	if MockFeatureAs != nil {
		return MockFeatureAs(target)
	}

	info, err := GetConfiguredProductLicenseInfo()
	if err != nil {
		return false
	}

	return info.Plan().HasFeature(target)
}

func checkFeature(info *Info, feature Feature) error {
	if info == nil {
		return NewFeatureNotActivatedError(fmt.Sprintf("The feature %q is not activated because it requires a valid Sourcegraph license. Purchase a Sourcegraph subscription to activate this feature.", feature))
	}

	featureTrimmed := BasicFeature(strings.TrimSpace(feature.FeatureName()))

	// Check if the feature is explicitly allowed via license tag.
	hasFeature := func(want Feature) bool {
		for _, t := range info.Tags {
			// We have been issuing licenses with trailing spaces in the tags for a while.
			// Eventually we should be able to remove these `TrimSpace` calls again,
			// as we now guard against that while generating licenses, but there
			// are quite a few "wrong" licenses out there as of today (2021-07-19).
			if BasicFeature(strings.TrimSpace(t)).FeatureName() == want.FeatureName() {
				return true
			}
		}
		return false
	}
	if !info.Plan().HasFeature(featureTrimmed) && !hasFeature(featureTrimmed) {
		return NewFeatureNotActivatedError(fmt.Sprintf("The feature %q is not activated in your Sourcegraph license. Upgrade your Sourcegraph subscription to use this feature.", feature))
	}
	return nil
}

func MockCheckFeatureError(expectedError string) {
	MockCheckFeature = func(feature Feature) error {
		if expectedError == "" {
			return nil
		}
		return errors.New(expectedError)
	}
}

// MockCheckFeature is for mocking Check in tests.
var MockCheckFeature func(feature Feature) error

// MockFeatureAs is for mocking FeatureAs in tests.
var MockFeatureAs func(target Feature) bool

// TestingSkipFeatureChecks is for tests that want to mock Check to always return nil (i.e.,
// behave as though the current license enables all features).
//
// It returns a cleanup func so callers can use `defer TestingSkipFeatureChecks()()` in a test body.
func TestingSkipFeatureChecks() func() {
	MockCheckFeature = func(feature Feature) error { return nil }
	return func() { MockCheckFeature = nil }
}

func NewFeatureNotActivatedError(message string) featureNotActivatedError {
	e := errcode.NewPresentationError(message).(errcode.PresentationError)
	return featureNotActivatedError{e}
}

type featureNotActivatedError struct{ errcode.PresentationError }

// IsFeatureNotActivated reports whether err indicates that the license is valid but does not
// activate the feature.
//
// It is used to distinguish between the multiple reasons for errors from Check: either
// failed license verification, or a valid license that does not activate a feature (e.g.,
// Enterprise Starter not including an Enterprise-only feature).
func IsFeatureNotActivated(err error) bool {
	// Also check for the pointer type to guard against stupid mistakes.
	return errors.HasType(err, featureNotActivatedError{}) || errors.HasType(err, &featureNotActivatedError{})
}

// IsFeatureEnabledLenient reports whether the current license enables the given feature. If there
// is an error reading the license, it is lenient and returns true.
//
// This is useful for callers who don't want to handle errors (usually because the user would be
// prevented from getting to this point if license verification had failed, so it's not necessary to
// handle license verification errors here).
func IsFeatureEnabledLenient(feature Feature) bool {
	return !IsFeatureNotActivated(Check(feature))
}
