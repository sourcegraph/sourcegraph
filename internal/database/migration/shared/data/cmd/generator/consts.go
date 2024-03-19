package main

import (
	"fmt"

	"github.com/sourcegraph/sourcegraph/internal/database/migration/shared/data/cmd/generator/version"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
)

// NOTE: This should be kept up-to-date with the upcoming version to be released, and bumped after.
// fallback schemas everything we support migrating to. The release tool automates this upgrade, so don't touch this :)
// This should be the last minor version since patch releases only happen in the release branch.
const maxVersionString = "5.3.0"

// MaxVersion is the highest known released version at the time the migrator was built.
var MaxVersion = func() oobmigration.Version {
	ver := maxVersionString
	if version.FinalVersionString != "dev" {
		ver = version.FinalVersionString
	}
	if oobVersion, ok := oobmigration.NewVersionFromString(ver); ok {
		return oobVersion
	}

	panic(fmt.Sprintf("malformed maxVersionString %q", ver))
}()

// MinVersion is the minimum version a migrator can support upgrading to a newer version of
// Sourcegraph.
var MinVersion = oobmigration.NewVersion(3, 20)

// FrozenRevisions are schemas at a point-in-time for which out-of-band migration unit tests
// can continue to run on their last pre-deprecation version. This code is still ran by the
// migrator, but only on a schema shape that existed in the past.
var FrozenRevisions = []string{
	"4.5.0",
}
