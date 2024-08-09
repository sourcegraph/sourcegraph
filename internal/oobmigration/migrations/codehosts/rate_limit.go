package codehosts

import (
	"fmt"

	"golang.org/x/time/rate"

	"github.com/sourcegraph/sourcegraph/internal/oobmigration/migrations/codehosts/schema"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// NOTE
// This code is largely copy-pasta from internal/extsvc/types.go, because we
// do not want to import internal packages into the OOB migrations where possible.
// This will allow OOB migrations to still work even if internal packages change in
// incompatible ways for multi version upgrades from versions many many releases ago.

// =======================================================================
// =======================================================================
// =======================================================================
// =======================================================================

// extractRateLimit extracts the rate limit from the given args. If rate limiting is not
// supported the error returned will be an ErrRateLimitUnsupported.
func extractRateLimit(config, kind string) (limit rate.Limit, isDefault bool, err error) {
	parsed, err := parseConfig(kind, config)
	if err != nil {
		return rate.Inf, false, errors.Wrap(err, "loading service configuration")
	}

	rlc, isDefault, err := getLimitFromConfig(kind, parsed)
	if err != nil {
		return rate.Inf, false, err
	}

	return rlc, isDefault, nil
}

// getLimitFromConfig gets RateLimitConfig from an already parsed config schema.
func getLimitFromConfig(kind string, config any) (limit rate.Limit, isDefault bool, err error) {
	// Rate limit config can be in a few states:
	// 1. Not defined: Some infinite, some limited, depending on code host.
	// 2. Defined and enabled: We use their defined limit.
	// 3. Defined and disabled: We use an infinite limiter.

	isDefault = true
	switch c := config.(type) {
	case *schema.GitLabConnection:
		limit = rate.Inf
		if c != nil && c.RateLimit != nil {
			isDefault = false
			limit = limitOrInf(c.RateLimit.Enabled, c.RateLimit.RequestsPerHour)
		}
	case *schema.GitHubConnection:
		// Use an infinite rate limiter. GitHub has an external rate limiter we obey.
		limit = rate.Inf
		if c != nil && c.RateLimit != nil {
			isDefault = false
			limit = limitOrInf(c.RateLimit.Enabled, c.RateLimit.RequestsPerHour)
		}
	case *schema.BitbucketServerConnection:
		// 8/s is the default limit we enforce
		limit = rate.Limit(8)
		if c != nil && c.RateLimit != nil {
			isDefault = false
			limit = limitOrInf(c.RateLimit.Enabled, c.RateLimit.RequestsPerHour)
		}
	case *schema.BitbucketCloudConnection:
		limit = rate.Limit(2)
		if c != nil && c.RateLimit != nil {
			isDefault = false
			limit = limitOrInf(c.RateLimit.Enabled, c.RateLimit.RequestsPerHour)
		}
	case *schema.PerforceConnection:
		limit = rate.Limit(5000.0 / 3600.0)
		if c != nil && c.RateLimit != nil {
			isDefault = false
			limit = limitOrInf(c.RateLimit.Enabled, c.RateLimit.RequestsPerHour)
		}
	case *schema.JVMPackagesConnection:
		limit = rate.Limit(2)
		if c != nil && c.Maven.RateLimit != nil {
			isDefault = false
			limit = limitOrInf(c.Maven.RateLimit.Enabled, c.Maven.RateLimit.RequestsPerHour)
		}
	case *schema.PagureConnection:
		// 8/s is the default limit we enforce
		limit = rate.Limit(8)
		if c != nil && c.RateLimit != nil {
			isDefault = false
			limit = limitOrInf(c.RateLimit.Enabled, c.RateLimit.RequestsPerHour)
		}
	case *schema.NpmPackagesConnection:
		limit = rate.Limit(6000 / 3600.0) // Same as the default in npm-packages.schema.json
		if c != nil && c.RateLimit != nil {
			isDefault = false
			limit = limitOrInf(c.RateLimit.Enabled, c.RateLimit.RequestsPerHour)
		}
	case *schema.GoModulesConnection:
		// Unlike the GitHub or GitLab APIs, the public npm registry (i.e. https://proxy.golang.org)
		// doesn't document an enforced req/s rate limit AND we do a lot more individual
		// requests in comparison since they don't offer enough batch APIs.
		limit = rate.Limit(57600.0 / 3600.0) // Same as default in go-modules.schema.json
		if c != nil && c.RateLimit != nil {
			isDefault = false
			limit = limitOrInf(c.RateLimit.Enabled, c.RateLimit.RequestsPerHour)
		}
	case *schema.PythonPackagesConnection:
		// Unlike the GitHub or GitLab APIs, the pypi.org doesn't
		// document an enforced req/s rate limit.
		limit = rate.Limit(57600.0 / 3600.0) // 16/second same as default in python-packages.schema.json
		if c != nil && c.RateLimit != nil {
			isDefault = false
			limit = limitOrInf(c.RateLimit.Enabled, c.RateLimit.RequestsPerHour)
		}
	case *schema.RustPackagesConnection:
		// The crates.io CDN has no rate limits https://www.pietroalbini.org/blog/downloading-crates-io/
		limit = rate.Limit(100)
		if c != nil && c.RateLimit != nil {
			isDefault = false
			limit = limitOrInf(c.RateLimit.Enabled, c.RateLimit.RequestsPerHour)
		}
	case *schema.RubyPackagesConnection:
		// The rubygems.org API allows 10 rps https://guides.rubygems.org/rubygems-org-rate-limits/
		limit = rate.Limit(10)
		if c != nil && c.RateLimit != nil {
			isDefault = false
			limit = limitOrInf(c.RateLimit.Enabled, c.RateLimit.RequestsPerHour)
		}
	default:
		return limit, isDefault, errRateLimitUnsupported{codehostKind: kind}
	}

	return limit, isDefault, nil
}

func limitOrInf(enabled bool, perHour float64) rate.Limit {
	if enabled {
		return rate.Limit(perHour / 3600)
	}
	return rate.Inf
}

type errRateLimitUnsupported struct {
	codehostKind string
}

func (e errRateLimitUnsupported) Error() string {
	return fmt.Sprintf("internal rate limiting not supported for %s", e.codehostKind)
}
