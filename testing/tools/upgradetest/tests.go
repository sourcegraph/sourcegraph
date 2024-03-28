package main

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver"

	"github.com/sourcegraph/run"

	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// standardUpgradeTest initializes Sourcegraph's dbs and runs a standard upgrade
// i.e. an upgrade test between some last minor version and the current release candidate
func standardUpgradeTest(ctx context.Context, initVersion, targetVersion, latestStableVersion *semver.Version) Test {
	postRelease := ctx.Value(postReleaseKey{}).(string)

	//start test env
	test, networkName, dbs, cleanup, err := setupTestEnv(ctx, "standard", initVersion)
	if err != nil {
		test.AddError(errors.Newf("🚨 failed to setup env: %w", err))
		cleanup()
		return test
	}
	defer cleanup()

	// Use the latest stable migrator for a pre release test, and the target version migrator if testing a released version
	var migratorImage string
	if postRelease != "" {
		migratorImage = fmt.Sprintf("sourcegraph/migrator:%s", postRelease)
	} else {
		migratorImage = fmt.Sprintf("sourcegraph/migrator:%s", latestStableVersion.String())
	}

	// ensure env correctly initialized
	if err := validateDBs(ctx, &test, initVersion.String(), migratorImage, networkName, dbs, false); err != nil {
		test.AddError(errors.Newf("🚨 Upgrade failed: %w", err))
		return test
	}

	test.AddLog("-- ⚙️  performing standard upgrade")

	if postRelease != "" {
		migratorImage = fmt.Sprintf("sourcegraph/migrator:%s", postRelease)
	} else {
		migratorImage = "migrator:candidate"
	}
	// Run standard upgrade via migrators "up" command
	out, err := run.Cmd(ctx, dockerMigratorBaseString(test, "up", migratorImage, networkName, dbs)...).Run().String()
	if err != nil {
		test.AddError(errors.Newf("🚨 failed to upgrade: %w", err))
		cleanup()
		return test
	}
	test.AddLog(out)

	// Start frontend with candidate
	var cleanFrontend func()
	if postRelease != "" {
		cleanFrontend, err = startFrontend(ctx, test, "sourcegraph/frontend", postRelease, networkName, false, dbs)
	} else {
		cleanFrontend, err = startFrontend(ctx, test, "frontend", "candidate", networkName, false, dbs)
	}
	if err != nil {
		test.AddError(errors.Newf("🚨 candidate frontend error: %w", err))
		cleanFrontend()
		return test
	}
	defer cleanFrontend()

	test.AddLog("-- ⚙️  post upgrade validation")
	// Validate the upgrade
	if postRelease != "" {
		migratorImage = fmt.Sprintf("sourcegraph/migrator:%s", postRelease)
	} else {
		migratorImage = "migrator:candidate"
	}
	if err := validateDBs(ctx, &test, targetVersion.String(), migratorImage, networkName, dbs, true); err != nil {
		test.AddError(errors.Newf("🚨 Upgrade failed: %w", err))
		return test
	}

	return test
}

// multiversionUpgradeTest tests the migrator upgrade command,
// initializing the three main dbs and conducting an upgrade to the release candidate version
func multiversionUpgradeTest(ctx context.Context, initVersion, targetVersion, latestStableVersion *semver.Version) Test {
	postRelease := ctx.Value(postReleaseKey{}).(string) // Post release version string

	//start test env
	test, networkName, dbs, cleanup, err := setupTestEnv(ctx, "multiversion", initVersion)
	if err != nil {
		fmt.Println("🚨 failed to setup env: ", err)
		cleanup()
		return test
	}
	defer cleanup()

	// Use the latest stable migrator for a pre release test, and the target version migrator if testing a released version
	var migratorImage string
	if postRelease != "" {
		migratorImage = fmt.Sprintf("sourcegraph/migrator:%s", postRelease)
	} else {
		migratorImage = fmt.Sprintf("sourcegraph/migrator:%s", latestStableVersion.String())
	}

	// ensure env correctly initialized
	if err := validateDBs(ctx, &test, initVersion.String(), migratorImage, networkName, dbs, false); err != nil {
		test.AddError(errors.Newf("🚨 Initializing env in multiversion test failed: %w", err))
		return test
	}

	// Run multiversion upgrade using candidate image unless a post release version is specified, in which case use that image
	//
	// If the build version in the test has been stamped this is the "to" input for the upgrade command. If the builds arent stamped we use the latest stable release verstion,
	// as the target for `migrator upgrade`. This is safe because we assume the target version of the test is always the latest release version. Therefore `migrator up` should always be a single minor version away.
	var toVersion string
	if targetVersion.String() != "0.0.0+dev" { // if version is stamped
		toVersion = targetVersion.String()
	} else {
		toVersion = latestStableVersion.String()
	}
	test.AddLog(fmt.Sprintf("-- ⚙️  performing multiversion upgrade (--from %s --to %s)", initVersion.String(), toVersion))
	if postRelease != "" {
		migratorImage = fmt.Sprintf("sourcegraph/migrator:%s", postRelease)
	} else {
		migratorImage = "migrator:candidate"
	}
	out, err := run.Cmd(ctx,
		dockerMigratorBaseString(test, fmt.Sprintf("upgrade --from %s --to %s --ignore-migrator-update", initVersion.String(), toVersion), migratorImage, networkName, dbs)...).
		Run().String()
	if err != nil {
		test.AddError(errors.Newf("🚨 failed to upgrade: %w", err))
		cleanup()
		return test
	}
	test.AddLog(out)

	// Run migrator up with migrator candidate to apply any patch migrations defined on the candidate version, unless a post release version is specified
	out, err = run.Cmd(ctx,
		dockerMigratorBaseString(test, "up", migratorImage, networkName, dbs)...).
		Run().String()
	if err != nil {
		test.AddError(errors.Newf("🚨 failed to upgrade: %w", err))
		cleanup()
		return test
	}
	test.AddLog(out)

	// Start frontend with candidate unless a post release version is specified
	var cleanFrontend func()
	if postRelease != "" {
		cleanFrontend, err = startFrontend(ctx, test, "sourcegraph/frontend", postRelease, networkName, false, dbs)
	} else {
		cleanFrontend, err = startFrontend(ctx, test, "frontend", "candidate", networkName, false, dbs)
	}
	if err != nil {
		test.AddError(errors.Newf("🚨 candidate frontend error: %w", err))
		cleanFrontend()
		return test
	}
	defer cleanFrontend()

	test.AddLog("-- ⚙️  post upgrade validation")
	// Validate the upgrade
	if err := validateDBs(ctx, &test, targetVersion.String(), migratorImage, networkName, dbs, true); err != nil {
		test.AddError(errors.Newf("🚨 Upgrade failed: %w", err))
		return test
	}

	return test
}

// Logic in tryAutoUpgrade is not compatible with dev builds. The autoUpgrade test must be run with a stamp version
//
// Without this in place autoupgrade fails and exits while trying to make an oobmigration comparison here: https://sourcegraph.com/github.com/sourcegraph/sourcegraph/-/blob/cmd/frontend/internal/cli/autoupgrade.go?L67-76
// {"SeverityText":"WARN","Timestamp":1706721478276103721,"InstrumentationScope":"frontend","Caller":"cli/autoupgrade.go:73","Function":"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/cli.tryAutoUpgrade","Body":"unexpected string for desired instance schema version, skipping auto-upgrade","Resource":{"service.name":"frontend","service.version":"devVersion","service.instance.id":"487754e1c54a"},"Attributes":{"version":"devVersion"}}
func autoUpgradeTest(ctx context.Context, initVersion, targetVersion, latestStableVersion *semver.Version) Test {
	postRelease := ctx.Value(postReleaseKey{}).(string) // Post release version string

	//start test env
	test, networkName, dbs, cleanup, err := setupTestEnv(ctx, "auto", initVersion)
	if err != nil {
		test.AddError(errors.Newf("failed to setup env: %w", err))
		cleanup()
		return test
	}
	defer cleanup()

	// Use the latest stable migrator for a pre release test, and the target version migrator if testing a released version
	var migratorImage string
	if postRelease != "" {
		migratorImage = fmt.Sprintf("sourcegraph/migrator:%s", postRelease)
	} else {
		migratorImage = fmt.Sprintf("sourcegraph/migrator:%s", latestStableVersion.String())
	}

	// ensure env correctly initialized
	// this allows us to avoid issues from changes in migrators invocation
	if err := validateDBs(ctx, &test, initVersion.String(), migratorImage, networkName, dbs, false); err != nil {
		test.AddError(errors.Newf("🚨 Initializing env in autoupgrade test failed: %w", err))
		return test
	}

	// Set SRC_AUTOUPGRADE=true on Migrator and Frontend containers. Then start the frontend container.
	test.AddLog("-- ⚙️  performing auto upgrade")

	// Start frontend with candidate
	var cleanFrontend func()
	if postRelease != "" {
		cleanFrontend, err = startFrontend(ctx, test, "sourcegraph/frontend", postRelease, networkName, true, dbs)
	} else {
		cleanFrontend, err = startFrontend(ctx, test, "frontend", "candidate", networkName, true, dbs)
	}
	if err != nil {
		test.AddError(errors.Newf("🚨 candidate frontend error: %w", err))
		cleanFrontend()
		return test
	}
	defer cleanFrontend()

	test.AddLog("-- ⚙️  post upgrade validation")
	// Validate the upgrade
	if postRelease != "" {
		migratorImage = fmt.Sprintf("sourcegraph/migrator:%s", postRelease)
	} else {
		migratorImage = "migrator:candidate"
	}
	if err := validateDBs(ctx, &test, targetVersion.String(), migratorImage, networkName, dbs, true); err != nil {
		test.AddError(errors.Newf("🚨 Upgrade failed: %w", err))
		return test
	}

	return test
}
