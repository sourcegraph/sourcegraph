package database

import (
	"context"
	"path"
	"time"

	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/basestore"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const pathInsertFmtstr = `
	WITH already_exists (id) AS (
		SELECT id
		FROM repo_paths
		WHERE repo_id = %s
		AND absolute_path = %s
	),
	need_to_insert (id) AS (
		INSERT INTO repo_paths (repo_id, absolute_path, parent_id)
		SELECT %s, %s, %s
		WHERE NOT EXISTS (
			SELECT
			FROM repo_paths
			WHERE repo_id = %s
			AND absolute_path = %s
		)
		ON CONFLICT (repo_id, absolute_path) DO NOTHING
		RETURNING id
	)
	SELECT id FROM already_exists
	UNION ALL
	SELECT id FROM need_to_insert
`

// ensureRepoPaths takes paths and makes sure they all exist in the database
// (alongside with their ancestor paths) as per the schema.
//
// The operation makes a number of queries to the database that is comparable to
// the size of the given file tree. In other words, every directory mentioned in
// the `files` (including parents and ancestors) will be queried or inserted with
// a single query (no repetitions though). Optimizing this into fewer queries
// seems to make the implementation very hard to read.
//
// The result int slice is guaranteed to be in order corresponding to the order
// of `files`.
func ensureRepoPaths(ctx context.Context, db *basestore.Store, files []string, repoID api.RepoID) ([]int, error) {
	// Compute all the ancestor paths for all given files.
	var paths []string
	for _, file := range files {
		for p := file; p != "."; p = path.Dir(p) {
			paths = append(paths, p)
		}
	}
	// Add empty string which references the repo root directory.
	paths = append(paths, "")
	// Reverse paths so we start at the root.
	for i := range len(paths) / 2 {
		j := len(paths) - i - 1
		paths[i], paths[j] = paths[j], paths[i]
	}
	// Remove duplicates from paths, to avoid extra query, especially if many files
	// within the same directory structure are referenced.
	seen := make(map[string]bool)
	j := 0
	for i := range len(paths) {
		if !seen[paths[i]] {
			seen[paths[i]] = true
			paths[j] = paths[i]
			j++
		}
	}
	paths = paths[:j]
	// Insert all directories one query each and note the IDs.
	ids := map[string]int{}
	for _, p := range paths {
		var parentID *int
		parent := path.Dir(p)
		if parent == "." {
			parent = ""
		}
		if id, ok := ids[parent]; p != "" && ok {
			parentID = &id
		} else if p != "" {
			return nil, errors.Newf("cannot find parent id of %q: this is a bug", p)
		}
		r := db.QueryRow(ctx, sqlf.Sprintf(pathInsertFmtstr, repoID, p, repoID, p, parentID, repoID, p))
		var id int
		if err := r.Scan(&id); err != nil {
			return nil, errors.Wrapf(err, "failed to insert or retrieve %q", p)
		}
		ids[p] = id
	}
	// Return the IDs of inserted files changed, in order of `files`.
	fIDs := make([]int, len(files))
	for i, f := range files {
		id, ok := ids[f]
		if !ok {
			return nil, errors.Newf("cannot find id of %q which should have been inserted, this is a bug", f)
		}
		fIDs[i] = id
	}
	return fIDs, nil
}

// RepoTreeCounts allows iterating over file paths and yield total counts
// of all the files within a file tree rooted at given path.
type RepoTreeCounts interface {
	Iterate(func(path string, totalFiles int) error) error
}

type RepoPathStore interface {
	// UpdateFileCounts inserts file counts for every iterated path at given repository.
	// If any of the iterated paths does not exist, it's created. Returns the number of updated paths.
	UpdateFileCounts(context.Context, api.RepoID, RepoTreeCounts, time.Time) (int, error)
	// AggregateFileCount returns the file count aggregated for given TreeLocationOps.
	// For instance, TreeLocationOpts with RepoID and Path returns counts for tree at given path,
	// setting only RepoID gives counts for repo root, while setting none gives counts for the whole
	// instance. Lack of data counts as 0.
	AggregateFileCount(context.Context, TreeLocationOpts) (int32, error)
}

var _ RepoPathStore = &repoPathStore{}

type repoPathStore struct {
	*basestore.Store
}

const updateFileCountsFmtstr = `
	UPDATE repo_paths
	SET tree_files_count = %s,
	tree_files_counts_updated_at = %s
	WHERE id = %s
`

func (s *repoPathStore) UpdateFileCounts(ctx context.Context, repoID api.RepoID, counts RepoTreeCounts, timestamp time.Time) (int, error) {
	var rowsUpdated int
	err := counts.Iterate(func(path string, totalFiles int) error {
		pathIDs, err := ensureRepoPaths(ctx, s.Store, []string{path}, repoID)
		if err != nil {
			return err
		}
		if got, want := len(pathIDs), 1; got != want {
			return errors.Newf("want exactly 1 repo path, got %d", got)
		}
		res, err := s.ExecResult(ctx, sqlf.Sprintf(updateFileCountsFmtstr, totalFiles, timestamp, pathIDs[0]))
		if err != nil {
			return err
		}
		rows, err := res.RowsAffected()
		if err != nil {
			return err
		}
		rowsUpdated += int(rows)
		return nil
	})
	return rowsUpdated, err
}

const aggregateFileCountFmtstr = `
	WITH signal_config AS (SELECT * FROM own_signal_configurations WHERE name = 'analytics' LIMIT 1)
    SELECT SUM(COALESCE(p.tree_files_count, 0))
    FROM repo_paths AS p
    WHERE p.absolute_path = %s AND p.repo_id NOT IN (
		SELECT repo.id FROM repo, signal_config WHERE repo.name ~~ ANY(signal_config.excluded_repo_patterns)
	)
`

// AggregateFileCount shows total number of files which repo paths are added to
// repo_paths table. As it is used by analytics, it considers the exclusions
// added to analytics configuration.
func (s *repoPathStore) AggregateFileCount(ctx context.Context, opts TreeLocationOpts) (int32, error) {
	var qs []*sqlf.Query
	qs = append(qs, sqlf.Sprintf(aggregateFileCountFmtstr, opts.Path))
	if repoID := opts.RepoID; repoID != 0 {
		qs = append(qs, sqlf.Sprintf("AND p.repo_id = %s", repoID))
	}
	var count int32
	if err := s.Store.QueryRow(ctx, sqlf.Join(qs, "\n")).Scan(&dbutil.NullInt32{N: &count}); err != nil {
		return 0, err
	}
	return count, nil
}
