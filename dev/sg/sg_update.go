package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/sourcegraph/sourcegraph/dev/sg/internal/category"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/repo"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/run"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/dev/sg/root"
	"github.com/sourcegraph/sourcegraph/internal/download"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

const sgReleases = "https://github.com/sourcegraph/sg/releases"

var updateCommand = &cli.Command{
	Name:  "update",
	Usage: "Update local sg installation",
	Description: fmt.Sprintf(`Update local sg installation with the latest changes. To see what's new, run:

    sg version changelog -next

A custom release from %s can be installed with the '-release' flag.`, sgReleases),
	Category: category.Util,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:    "release",
			Aliases: []string{"r"},
			Usage:   fmt.Sprintf("The name of the release in %s to update to", sgReleases),
			Value:   "latest",
		},
	},
	Action: func(cmd *cli.Context) error {
		release := cmd.String("release")
		if release != "latest" {
			if release == cmd.App.Version {
				std.Out.WriteNoticef("sg is already up to date (currently installed version: %q)",
					cmd.App.Version)
				return nil
			}

			// If user specifies non-latest release, chances are they are interested
			// in using an older revision intentionally. Let them know that they
			// may want to disable auto-updates.
			std.Out.WriteWarningf("Installing user-specified release %q - "+
				"'sg' auto-updates might update your 'sg' installation anyway, "+
				"set 'SG_SKIP_AUTO_UPDATE=false' to disable auto-updates.", release)
		}

		p := std.Out.Pending(output.StylePending.Linef("Downloading sg release %q...", release))
		if _, err := updateToPrebuiltSG(cmd.Context, release); err != nil {
			p.Destroy()
			return err
		}
		p.Complete(output.Linef(output.EmojiSuccess, output.StyleSuccess,
			"sg has been updated to %q!", release))

		std.Out.Write("To see what's new, run 'sg version changelog'.")
		return nil
	},
}

// updateToPrebuiltSG downloads the latest release of sg prebuilt binaries and install it.
func updateToPrebuiltSG(ctx context.Context, release string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/%s", sgReleases, release),
		nil)
	if err != nil {
		return false, err
	}
	// We use the RoundTripper to make an HTTP request without having to deal
	// with redirections.
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		return false, errors.Wrapf(err, "Fetch GitHub release %q", release)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return false, errors.Newf("GitHub release %q not found", release)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 400 {
		return false, errors.Newf("Fetch GitHub release %q: unexpected status code %d",
			release, resp.StatusCode)
	}

	var downloadURL string
	if release == "latest" {
		// GitHub redirects to the latest release, so we need to fetch the
		// location header to get the download URL.
		location := resp.Header.Get("location")
		if location == "" {
			return false, errors.New("GitHub latest release: empty location")
		}
		location = strings.ReplaceAll(location, "/tag/", "/download/")
		downloadURL = fmt.Sprintf("%s/sg_%s_%s", location, runtime.GOOS, runtime.GOARCH)
	} else {
		// Otherwise, we can compose the download link from the user-provided
		// release name.
		downloadURL = fmt.Sprintf("%s/download/%s/sg_%s_%s",
			sgReleases, release, runtime.GOOS, runtime.GOARCH)
	}

	currentExecPath, err := os.Executable()
	if err != nil {
		return false, err
	}
	return download.Executable(ctx, downloadURL, currentExecPath, false)
}

func checkSgVersionAndUpdate(ctx context.Context, out *std.Output, skipUpdate bool) error {
	if BuildCommit == "dev" {
		// If `sg` was built with a dirty `./dev/sg` directory it's a dev build
		// and we don't need to display this message.
		out.Verbose("Skipping update check on dev build")
		return nil
	}

	_, err := root.RepositoryRoot()
	if err != nil {
		// Ignore the error, because we only want to check the version if we're
		// in sourcegraph/sourcegraph
		return nil
	}

	rev := strings.TrimPrefix(BuildCommit, "dev-")

	// If the revision of sg is not found locally, the user has likely not run 'git fetch'
	// recently, and we can skip the version check for now.
	if !repo.HasCommit(ctx, rev) {
		out.VerboseLine(output.Styledf(output.StyleWarning,
			"current sg version %s not found locally - you may want to run 'git fetch origin main'.", rev))
		return nil
	}

	// Check for new commits since the current build of 'sg'
	revList, err := run.GitCmd("rev-list", fmt.Sprintf("%s..origin/main", rev), "--", "./dev/sg")
	if err != nil {
		// Unexpected error occured
		return err
	}
	revList = strings.TrimSpace(revList)
	if revList == "" {
		// No newer commits found. sg is up to date.
		return nil
	}

	if skipUpdate {
		out.WriteLine(output.Styled(output.StyleSearchMatch, "╭───────────────────────────────────────────────────────────────────────╮"))
		out.WriteLine(output.Styled(output.StyleSearchMatch, "│ HEY! A new version of sg is available. Run 'sg update' to install it. │"))
		out.WriteLine(output.Styled(output.StyleSearchMatch, "│         To see what's new, run 'sg version changelog -next'.          │"))
		out.WriteLine(output.Styled(output.StyleSearchMatch, "╰───────────────────────────────────────────────────────────────────────╯"))

		return nil
	}

	out.WriteLine(output.Line(output.EmojiInfo, output.StyleSuggestion, "Auto updating sg ..."))
	updated, err := updateToPrebuiltSG(ctx, "latest") // always install latest when auto-updating
	if err != nil {
		return errors.Newf("failed to install update: %s", err)
	}
	if !updated {
		return nil
	}

	out.WriteSuccessf("sg has been updated!")
	out.Write("To see what's new, run 'sg version changelog'.")
	return nil
}
