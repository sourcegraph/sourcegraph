package cliutil

import (
	"context"
	"flag"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/migration/runner"
	"github.com/sourcegraph/sourcegraph/internal/database/migration/schemas"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/oobmigration"
	"github.com/sourcegraph/sourcegraph/internal/version"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

type actionFunction func(ctx context.Context, cmd *cli.Context, out *output.Output) error

// makeAction creates a new migration action function. It is expected that these
// commands accept zero arguments and define their own flags.
func makeAction(outFactory OutputFactory, f actionFunction) func(cmd *cli.Context) error {
	return func(cmd *cli.Context) error {
		if cmd.NArg() != 0 {
			return flagHelp(outFactory(), "too many arguments")
		}

		return f(cmd.Context, cmd, outFactory())
	}
}

// flagHelp returns an error that prints the specified error message with usage text.
func flagHelp(out *output.Output, message string, args ...any) error {
	out.WriteLine(output.Linef("", output.StyleWarning, "ERROR: "+message, args...))
	return flag.ErrHelp
}

// setupRunner initializes and returns the runner associated witht the given schema.
func setupRunner(factory RunnerFactory, schemaNames ...string) (*runner.Runner, error) {
	r, err := factory(schemaNames)
	if err != nil {
		return nil, err
	}

	return r, nil
}

// setupStore initializes and returns the store associated witht the given schema.
func setupStore(ctx context.Context, factory RunnerFactory, schemaName string) (runner.Store, error) {
	r, err := setupRunner(factory, schemaName)
	if err != nil {
		return nil, err
	}

	store, err := r.Store(ctx, schemaName)
	if err != nil {
		return nil, err
	}

	return store, nil
}

// sanitizeSchemaNames sanitizies the given string slice from the user.
func sanitizeSchemaNames(schemaNames []string, out *output.Output) []string {
	if len(schemaNames) == 1 && schemaNames[0] == "" {
		schemaNames = nil
	}

	if len(schemaNames) == 1 && schemaNames[0] == "all" {
		return schemas.SchemaNames
	}

	for i, name := range schemaNames {
		schemaNames[i] = TranslateSchemaNames(name, out)
	}

	return schemaNames
}

var dbNameToSchema = map[string]string{
	"pgsql":           "frontend",
	"codeintel-db":    "codeintel",
	"codeinsights-db": "codeinsights",
}

// TranslateSchemaNames translates a string with potentially the value of the service/container name
// of the db schema the user wants to operate on into the schema name.
func TranslateSchemaNames(name string, out *output.Output) string {
	// users might input the name of the service e.g. pgsql instead of frontend, so we
	// translate to what it actually should be
	if translated, ok := dbNameToSchema[name]; ok {
		out.WriteLine(output.Linef(output.EmojiInfo, output.StyleGrey, "Translating container/service name %q to schema name %q", name, translated))
		name = translated
	}

	return name
}

// parseTargets parses the given strings as integers.
func parseTargets(targets []string) ([]int, error) {
	if len(targets) == 1 && targets[0] == "" {
		targets = nil
	}

	versions := make([]int, 0, len(targets))
	for _, target := range targets {
		version, err := strconv.Atoi(target)
		if err != nil {
			return nil, err
		}

		versions = append(versions, version)
	}

	return versions, nil
}

// getPivilegedModeFromFlags transforms the given flags into an equivalent PrivilegedMode value. A user error is
// returned if the supplied flags form an invalid state.
func getPivilegedModeFromFlags(cmd *cli.Context, out *output.Output, unprivilegedOnlyFlag, noopPrivilegedFlag *cli.BoolFlag) (runner.PrivilegedMode, error) {
	unprivilegedOnly := unprivilegedOnlyFlag.Get(cmd)
	noopPrivileged := noopPrivilegedFlag.Get(cmd)
	if unprivilegedOnly && noopPrivileged {
		return runner.InvalidPrivilegedMode, flagHelp(out, "-unprivileged-only and -noop-privileged are mutually exclusive")
	}

	if unprivilegedOnly {
		return runner.RefusePrivilegedMigrations, nil
	}
	if noopPrivileged {
		return runner.NoopPrivilegedMigrations, nil
	}

	return runner.ApplyPrivilegedMigrations, nil
}

var migratorObservationCtx = &observation.TestContext

func outOfBandMigrationRunner(db database.DB) *oobmigration.Runner {
	return oobmigration.NewRunnerWithDB(migratorObservationCtx, db, time.Second)
}

// checks if a known good version's schema can be reached through either Github
// or GCS, to report whether the migrator may be operating in an airgapped environment.
func isAirgapped(ctx context.Context) (err error) {
	// known good version and filename in both GCS and Github
	filename, _ := schemas.GetSchemaJSONFilename("frontend")
	const version = "v3.41.1"

	timedCtx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	url := schemas.GithubExpectedSchemaPath(filename, version)
	req, _ := http.NewRequestWithContext(timedCtx, http.MethodHead, url, nil)
	resp, gherr := http.DefaultClient.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	ghUnreachable := gherr != nil || resp.StatusCode != http.StatusOK

	timedCtx, cancel = context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	url = schemas.GcsExpectedSchemaPath(filename, version)
	req, _ = http.NewRequestWithContext(timedCtx, http.MethodHead, url, nil)
	resp, gcserr := http.DefaultClient.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	gcsUnreachable := gcserr != nil || resp.StatusCode != http.StatusOK

	switch {
	case ghUnreachable && gcsUnreachable:
		err = errors.New("Neither Github nor GCS reachable, some features may not work as expected")
	case ghUnreachable:
		err = errors.New("Github not reachable, GCS is reachable, some features may not work as expected")
	case gcsUnreachable:
		err = errors.New("Github is reachable, GCS not reachable, some features may not work as expected")
	}

	return err
}

func checkForMigratorUpdate(ctx context.Context) (latest string, hasUpdate bool, err error) {
	migratorVersion, migratorPatch, ok := oobmigration.NewVersionAndPatchFromString(version.Version())
	if !ok || migratorVersion.Dev {
		return "", false, nil
	}

	timedCtx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	req, _ := http.NewRequestWithContext(timedCtx, http.MethodHead, "https://github.com/sourcegraph/sourcegraph/releases/latest", nil)
	resp, err := (&http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}).Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		return "", false, errors.Newf("unexpected status code %d", resp.StatusCode)
	}

	location, err := resp.Location()
	if err != nil {
		return "", false, err
	}

	pathParts := strings.Split(location.Path, "/")
	if len(pathParts) == 0 {
		return "", false, errors.Newf("empty path in Location header URL: %s", location.String())
	}
	latest = pathParts[len(pathParts)-1]

	latestVersion, latestPatch, ok := oobmigration.NewVersionAndPatchFromString(latest)
	if !ok {
		return "", false, errors.Newf("last section in path is an invalid format: %s", latest)
	}

	isMigratorOutOfDate := oobmigration.CompareVersions(latestVersion, migratorVersion) == oobmigration.VersionOrderBefore || (latestVersion.Minor == migratorVersion.Minor && latestPatch > migratorPatch)

	return latest, isMigratorOutOfDate, nil
}
