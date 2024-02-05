package graphqlbackend

import (
	"context"
	"io/fs"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func (r *GitTreeEntryResolver) IsRoot() bool {
	cleanPath := path.Clean(r.Path())
	return cleanPath == "/" || cleanPath == "." || cleanPath == ""
}

type gitTreeEntryConnectionArgs struct {
	graphqlutil.ConnectionArgs
	// If Ancestors is true and the tree is loaded from a subdirectory, we will
	// return a flat list of all entries in all parent directories.
	Ancestors bool
}

func (r *GitTreeEntryResolver) Entries(ctx context.Context, args *gitTreeEntryConnectionArgs) ([]*GitTreeEntryResolver, error) {
	return r.entries(ctx, args, nil)
}

func (r *GitTreeEntryResolver) Directories(ctx context.Context, args *gitTreeEntryConnectionArgs) ([]*GitTreeEntryResolver, error) {
	return r.entries(ctx, args, func(fi fs.FileInfo) bool { return fi.Mode().IsDir() })
}

func (r *GitTreeEntryResolver) Files(ctx context.Context, args *gitTreeEntryConnectionArgs) ([]*GitTreeEntryResolver, error) {
	return r.entries(ctx, args, func(fi fs.FileInfo) bool { return !fi.Mode().IsDir() })
}

func (r *GitTreeEntryResolver) entries(ctx context.Context, args *gitTreeEntryConnectionArgs, filter func(fi fs.FileInfo) bool) (_ []*GitTreeEntryResolver, err error) {
	tr, ctx := trace.New(ctx, "GitTreeEntryResolver.entries")
	defer tr.EndWithErr(&err)

	if args.First != nil && *args.First < 0 {
		return nil, errors.Newf("invalid argument for first, must be non-negative")
	}

	entries, err := r.gitserverClient.ReadDir(ctx, r.commit.repoResolver.RepoName(), api.CommitID(r.commit.OID()), r.Path(), false)
	if err != nil {
		if strings.Contains(err.Error(), "file does not exist") { // TODO proper error value
			// empty tree is not an error
		} else {
			return nil, err
		}
	}

	maxResolvers := len(entries)
	if args.First != nil && int(*args.First) < maxResolvers {
		maxResolvers = int(*args.First)
	}

	sort.Sort(byDirectory(entries))

	resolvers := make([]*GitTreeEntryResolver, 0, maxResolvers)
	for _, entry := range entries {
		if len(resolvers) >= maxResolvers {
			break
		}

		// Apply any additional filtering
		if filter == nil || filter(entry) {
			opts := GitTreeEntryResolverOpts{
				Commit: r.Commit(),
				Stat:   entry,
			}
			resolvers = append(resolvers, NewGitTreeEntryResolver(r.db, r.gitserverClient, opts))
		}
	}

	if args.Ancestors && !r.IsRoot() {
		p := r.Path()
		p = strings.Trim(p, "/")
		parent := NewGitTreeEntryResolver(r.db, r.gitserverClient, GitTreeEntryResolverOpts{
			Commit: r.commit,
			Stat:   CreateFileInfo(filepath.Dir(p), true),
		})
		parentEntries, err := parent.entries(ctx, &gitTreeEntryConnectionArgs{
			Ancestors: true,
		}, nil)
		if err != nil {
			return nil, err
		}
		resolvers = append(parentEntries, resolvers...)
	}

	return resolvers, nil
}

// byDirectory implements sort.Sortable and orders directories before files.
type byDirectory []fs.FileInfo

func (s byDirectory) Len() int {
	return len(s)
}

func (s byDirectory) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}

func (s byDirectory) Less(i, j int) bool {
	if s[i].IsDir() && !s[j].IsDir() {
		return true
	}

	if !s[i].IsDir() && s[j].IsDir() {
		return false
	}

	return s[i].Name() < s[j].Name()
}
