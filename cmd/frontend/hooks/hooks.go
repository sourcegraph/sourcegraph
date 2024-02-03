// Package hooks allow hooking into the frontend.
package hooks

import (
	"net/http"
)

// PostAuthMiddleware is an HTTP handler middleware that, if set, runs just before auth-related
// middleware. The client is authenticated when PostAuthMiddleware is called.
var PostAuthMiddleware func(http.Handler) http.Handler

// FeatureBatchChanges describes if and how the Batch Changes feature is available on
// the given license plan. It mirrors the type licensing.FeatureBatchChanges.
type FeatureBatchChanges struct {
	// If true, there is no limit to the number of changesets that can be created.
	Unrestricted bool `json:"unrestricted"`
	// Maximum number of changesets that can be created per batch change.
	// If Unrestricted is true, this is ignored.
	MaxNumChangesets int `json:"maxNumChangesets"`
}

// LicenseFeatures contains information about licensed features that are
// enabled/disabled on the current license.
type LicenseFeatures struct {
	CodeSearch     bool `json:"codeSearch"`
	Cody           bool `json:"cody"`
	AllowAirGapped bool `json:"allowAirGapped"`
}

// LicenseInfo contains non-sensitive information about the legitimate usage of the
// current license on the instance. It is technically accessible to all users, so only
// include information that is safe to be seen by others.
type LicenseInfo struct {
	CurrentPlan string `json:"currentPlan"`

	CodeScaleLimit         string               `json:"codeScaleLimit"`
	CodeScaleCloseToLimit  bool                 `json:"codeScaleCloseToLimit"`
	CodeScaleExceededLimit bool                 `json:"codeScaleExceededLimit"`
	KnownLicenseTags       []string             `json:"knownLicenseTags"`
	BatchChanges           *FeatureBatchChanges `json:"batchChanges"`

	Features LicenseFeatures `json:"features"`
}

var GetLicenseInfo = func() *LicenseInfo { return nil }
