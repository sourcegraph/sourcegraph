package graphqlbackend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"html/template"
	"io"
	"strconv"
	"strings"
	"sync"

	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log
	"github.com/sourcegraph/conc/pool"
	"github.com/sourcegraph/go-diff/diff"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gosyntect"
	"github.com/sourcegraph/sourcegraph/internal/highlight"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

type RepositoryComparisonInput struct {
	Base         *string
	Head         *string
	FetchMissing bool
}

type FileDiffsConnectionArgs struct {
	First *int32
	After *string
	Paths *[]string
}

type RepositoryComparisonInterface interface {
	BaseRepository() *RepositoryResolver
	FileDiffs(ctx context.Context, args *FileDiffsConnectionArgs) (FileDiffConnection, error)

	ToRepositoryComparison() (*RepositoryComparisonResolver, bool)
	ToPreviewRepositoryComparison() (PreviewRepositoryComparisonResolver, bool)
}

type FileDiffConnection interface {
	Nodes(ctx context.Context) ([]FileDiff, error)
	TotalCount(ctx context.Context) (*int32, error)
	PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error)
	DiffStat(ctx context.Context) (*DiffStat, error)
	RawDiff(ctx context.Context) (string, error)
}

type FileDiff interface {
	OldPath() *string
	NewPath() *string
	Hunks() []*DiffHunk
	Stat() *DiffStat
	OldFile() FileResolver
	NewFile() FileResolver
	MostRelevantFile() FileResolver
	InternalID() string
}

func NewRepositoryComparison(ctx context.Context, db database.DB, client gitserver.Client, r *RepositoryResolver, args *RepositoryComparisonInput) (*RepositoryComparisonResolver, error) {
	baseRevspec := pointers.Deref(args.Base, "HEAD")
	headRevspec := pointers.Deref(args.Head, "HEAD")

	getCommit := func(ctx context.Context, repo api.RepoName, revspec string) (*GitCommitResolver, error) {
		if revspec == gitserver.DevNullSHA {
			return nil, nil
		}

		opt := gitserver.ResolveRevisionOptions{
			EnsureRevision: args.FetchMissing,
		}

		// Call ResolveRevision to trigger fetches from remote (in case base/head commits don't
		// exist).
		commitID, err := client.ResolveRevision(ctx, repo, revspec, opt)
		if err != nil {
			return nil, err
		}

		return NewGitCommitResolver(db, client, r, commitID, nil), nil
	}

	head, err := getCommit(ctx, r.RepoName(), headRevspec)
	if err != nil {
		return nil, err
	}

	var base *GitCommitResolver
	rangeType := ".."
	if baseRevspec != gitserver.DevNullSHA {
		rangeType = "..."
		// Find the common merge-base for the diff. That's the revision the diff applies to,
		// not the baseRevspec.
		mergeBaseCommit, err := client.MergeBase(ctx, r.id, baseRevspec, headRevspec)
		if err != nil {
			return nil, err
		}

		// If possible, use the merge-base as the base commit, as the diff will only be guaranteed to be
		// applicable to the file from that revision.
		commitString := string(mergeBaseCommit)
		if commitString == "" {
			// Fallback option which should work even if there is no merge base.
			commitString = baseRevspec
			rangeType = ".."
		}
		base, err = getCommit(ctx, r.RepoName(), commitString)
		if err != nil {
			return nil, err
		}
	}

	return &RepositoryComparisonResolver{
		db:              db,
		gitserverClient: client,
		baseRevspec:     baseRevspec,
		headRevspec:     headRevspec,
		base:            base,
		head:            head,
		repo:            r,
		rangeType:       rangeType,
	}, nil
}

func (r *RepositoryResolver) Comparison(ctx context.Context, args *RepositoryComparisonInput) (*RepositoryComparisonResolver, error) {
	return NewRepositoryComparison(ctx, r.db, r.gitserverClient, r, args)
}

type RepositoryComparisonResolver struct {
	db                       database.DB
	gitserverClient          gitserver.Client
	baseRevspec, headRevspec string
	base, head               *GitCommitResolver
	rangeType                string
	repo                     *RepositoryResolver
}

// Type guard.
var _ RepositoryComparisonInterface = &RepositoryComparisonResolver{}

func (r *RepositoryComparisonResolver) ToPreviewRepositoryComparison() (PreviewRepositoryComparisonResolver, bool) {
	return nil, false
}

func (r *RepositoryComparisonResolver) ToRepositoryComparison() (*RepositoryComparisonResolver, bool) {
	return r, true
}

func (r *RepositoryComparisonResolver) BaseRepository() *RepositoryResolver { return r.repo }

func (r *RepositoryComparisonResolver) HeadRepository() *RepositoryResolver { return r.repo }

func (r *RepositoryComparisonResolver) Range() *gitRevisionRange {
	return &gitRevisionRange{
		expr:      r.baseRevspec + "..." + r.headRevspec,
		base:      &gitRevSpec{expr: &gitRevSpecExpr{expr: r.baseRevspec, repo: r.repo}},
		head:      &gitRevSpec{expr: &gitRevSpecExpr{expr: r.headRevspec, repo: r.repo}},
		mergeBase: nil, // not currently used
	}
}

// RepositoryComparisonCommitsArgs is a set of arguments for listing commits on the RepositoryComparisonResolver
type RepositoryComparisonCommitsArgs struct {
	graphqlutil.ConnectionArgs
	Path *string
}

func (r *RepositoryComparisonResolver) Commits(
	args *RepositoryComparisonCommitsArgs,
) *gitCommitConnectionResolver {
	return &gitCommitConnectionResolver{
		db:              r.db,
		gitserverClient: r.gitserverClient,
		revisionRange:   r.baseRevspec + ".." + r.headRevspec,
		first:           args.First,
		repo:            r.repo,
		path:            args.Path,
	}
}

func (r *RepositoryComparisonResolver) FileDiffs(ctx context.Context, args *FileDiffsConnectionArgs) (FileDiffConnection, error) {
	return NewFileDiffConnectionResolver(
		r.db,
		r.gitserverClient,
		r.base,
		r.head,
		args,
		computeRepositoryComparisonDiff(r),
		repositoryComparisonNewFile,
	), nil
}

// repositoryComparisonNewFile is the default NewFileFunc used by
// RepositoryComparisonResolver to produce the new file in a FileDiffResolver.
func repositoryComparisonNewFile(db database.DB, r *fileDiffResolver) FileResolver {
	opts := GitTreeEntryResolverOpts{
		Commit: r.Head,
		Stat:   CreateFileInfo(r.FileDiff.NewName, false),
	}
	return NewGitTreeEntryResolver(db, r.gitserverClient, opts)
}

// computeRepositoryComparisonDiff returns a ComputeDiffFunc for the given
// RepositoryComparisonResolver.
func computeRepositoryComparisonDiff(cmp *RepositoryComparisonResolver) ComputeDiffFunc {
	var (
		once        sync.Once
		fileDiffs   []*diff.FileDiff
		afterIdx    int32
		hasNextPage bool
		err         error
	)
	return func(ctx context.Context, args *FileDiffsConnectionArgs) ([]*diff.FileDiff, int32, bool, error) {
		once.Do(func() {
			// Todo: It's possible that the rangeSpec changes in between two executions, then the cursor would be invalid and the
			// whole pagination should not be continued.
			if args.After != nil {
				parsedIdx, err := strconv.ParseInt(*args.After, 0, 32)
				if err != nil {
					return
				}
				if parsedIdx < 0 {
					parsedIdx = 0
				}
				afterIdx = int32(parsedIdx)
			}

			var base string
			if cmp.base == nil {
				base = cmp.baseRevspec
			} else {
				base = string(cmp.base.OID())
			}

			paths := pointers.Deref(args.Paths, nil)

			var iter *gitserver.DiffFileIterator
			iter, err = cmp.gitserverClient.Diff(ctx, gitserver.DiffOptions{
				Repo:      cmp.repo.id,
				Base:      base,
				Head:      string(cmp.head.OID()),
				RangeType: cmp.rangeType,
				Paths:     paths,
			})
			if err != nil {
				return
			}
			defer iter.Close()

			if args.First != nil {
				fileDiffs = make([]*diff.FileDiff, 0, int(*args.First)) // preallocate
			}
			for {
				var fileDiff *diff.FileDiff
				fileDiff, err = iter.Next()
				if err == io.EOF {
					err = nil
					break
				}
				if err != nil {
					return
				}
				fileDiffs = append(fileDiffs, fileDiff)
				if args.First != nil && len(fileDiffs) == int(*args.First+afterIdx) {
					// Check for hasNextPage.
					_, err = iter.Next()
					if err != nil && err != io.EOF {
						return
					}
					if err == io.EOF {
						err = nil
					} else {
						hasNextPage = true
					}
					break
				}
			}
		})
		return fileDiffs, afterIdx, hasNextPage, err
	}
}

// ComputeDiffFunc is a function that computes FileDiffs for the given args. It
// returns the diffs, the starting index from which to return entries (`after`
// param), whether there's a next page, and an optional error.
type ComputeDiffFunc func(ctx context.Context, args *FileDiffsConnectionArgs) ([]*diff.FileDiff, int32, bool, error)

// NewFileFunc is a function that returns the "new" file in a FileDiff as a
// FileResolver.
type NewFileFunc func(db database.DB, r *fileDiffResolver) FileResolver

func NewFileDiffConnectionResolver(
	db database.DB,
	gitserverClient gitserver.Client,
	base, head *GitCommitResolver,
	args *FileDiffsConnectionArgs,
	compute ComputeDiffFunc,
	newFileFunc NewFileFunc,
) *fileDiffConnectionResolver {
	return &fileDiffConnectionResolver{
		db:              db,
		gitserverClient: gitserverClient,
		base:            base,
		head:            head,
		args:            args,
		compute:         compute,
		newFile:         newFileFunc,
	}
}

type fileDiffConnectionResolver struct {
	db              database.DB
	gitserverClient gitserver.Client
	base            *GitCommitResolver
	head            *GitCommitResolver
	args            *FileDiffsConnectionArgs
	compute         ComputeDiffFunc
	newFile         NewFileFunc
}

func (r *fileDiffConnectionResolver) Nodes(ctx context.Context) ([]FileDiff, error) {
	fileDiffs, afterIdx, _, err := r.compute(ctx, r.args)
	if err != nil {
		return nil, err
	}
	if int(afterIdx) <= len(fileDiffs) {
		// If the lower boundary is within bounds, return from the lower boundary.
		fileDiffs = fileDiffs[afterIdx:]
	} else {
		// If the lower boundary is out of bounds, return an empty result.
		fileDiffs = []*diff.FileDiff{}
	}

	resolvers := make([]FileDiff, len(fileDiffs))
	for i, fileDiff := range fileDiffs {
		resolvers[i] = &fileDiffResolver{
			db:              r.db,
			gitserverClient: r.gitserverClient,
			newFile:         r.newFile,
			FileDiff:        fileDiff,
			Base:            r.base,
			Head:            r.head,
		}
	}
	return resolvers, nil
}

func (r *fileDiffConnectionResolver) TotalCount(ctx context.Context) (*int32, error) {
	fileDiffs, _, hasNextPage, err := r.compute(ctx, r.args)
	if err != nil {
		return nil, err
	}
	if !hasNextPage {
		n := int32(len(fileDiffs))
		return &n, nil
	}
	return nil, nil // total count is not available
}

func (r *fileDiffConnectionResolver) PageInfo(ctx context.Context) (*graphqlutil.PageInfo, error) {
	_, afterIdx, hasNextPage, err := r.compute(ctx, r.args)
	if err != nil {
		return nil, err
	}
	if !hasNextPage {
		return graphqlutil.HasNextPage(hasNextPage), nil
	}
	next := afterIdx
	if r.args.First != nil {
		next += *r.args.First
	}
	return graphqlutil.NextPageCursor(strconv.Itoa(int(next))), nil
}

func (r *fileDiffConnectionResolver) DiffStat(ctx context.Context) (*DiffStat, error) {
	fileDiffs, _, _, err := r.compute(ctx, r.args)
	if err != nil {
		return nil, err
	}

	stat := &DiffStat{}
	for _, fileDiff := range fileDiffs {
		s := fileDiff.Stat()
		stat.AddStat(s)
	}
	return stat, nil
}

func (r *fileDiffConnectionResolver) RawDiff(ctx context.Context) (string, error) {
	fileDiffs, _, _, err := r.compute(ctx, r.args)
	if err != nil {
		return "", err
	}
	b, err := diff.PrintMultiFileDiff(fileDiffs)
	return string(b), err
}

type fileDiffResolver struct {
	FileDiff *diff.FileDiff
	Base     *GitCommitResolver
	Head     *GitCommitResolver

	db              database.DB
	gitserverClient gitserver.Client
	newFile         NewFileFunc
}

func (r *fileDiffResolver) OldPath() *string { return diffPathOrNull(r.FileDiff.OrigName) }
func (r *fileDiffResolver) NewPath() *string { return diffPathOrNull(r.FileDiff.NewName) }

func (r *fileDiffResolver) Hunks() []*DiffHunk {
	highlighter := &fileDiffHighlighter{
		oldFile: r.OldFile(),
		newFile: r.NewFile(),
	}
	hunks := make([]*DiffHunk, len(r.FileDiff.Hunks))
	for i, hunk := range r.FileDiff.Hunks {
		hunks[i] = NewDiffHunk(hunk, highlighter)
	}
	return hunks
}

func (r *fileDiffResolver) Stat() *DiffStat {
	stat := r.FileDiff.Stat()
	return NewDiffStat(stat)
}

func (r *fileDiffResolver) OldFile() FileResolver {
	if diffPathOrNull(r.FileDiff.OrigName) == nil {
		return nil
	}
	opts := GitTreeEntryResolverOpts{
		Commit: r.Base,
		Stat:   CreateFileInfo(r.FileDiff.OrigName, false),
	}
	return NewGitTreeEntryResolver(r.db, r.gitserverClient, opts)
}

func (r *fileDiffResolver) NewFile() FileResolver {
	if diffPathOrNull(r.FileDiff.NewName) == nil {
		return nil
	}
	return r.newFile(r.db, r)
}

func (r *fileDiffResolver) MostRelevantFile() FileResolver {
	if newFile := r.NewFile(); newFile != nil {
		return newFile
	}
	return r.OldFile()
}

func (r *fileDiffResolver) InternalID() string {
	b := sha256.Sum256([]byte(fmt.Sprintf("%d:%s:%s", len(r.FileDiff.OrigName), r.FileDiff.OrigName, r.FileDiff.NewName)))
	return hex.EncodeToString(b[:])[:32]
}

func diffPathOrNull(path string) *string {
	if path == "/dev/null" || path == "" {
		return nil
	}
	return &path
}

func NewDiffHunk(hunk *diff.Hunk, highlighter FileDiffHighlighter) *DiffHunk {
	return &DiffHunk{hunk: hunk, highlighter: highlighter}
}

type FileDiffHighlighter interface {
	Highlight(ctx context.Context, args *HighlightArgs) ([]template.HTML, []template.HTML, bool, error)
}

type fileDiffHighlighter struct {
	oldFile          FileResolver
	newFile          FileResolver
	highlightedBase  []template.HTML
	highlightedHead  []template.HTML
	highlightOnce    sync.Once
	highlightErr     error
	highlightAborted bool
}

func (r *fileDiffHighlighter) Highlight(ctx context.Context, args *HighlightArgs) ([]template.HTML, []template.HTML, bool, error) {
	r.highlightOnce.Do(func() {
		highlightFile := func(ctx context.Context, file FileResolver) ([]template.HTML, error) {
			if file == nil {
				return nil, nil
			}
			content, err := file.Content(ctx, &GitTreeContentPageArgs{
				StartLine: args.StartLine,
				EndLine:   args.EndLine,
			})
			if err != nil {
				return nil, err
			}
			lines, aborted, err := highlight.CodeAsLines(ctx, highlight.Params{
				// We rely on the final newline to be kept for proper highlighting.
				KeepFinalNewline:   true,
				Content:            []byte(content),
				Filepath:           file.Path(),
				DisableTimeout:     args.DisableTimeout,
				HighlightLongLines: args.HighlightLongLines,
				Format:             gosyntect.HighlightResponseType(args.Format),
			})
			if aborted {
				r.highlightAborted = aborted
			}
			// It is okay to fail on binary files, we won't have to pick lines from such files in the Highlight resolver.
			if err != nil && err == highlight.ErrBinary {
				return []template.HTML{}, nil
			}
			return lines, err
		}
		p := pool.New().WithErrors()
		p.Go(func() (err error) {
			r.highlightedBase, err = highlightFile(ctx, r.oldFile)
			return err
		})
		p.Go(func() (err error) {
			r.highlightedHead, err = highlightFile(ctx, r.newFile)
			return err
		})
		r.highlightErr = p.Wait()
	})
	return r.highlightedBase, r.highlightedHead, r.highlightAborted, r.highlightErr
}

type DiffHunk struct {
	hunk        *diff.Hunk
	highlighter FileDiffHighlighter
}

func (r *DiffHunk) OldRange() *DiffHunkRange {
	return NewDiffHunkRange(r.hunk.OrigStartLine, r.hunk.OrigLines)
}
func (r *DiffHunk) OldNoNewlineAt() bool { return r.hunk.OrigNoNewlineAt != 0 }
func (r *DiffHunk) NewRange() *DiffHunkRange {
	return NewDiffHunkRange(r.hunk.NewStartLine, r.hunk.NewLines)
}

func (r *DiffHunk) Section() *string {
	if r.hunk.Section == "" {
		return nil
	}
	return &r.hunk.Section
}

func (r *DiffHunk) Body() string { return string(r.hunk.Body) }

func (r *DiffHunk) Highlight(ctx context.Context, args *HighlightArgs) (*highlightedDiffHunkBodyResolver, error) {
	highlightedBase, highlightedHead, aborted, err := r.highlighter.Highlight(ctx, args)
	if err != nil {
		return nil, err
	}

	// If the diff ends with a newline, we have to strip it, otherwise we iterate
	// over a ghost line that we don't want to render.
	hunkLines := strings.Split(strings.TrimSuffix(string(r.hunk.Body), "\n"), "\n")

	// Lines in highlightedBase and highlightedHead are 0-indexed.
	baseLine := r.hunk.OrigStartLine - 1
	headLine := r.hunk.NewStartLine - 1

	// TODO: There's been historically a lot of bugs in this code. They should be resolved now,
	// but let's keep this in for one more release and check log aggregators before we
	// finally remove this in Sourcegraph 4.4.
	safeIndex := func(lines []template.HTML, target int32) string {
		if len(lines) > int(target) {
			return string(lines[target])
		}
		log15.Error("returned default value for out of bounds index on highlighted code")
		return `<div></div>`
	}

	highlightedDiffHunkLineResolvers := make([]*highlightedDiffHunkLineResolver, len(hunkLines))
	for i, hunkLine := range hunkLines {
		highlightedDiffHunkLineResolver := highlightedDiffHunkLineResolver{}
		if hunkLine[0] == ' ' {
			highlightedDiffHunkLineResolver.kind = highlightedDiffHunkLineKindUnchanged
			highlightedDiffHunkLineResolver.html = safeIndex(highlightedBase, baseLine)
			baseLine++
			headLine++
		} else if hunkLine[0] == '+' {
			highlightedDiffHunkLineResolver.kind = highlightedDiffHunkLineKindAdded
			highlightedDiffHunkLineResolver.html = safeIndex(highlightedHead, headLine)
			headLine++
		} else if hunkLine[0] == '-' {
			highlightedDiffHunkLineResolver.kind = highlightedDiffHunkLineKindDeleted
			highlightedDiffHunkLineResolver.html = safeIndex(highlightedBase, baseLine)
			baseLine++
		} else {
			return nil, errors.Errorf("expected patch lines to start with ' ', '-', '+', but found %q", hunkLine[0])
		}
		highlightedDiffHunkLineResolvers[i] = &highlightedDiffHunkLineResolver
	}

	return &highlightedDiffHunkBodyResolver{
		highlightedDiffHunkLineResolvers: highlightedDiffHunkLineResolvers,
		aborted:                          aborted,
	}, nil
}

type highlightedDiffHunkBodyResolver struct {
	highlightedDiffHunkLineResolvers []*highlightedDiffHunkLineResolver
	aborted                          bool
}

func (r *highlightedDiffHunkBodyResolver) Aborted() bool {
	return r.aborted
}

func (r *highlightedDiffHunkBodyResolver) Lines() []*highlightedDiffHunkLineResolver {
	return r.highlightedDiffHunkLineResolvers
}

type highlightedDiffHunkLineKind int

const (
	highlightedDiffHunkLineKindUnchanged highlightedDiffHunkLineKind = iota
	highlightedDiffHunkLineKindAdded
	highlightedDiffHunkLineKindDeleted
)

type highlightedDiffHunkLineResolver struct {
	html string
	kind highlightedDiffHunkLineKind
}

func (r *highlightedDiffHunkLineResolver) HTML() string {
	return r.html
}

func (r *highlightedDiffHunkLineResolver) Kind() string {
	switch r.kind {
	case highlightedDiffHunkLineKindUnchanged:
		return "UNCHANGED"
	case highlightedDiffHunkLineKindAdded:
		return "ADDED"
	case highlightedDiffHunkLineKindDeleted:
		return "DELETED"
	}
	panic("unreachable code: r.kind didn't match a known type")
}

func NewDiffHunkRange(startLine, lines int32) *DiffHunkRange {
	return &DiffHunkRange{startLine: startLine, lines: lines}
}

type DiffHunkRange struct {
	startLine int32
	lines     int32
}

func (r *DiffHunkRange) StartLine() int32 { return r.startLine }
func (r *DiffHunkRange) Lines() int32     { return r.lines }

func NewDiffStat(s diff.Stat) *DiffStat {
	return &DiffStat{
		added:   s.Added + s.Changed,
		deleted: s.Deleted + s.Changed,
	}
}

type DiffStat struct{ added, deleted int32 }

func (r *DiffStat) AddStat(s diff.Stat) {
	r.added += s.Added + s.Changed
	r.deleted += s.Deleted + s.Changed
}

func (r *DiffStat) AddDiffStat(s *DiffStat) {
	r.added += s.Added()
	r.deleted += s.Deleted()
}

func (r *DiffStat) Added() int32   { return r.added }
func (r *DiffStat) Deleted() int32 { return r.deleted }
