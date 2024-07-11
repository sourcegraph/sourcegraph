package release

import (
	"context"
	"fmt"
	"github.com/Masterminds/semver"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/lib/output"
	"github.com/urfave/cli/v2"
	"os/exec"
	"strings"

	"github.com/sourcegraph/run"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/execute"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/repo"
)

func genMigrationGraph(ctx context.Context, newVersion string) error {
	err := run.Cmd(ctx, "comby", "-in-place", "\"const maxVersionString = :[1]\"", fmt.Sprintf("\"const maxVersionString = \"%d\"\"", newVersion), "internal/database/migration/shared/data/cmd/generator/consts.go").Run()
	if err != nil {
		return errors.Wrap(err, "Could not run comby to change maxVersionString")
	}
	err = run.Cmd(ctx, "git", "add", "./internal/database/migration/shared/data/cmd/generator/consts.go").Run()
	if err != nil {
		return errors.Wrap(err, "Could not git add file")
	}
	err = run.Cmd(ctx, "git", "commit", "-m", "Update maxVersionString").Run()
	if err != nil {
		return errors.Wrap(err, "Could not git commit file")
	}
	err = run.Cmd(ctx, "git", "archive", "--format=tar.gz", "HEAD", "migrations", ">", fmt.Sprintf("migrations-v%d.tar.gz", newVersion)).Run()
	if err != nil {
		return errors.Wrap(err, "Could not create git archive")
	}
	err = run.Cmd(ctx, "CLOUDSDK_CORE_PROJECT=\"sourcegraph-ci\"", "gsutil", "cp", fmt.Sprintf("migrations-v%d", newVersion), "gs://schemas-migrations/migrations/").Run()
	if err != nil {
		return errors.Wrap(err, "Could not push git archive to GCS")
	}
	return nil
}

func cutReleaseBranch(cctx *cli.Context) error {
	p := std.Out.Pending(output.Styled(output.StylePending, "Checking for GitHub CLI..."))
	ghPath, err := exec.LookPath("gh")
	if err != nil {
		p.Destroy()
		return errors.Wrap(err, "GitHub CLI (https://cli.github.com/) is required for installation")
	}
	p.Complete(output.Linef(output.EmojiSuccess, output.StyleSuccess, "Using GitHub CLI at %q", ghPath))

	var version string
	if cctx.String("version") == "auto" {
		var err error
		version, err = determineNextReleaseVersion(cctx.Context)
		if err != nil {
			return err
		}
	} else {
		// Normalize the version string, to prevent issues where this was given with the wrong convention
		// which requires a full rebuild.
		version = fmt.Sprintf("v%s", strings.TrimPrefix(cctx.String("version"), "v"))
	}
	v, err := semver.NewVersion(version)
	if err != nil {
		return errors.Newf("invalid version %q, must be semver", version)
	}

	releaseBranch := v.String()
	defaultBranch := cctx.String("branch")

	ctx := cctx.Context
	releaseGitRepoBranch := repo.NewGitRepo(releaseBranch, releaseBranch)
	defaultGitRepoBranch := repo.NewGitRepo(defaultBranch, defaultBranch)

	if ok, err := defaultGitRepoBranch.IsDirty(ctx); err != nil {
		return errors.Wrap(err, "check if current branch is dirty")
	} else if ok {
		return errors.Newf("current branch is dirty. please commit your unstaged changes")
	}

	p = std.Out.Pending(output.Styled(output.StylePending, "Checking if the release branch exists locally ..."))
	if ok, err := releaseGitRepoBranch.HasLocalBranch(ctx); err != nil {
		p.Destroy()
		return errors.Wrapf(err, "checking if %q branch exists localy", releaseBranch)
	} else if ok {
		p.Destroy()
		return errors.Newf("branch %q exists locally", releaseBranch)
	}
	p.Complete(output.Linef(output.EmojiSuccess, output.StyleSuccess, "Release branch %q does not exist locally", releaseBranch))

	p = std.Out.Pending(output.Styled(output.StylePending, "Checking if the release branch exists in remote ..."))
	if ok, err := releaseGitRepoBranch.HasRemoteBranch(ctx); err != nil {
		p.Destroy()
		return errors.Wrapf(err, "checking if %q branch exists in remote repo", releaseBranch)
	} else if ok {
		p.Destroy()
		return errors.Newf("release branch %q exists in remote repo", releaseBranch)
	}
	p.Complete(output.Linef(output.EmojiSuccess, output.StyleSuccess, "Release branch %q does not exist in remote", releaseBranch))

	p = std.Out.Pending(output.Styled(output.StylePending, "Checking if the default branch is up to date with remote ..."))
	if _, err := defaultGitRepoBranch.FetchOrigin(ctx); err != nil {
		p.Destroy()
		return errors.Wrapf(err, "fetching origin for %q", defaultBranch)
	}

	if err := defaultGitRepoBranch.Checkout(ctx); err != nil {
		p.Destroy()
		return errors.Wrapf(err, "checking out %q", defaultBranch)
	}

	if ok, err := defaultGitRepoBranch.IsOutOfSync(ctx); err != nil {
		p.Destroy()
		return errors.Wrapf(err, "checking if %q branch is up to date with remote", defaultBranch)
	} else if ok {
		p.Destroy()
		return errors.Newf("local branch %q is not up to date with remote", defaultBranch)
	}
	p.Complete(output.Linef(output.EmojiSuccess, output.StyleSuccess, "Local branch is up to date with remote"))

	p = std.Out.Pending(output.Styled(output.StylePending, "Creating release branch..."))
	if err := releaseGitRepoBranch.CheckoutNewBranch(ctx); err != nil {
		p.Destroy()
		return errors.Wrap(err, "failed to create release branch")
	}
	p.Complete(output.Linef(output.EmojiSuccess, output.StyleSuccess, "Release branch %q created", releaseBranch))

	p = std.Out.Pending(output.Styled(output.StylePending, "Pushing release branch..."))
	if _, err := releaseGitRepoBranch.Push(ctx); err != nil {
		p.Destroy()
		return errors.Wrap(err, "failed to push release branch")
	}
	p.Complete(output.Linef(output.EmojiSuccess, output.StyleSuccess, "Release branch %q pushed", releaseBranch))

	p = std.Out.Pending(output.Styled(output.StylePending, "Creating backport label..."))
	if _, err := execute.GH(
		ctx,
		"label",
		"create",
		fmt.Sprintf("backport %s", releaseBranch),
		"-d",
		fmt.Sprintf("label used to backport PRs to the %s release branch", releaseBranch),
	); err != nil {
		p.Destroy()
		return errors.Wrap(err, "failed to create backport label")
	}
	p.Complete(output.Linef(output.EmojiSuccess, output.StyleSuccess, "Backport label created"))

	return nil
}
