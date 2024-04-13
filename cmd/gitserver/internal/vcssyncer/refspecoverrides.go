package vcssyncer

import (
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/env"
)

// HACK(keegancsmith) workaround to experiment with cloning less in a large
// monorepo. https://github.com/sourcegraph/customer/issues/19
var refspecOverridesEnv = strings.Fields(env.Get("SRC_GITSERVER_REFSPECS", "", "EXPERIMENTAL: override refspec we fetch. Space separated."))

// HACK(keegancsmith) workaround to experiment with cloning less in a large
// monorepo. https://github.com/sourcegraph/customer/issues/19
func useRefspecOverrides() bool {
	return len(refspecOverridesEnv) > 0
}

// HACK(keegancsmith) workaround to experiment with cloning less in a large
// monorepo. https://github.com/sourcegraph/customer/issues/19
func refspecOverrides() []string {
	return append([]string{}, refspecOverridesEnv...)
}
