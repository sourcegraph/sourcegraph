package janitor

import (
	"context"
	"fmt"
	"time"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/git"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/janitor/stats"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// ValidateRepacking validates the input repacking config. This function any validating error and if the configuration
// is for full repack.
func ValidateRepacking(cfg RepackObjectsConfig) (bool, error) {
	var isFullRepack bool
	switch cfg.Strategy {
	case RepackObjectsStrategyIncrementalWithUnreachable:
		isFullRepack = false
		if cfg.WriteBitmap {
			return false, errors.New("cannot write packfile bitmap for an incremental repack")
		}
		if cfg.WriteMultiPackIndex {
			return false, errors.New("cannot write multi-pack index for an incremental repack")
		}
	case RepackObjectsStrategyGeometric:
		isFullRepack = false
	case RepackObjectsStrategyFullWithCruft:
		isFullRepack = true
	default:
		return false, errors.Newf("invalid strategy: %q", cfg.Strategy)
	}

	if !isFullRepack && !cfg.WriteMultiPackIndex && cfg.WriteBitmap {
		return false, errors.New("cannot write packfile bitmap for an incremental repack")
	}
	if cfg.Strategy != RepackObjectsStrategyFullWithCruft && !cfg.CruftExpireBefore.IsZero() {
		return isFullRepack, errors.New("cannot expire cruft objects when not writing cruft packs")
	}

	return isFullRepack, nil
}

// RepackObjects repacks objects in the given repository and updates the commit-graph. The way
// objects are repacked is determined via the RepackObjectsConfig.
func RepackObjects(ctx context.Context, backend git.GitBackend, repo common.GitDir, cfg RepackObjectsConfig) error {
	isFullRepack, err := ValidateRepacking(cfg)
	if err != nil {
		return err
	}

	if isFullRepack {
		// When we have performed a full repack we're updating the "full-repack-timestamp"
		// file. This is done so that we can tell when we have last performed a full repack
		// in a repository. This information can be used by our heuristics to effectively
		// rate-limit the frequency of full repacks.
		//
		// Note that we write the file _before_ actually writing the new pack, which means
		// that even if the full repack fails, we would still pretend to have done it. This
		// is done intentionally, as the likelihood for huge repositories to fail during a
		// full repack is comparatively high. So if we didn't update the timestamp in case
		// of a failure we'd potentially busy-spin trying to do a full repack.
		//
		// TODO: Verify that after a initial repack to generate MIDXes this will be set
		// correctly, I saw some case locally where the next janitor run repacked objects
		// right away becayse "last full repack is long ago".
		if err := stats.UpdateFullRepackTimestamp(repo, time.Now()); err != nil {
			return fmt.Errorf("updating full-repack timestamp: %w", err)
		}
	}

	switch cfg.Strategy {
	case RepackObjectsStrategyIncrementalWithUnreachable:
		// Pack all loose objects into a new packfile, regardless of their reachability.
		// There is no git-repack(1) mode that would allow us to do this, so we have to
		// instead do it ourselves.
		//
		// Note: we explicitly do not pass `GetRepackGitConfig()` here as none of
		// its options apply to this kind of repack: we have no delta islands given
		// that we do not walk the revision graph, and we won't ever write bitmaps.
		if err := backend.PackObjects(ctx); err != nil {
			return err
		}

		// The `-d` switch of git-repack(1) handles deletion of objects that have just been
		// packed into a new packfile. As we pack objects ourselves, we have to manually
		// ensure that packed loose objects are deleted.
		if err := backend.PrunePacked(ctx); err != nil {
			return err
		}

		return nil
	case RepackObjectsStrategyFullWithCruft:
		return backend.Repack(ctx, git.RepackOptions{
			Cruft:           true,
			CruftExpiration: cfg.CruftExpireBefore,
			// Make sure to delete loose objects and packfiles that are made obsolete
			// by the new packfile.
			DeleteLoose: true,
			// Don't include objects part of an alternate.
			Local:               true,
			WriteMultiPackIndex: cfg.WriteMultiPackIndex,
			WriteBitmap:         cfg.WriteBitmap,
		})
	case RepackObjectsStrategyGeometric:
		return PerformGeometricRepacking(ctx, backend, cfg)
	}
	return nil
}

// PerformGeometricRepacking performs geometric repacking task using git-repack(1) command. It allows us to merge
// multiple packfiles without having to rewrite all packfiles into one. This new "geometric" strategy tries to ensure
// that existing packfiles in the repository form a geometric sequence where each successive packfile contains at least
// n times as many objects as the preceding packfile. If the sequence isn't maintained, Git will determine a slice of
// packfiles that it must repack to maintain the sequence again. With this process, we can limit the number of packfiles
// that exist in the repository without having to repack all objects into a single packfile regularly.
// This repacking does not take reachability into account.
// For more information, https://about.gitlab.com/blog/2023/11/02/rearchitecting-git-object-database-mainentance-for-scale/#geometric-repacking
func PerformGeometricRepacking(ctx context.Context, backend git.GitBackend, cfg RepackObjectsConfig) error {
	return backend.Repack(ctx, git.RepackOptions{
		Geometric: true,
		// Make sure to delete loose objects and packfiles that are made obsolete
		// by the new packfile.
		DeleteLoose: true,
		// Don't include objects part of an alternate.
		Local:               true,
		WriteMultiPackIndex: cfg.WriteMultiPackIndex,
		WriteBitmap:         cfg.WriteBitmap,
	})
}
