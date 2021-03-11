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

	"github.com/sourcegraph/go-diff/diff"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend/graphqlutil"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/highlight"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/vcs/git"
)

type RepositoryComparisonInput struct {
	Base         *string
	Head         *string
	FetchMissing bool
}

type FileDiffsConnectionArgs struct {
	First *int32
	After *string
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

func NewRepositoryComparison(ctx context.Context, db dbutil.DB, r *RepositoryResolver, args *RepositoryComparisonInput) (*RepositoryComparisonResolver, error) {
	var baseRevspec, headRevspec string
	if args.Base == nil {
		baseRevspec = "HEAD"
	} else {
		baseRevspec = *args.Base
	}
	if args.Head == nil {
		headRevspec = "HEAD"
	} else {
		headRevspec = *args.Head
	}

	getCommit := func(ctx context.Context, repo api.RepoName, revspec string) (*GitCommitResolver, error) {
		if revspec == git.DevNullSHA {
			return nil, nil
		}

		opt := git.ResolveRevisionOptions{
			NoEnsureRevision: !args.FetchMissing,
		}

		// Call ResolveRevision to trigger fetches from remote (in case base/head commits don't
		// exist).
		commitID, err := git.ResolveRevision(ctx, repo, revspec, opt)
		if err != nil {
			return nil, err
		}

		return toGitCommitResolver(r, db, commitID, nil), nil
	}

	head, err := getCommit(ctx, r.name, headRevspec)
	if err != nil {
		return nil, err
	}

	// Find the common merge-base for the diff. That's the revision the diff applies to,
	// not the baseRevspec.
	mergeBaseCommit, err := git.MergeBase(ctx, r.name, api.CommitID(baseRevspec), api.CommitID(headRevspec))
	if err != nil {
		return nil, err
	}

	// We use the merge-base as the base commit here, as the diff will only be guaranteed to be
	// applicable to the file from that revision.
	commitString := strings.TrimSpace(string(mergeBaseCommit))
	base, err := getCommit(ctx, r.name, commitString)
	if err != nil {
		return nil, err
	}

	return &RepositoryComparisonResolver{
		db:          db,
		baseRevspec: baseRevspec,
		headRevspec: headRevspec,
		base:        base,
		head:        head,
		repo:        r,
	}, nil
}

func (r *RepositoryResolver) Comparison(ctx context.Context, args *RepositoryComparisonInput) (*RepositoryComparisonResolver, error) {
	return NewRepositoryComparison(ctx, r.db, r, args)
}

type RepositoryComparisonResolver struct {
	db                       dbutil.DB
	baseRevspec, headRevspec string
	base, head               *GitCommitResolver
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

func (r *RepositoryComparisonResolver) Commits(
	args *graphqlutil.ConnectionArgs,
) *gitCommitConnectionResolver {
	return &gitCommitConnectionResolver{
		db:            r.db,
		revisionRange: r.baseRevspec + ".." + r.headRevspec,
		first:         args.First,
		repo:          r.repo,
	}
}

func (r *RepositoryComparisonResolver) FileDiffs(ctx context.Context, args *FileDiffsConnectionArgs) (FileDiffConnection, error) {
	return NewFileDiffConnectionResolver(
		r.db,
		r.base,
		r.head,
		args,
		computeRepositoryComparisonDiff(r),
		repositoryComparisonNewFile,
	), nil
}

// repositoryComparisonNewFile is the default NewFileFunc used by
// RepositoryComparisonResolver to produce the new file in a FileDiffResolver.
func repositoryComparisonNewFile(db dbutil.DB, r *FileDiffResolver) FileResolver {
	return &GitTreeEntryResolver{
		db:     db,
		commit: r.Head,
		stat:   CreateFileInfo(r.FileDiff.NewName, false),
	}
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

			var iter *git.DiffFileIterator
			iter, err = git.Diff(ctx, git.DiffOptions{
				Repo: cmp.repo.name,
				Base: base,
				Head: string(cmp.head.OID()),
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
type NewFileFunc func(db dbutil.DB, r *FileDiffResolver) FileResolver

func NewFileDiffConnectionResolver(
	db dbutil.DB,
	base, head *GitCommitResolver,
	args *FileDiffsConnectionArgs,
	compute ComputeDiffFunc,
	newFileFunc NewFileFunc,
) *fileDiffConnectionResolver {
	return &fileDiffConnectionResolver{
		db:      db,
		base:    base,
		head:    head,
		first:   args.First,
		after:   args.After,
		compute: compute,
		newFile: newFileFunc,
	}
}

type fileDiffConnectionResolver struct {
	db      dbutil.DB
	base    *GitCommitResolver
	head    *GitCommitResolver
	first   *int32
	after   *string
	compute ComputeDiffFunc
	newFile NewFileFunc
}

func (r *fileDiffConnectionResolver) Nodes(ctx context.Context) ([]FileDiff, error) {
	fileDiffs, afterIdx, _, err := r.compute(ctx, &FileDiffsConnectionArgs{First: r.first, After: r.after})
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
		resolvers[i] = &FileDiffResolver{
			db:       r.db,
			newFile:  r.newFile,
			FileDiff: fileDiff,
			Base:     r.base,
			Head:     r.head,
		}
	}
	return resolvers, nil
}

func (r *fileDiffConnectionResolver) TotalCount(ctx context.Context) (*int32, error) {
	fileDiffs, _, hasNextPage, err := r.compute(ctx, &FileDiffsConnectionArgs{After: r.after, First: r.first})
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
	_, afterIdx, hasNextPage, err := r.compute(ctx, &FileDiffsConnectionArgs{After: r.after, First: r.first})
	if err != nil {
		return nil, err
	}
	if !hasNextPage {
		return graphqlutil.HasNextPage(hasNextPage), nil
	}
	next := afterIdx
	if r.first != nil {
		next += *r.first
	}
	return graphqlutil.NextPageCursor(strconv.Itoa(int(next))), nil
}

func (r *fileDiffConnectionResolver) DiffStat(ctx context.Context) (*DiffStat, error) {
	fileDiffs, _, _, err := r.compute(ctx, &FileDiffsConnectionArgs{After: r.after, First: r.first})
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
	fileDiffs, _, _, err := r.compute(ctx, &FileDiffsConnectionArgs{After: r.after, First: r.first})
	if err != nil {
		return "", err
	}
	b, err := diff.PrintMultiFileDiff(fileDiffs)
	return string(b), err
}

type FileDiffResolver struct {
	FileDiff *diff.FileDiff
	Base     *GitCommitResolver
	Head     *GitCommitResolver

	db      dbutil.DB
	newFile NewFileFunc
}

func (r *FileDiffResolver) OldPath() *string { return diffPathOrNull(r.FileDiff.OrigName) }
func (r *FileDiffResolver) NewPath() *string { return diffPathOrNull(r.FileDiff.NewName) }

func (r *FileDiffResolver) Hunks() []*DiffHunk {
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

func (r *FileDiffResolver) Stat() *DiffStat {
	stat := r.FileDiff.Stat()
	return NewDiffStat(stat)
}

func (r *FileDiffResolver) OldFile() FileResolver {
	if diffPathOrNull(r.FileDiff.OrigName) == nil {
		return nil
	}
	return &GitTreeEntryResolver{
		db:     r.db,
		commit: r.Base,
		stat:   CreateFileInfo(r.FileDiff.OrigName, false),
	}
}

func (r *FileDiffResolver) NewFile() FileResolver {
	if diffPathOrNull(r.FileDiff.NewName) == nil {
		return nil
	}
	return r.newFile(r.db, r)
}

func (r *FileDiffResolver) MostRelevantFile() FileResolver {
	if newFile := r.NewFile(); newFile != nil {
		return newFile
	}
	return r.OldFile()
}

func (r *FileDiffResolver) InternalID() string {
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
			content, err := file.Content(ctx)
			if err != nil {
				return nil, err
			}
			lines, aborted, err := highlight.CodeAsLines(ctx, highlight.Params{
				Content:            []byte(content),
				Filepath:           file.Path(),
				DisableTimeout:     args.DisableTimeout,
				HighlightLongLines: args.HighlightLongLines,
				IsLightTheme:       args.IsLightTheme,
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
		r.highlightedBase, r.highlightErr = highlightFile(ctx, r.oldFile)
		if r.highlightErr != nil {
			return
		}
		r.highlightedHead, r.highlightErr = highlightFile(ctx, r.newFile)
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
	hunkLines := strings.Split(string(r.hunk.Body), "\n")
	// Remove final empty line on files that end with a newline, as most code hosts do.
	if hunkLines[len(hunkLines)-1] == "" {
		hunkLines = hunkLines[:len(hunkLines)-1]
	}
	highlightedDiffHunkLineResolvers := make([]*highlightedDiffHunkLineResolver, len(hunkLines))
	// Lines in highlightedBase and highlightedHead are 0-indexed.
	baseLine := r.hunk.OrigStartLine - 1
	headLine := r.hunk.NewStartLine - 1
	for i, hunkLine := range hunkLines {
		highlightedDiffHunkLineResolver := highlightedDiffHunkLineResolver{}
		if hunkLine[0] == ' ' {
			highlightedDiffHunkLineResolver.kind = "UNCHANGED"
			highlightedDiffHunkLineResolver.html = string(highlightedBase[baseLine])
			baseLine++
			headLine++
		} else if hunkLine[0] == '+' {
			highlightedDiffHunkLineResolver.kind = "ADDED"
			highlightedDiffHunkLineResolver.html = string(highlightedHead[headLine])
			headLine++
		} else if hunkLine[0] == '-' {
			highlightedDiffHunkLineResolver.kind = "DELETED"
			highlightedDiffHunkLineResolver.html = string(highlightedBase[baseLine])
			baseLine++
		} else {
			return nil, fmt.Errorf("expected patch lines to start with ' ', '-', '+', but found %q", hunkLine[0])
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

type highlightedDiffHunkLineResolver struct {
	html string
	kind string
}

func (r *highlightedDiffHunkLineResolver) HTML() string {
	return r.html
}

func (r *highlightedDiffHunkLineResolver) Kind() string {
	return r.kind
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
		added:   s.Added,
		changed: s.Changed,
		deleted: s.Deleted,
	}
}

type DiffStat struct{ added, changed, deleted int32 }

func (r *DiffStat) AddStat(s diff.Stat) {
	r.added += s.Added
	r.changed += s.Changed
	r.deleted += s.Deleted
}

func (r *DiffStat) AddDiffStat(s *DiffStat) {
	r.added += s.Added()
	r.changed += s.Changed()
	r.deleted += s.Deleted()
}

func (r *DiffStat) Added() int32   { return r.added }
func (r *DiffStat) Changed() int32 { return r.changed }
func (r *DiffStat) Deleted() int32 { return r.deleted }
