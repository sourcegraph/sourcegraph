package dotcom

import (
	"strconv"

	"github.com/sourcegraph/sourcegraph/internal/env"
)

var sourcegraphDotComMode, _ = strconv.ParseBool(env.Get("SOURCEGRAPHDOTCOM_MODE", "false", "run as Sourcegraph.com, with add'l marketing and redirects"))

// SourcegraphDotComMode is true if this server is running Sourcegraph.com
// (solely by checking the SOURCEGRAPHDOTCOM_MODE env var). Sourcegraph.com shows
// additional marketing and sets up some additional redirects.
func SourcegraphDotComMode() bool {
	return sourcegraphDotComMode
}

type TB interface {
	Cleanup(func())
}

// MockSourcegraphDotComMode is used by tests to mock the result of SourcegraphDotComMode.
func MockSourcegraphDotComMode(t TB, value bool) {
	orig := sourcegraphDotComMode
	sourcegraphDotComMode = value
	t.Cleanup(func() { sourcegraphDotComMode = orig })
}
