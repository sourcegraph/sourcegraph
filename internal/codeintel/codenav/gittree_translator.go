package codenav

import (
	"context"
	"io"
	"strconv"
	"strings"

	"github.com/dgraph-io/ristretto"
	"github.com/sourcegraph/go-diff/diff"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/shared"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	sgtypes "github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// GitTreeTranslator translates a position within a git tree at a source commit into the
// equivalent position in a target commit. The git tree translator instance carries
// along with it the source commit.
//
// NOTE(id: codenav-file-rename-detection) At the moment, this code cannot handle positions/ranges
// going from one file to another (notice that the return values don't contain any updated
// path), because there is no way in gitserver to get rename detection without requesting
// a diff of the full repo (which may be quite large).
//
// Additionally, it's not clear if reusing the Document at path P1 in an older commit,
// at a different path P2 in a newer commit is even reliably useful, since symbol names
// may change based on the file name or directory name depending on the language.
//
// TODO(id: GitTreeTranslator-cleanup): Instead of storing the translationBase, we should
// take that as an argument. Specifically, use a struct with two fields, AncestorCommit
// and DescendantCommit, and avoid Source/Target terminology (which becomes confusing to
// understand with the reverse parameter). Instead, we can use an enum MappingDirection
// FromDescendantToAncestor | FromAncestorToDescendant if really needed (to avoid
// inconsistency when modifying the APIs below, as they take different values for 'reverse'
// in production).
type GitTreeTranslator interface {
	// GetTargetCommitPositionFromSourcePosition translates the given position from the source commit into the given
	// target commit. The target commit's position is returned, along with a boolean flag
	// indicating that the translation was successful. If reverse is true, then the source and
	// target commits are swapped.
	//
	// TODO(id: GitTreeTranslator-cleanup): The reverse parameter is always false in production,
	// let's remove the extra parameter.
	GetTargetCommitPositionFromSourcePosition(ctx context.Context, commit string, path string, px shared.Position, reverse bool) (shared.Position, bool, error)

	// GetTargetCommitRangeFromSourceRange translates the given range from the source commit into the given target
	// commit. The target commit's range is returned, along with a boolean flag indicating
	// that the translation was successful. If reverse is true, then the source and target commits
	// are swapped.
	//
	// TODO(id: GitTreeTranslator-cleanup): The reverse parameter is always true in production,
	// let's remove the extra parameter.
	GetTargetCommitRangeFromSourceRange(ctx context.Context, commit string, path string, rx shared.Range, reverse bool) (shared.Range, bool, error)
}

type gitTreeTranslator struct {
	client    gitserver.Client
	base      *translationBase
	hunkCache HunkCache
}

// TODO(id: GitTreeTranslator-cleanup): Strictly speaking, calling this translationBase is not
// quite correct as things can flip around based on the reverse parameter. So get rid
// of the commit field and pass that as a parameter for increased clarity at call-sites.
type translationBase struct {
	repo   *sgtypes.Repo
	commit api.CommitID
}

func (r *translationBase) GetRepoID() int {
	return int(r.repo.ID)
}

// HunkCache is a LRU cache that holds git diff hunks.
type HunkCache interface {
	// Get returns the value (if any) and a boolean representing whether the value was
	// found or not.
	Get(key any) (any, bool)

	// Set attempts to add the key-value item to the cache with the given cost. If it
	// returns false, then the value as dropped and the item isn't added to the cache.
	Set(key, value any, cost int64) bool
}

// NewHunkCache creates a data cache instance with the given maximum capacity.
func NewHunkCache(size int) (HunkCache, error) {
	return ristretto.NewCache(&ristretto.Config{
		NumCounters: int64(size) * 10,
		MaxCost:     int64(size),
		BufferItems: 64,
	})
}

// NewGitTreeTranslator creates a new GitTreeTranslator with the given repository and source commit.
func NewGitTreeTranslator(client gitserver.Client, base *translationBase, hunkCache HunkCache) GitTreeTranslator {
	return &gitTreeTranslator{
		client:    client,
		hunkCache: hunkCache,
		base:      base,
	}
}

// GetTargetCommitPositionFromSourcePosition translates the given position from the source commit into the given
// target commit. The target commit position is returned, along with a boolean flag
// indicating that the translation was successful. If reverse is true, then the source and
// target commits are swapped.
func (g *gitTreeTranslator) GetTargetCommitPositionFromSourcePosition(ctx context.Context, commit string, path string, px shared.Position, reverse bool) (shared.Position, bool, error) {
	hunks, err := g.readCachedHunks(ctx, g.base.repo, string(g.base.commit), commit, path, reverse)
	if err != nil {
		return shared.Position{}, false, err
	}

	commitPosition, ok := translatePosition(hunks, px)
	return commitPosition, ok, nil
}

// GetTargetCommitRangeFromSourceRange translates the given range from the source commit into the given target
// commit. The target commit range is returned, along with a boolean flag indicating
// that the translation was successful. If reverse is true, then the source and target commits
// are swapped.
func (g *gitTreeTranslator) GetTargetCommitRangeFromSourceRange(ctx context.Context, commit string, path string, rx shared.Range, reverse bool) (shared.Range, bool, error) {
	hunks, err := g.readCachedHunks(ctx, g.base.repo, string(g.base.commit), commit, path, reverse)
	if err != nil {
		return shared.Range{}, false, err
	}

	commitRange, ok := translateRange(hunks, rx)
	return commitRange, ok, nil
}

// readCachedHunks returns a position-ordered slice of changes (additions or deletions) of
// the given path between the given source and target commits. If reverse is true, then the
// source and target commits are swapped. If the git tree translator has a hunk cache, it
// will read from it before attempting to contact a remote server, and populate the cache
// with new results
func (g *gitTreeTranslator) readCachedHunks(ctx context.Context, repo *sgtypes.Repo, sourceCommit, targetCommit, path string, reverse bool) ([]*diff.Hunk, error) {
	if sourceCommit == targetCommit {
		return nil, nil
	}
	if reverse {
		sourceCommit, targetCommit = targetCommit, sourceCommit
	}

	if g.hunkCache == nil {
		return g.readHunks(ctx, repo, sourceCommit, targetCommit, path)
	}

	key := makeKey(strconv.FormatInt(int64(repo.ID), 10), sourceCommit, targetCommit, path)
	if hunks, ok := g.hunkCache.Get(key); ok {
		if hunks == nil {
			return nil, nil
		}

		return hunks.([]*diff.Hunk), nil
	}

	hunks, err := g.readHunks(ctx, repo, sourceCommit, targetCommit, path)
	if err != nil {
		return nil, err
	}

	g.hunkCache.Set(key, hunks, int64(len(hunks)))

	return hunks, nil
}

// readHunks returns a position-ordered slice of changes (additions or deletions) of
// the given path between the given source and target commits.
func (g *gitTreeTranslator) readHunks(ctx context.Context, repo *sgtypes.Repo, sourceCommit, targetCommit, path string) (_ []*diff.Hunk, err error) {
	r, err := g.client.Diff(ctx, repo.Name, gitserver.DiffOptions{
		Base:      sourceCommit,
		Head:      targetCommit,
		Paths:     []string{path},
		RangeType: "..",
	})
	if err != nil {
		return nil, err
	}
	defer func() {
		closeErr := r.Close()
		if err == nil {
			err = closeErr
		}
	}()

	fd, err := r.Next()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, err
	}

	return fd.Hunks, nil
}

// findHunk returns the last thunk that does not begin after the given line.
func findHunk(hunks []*diff.Hunk, line int) *diff.Hunk {
	i := 0
	for i < len(hunks) && int(hunks[i].OrigStartLine) <= line {
		i++
	}

	if i == 0 {
		return nil
	}
	return hunks[i-1]
}

// translateRange translates the given range by calling translatePosition on both of the range's
// endpoints. This function returns a boolean flag indicating that the translation was
// successful (which occurs when both endpoints of the range can be translated).
func translateRange(hunks []*diff.Hunk, r shared.Range) (shared.Range, bool) {
	start, ok := translatePosition(hunks, r.Start)
	if !ok {
		return shared.Range{}, false
	}

	end, ok := translatePosition(hunks, r.End)
	if !ok {
		return shared.Range{}, false
	}

	return shared.Range{Start: start, End: end}, true
}

// translatePosition translates the given position by setting the line number based on the
// number of additions and deletions that occur before that line. This function returns a
// boolean flag indicating that the translation is successful. A translation fails when the
// line indicated by the position has been edited.
func translatePosition(hunks []*diff.Hunk, pos shared.Position) (shared.Position, bool) {
	line, ok := translateLineNumbers(hunks, pos.Line)
	if !ok {
		return shared.Position{}, false
	}

	return shared.Position{Line: line, Character: pos.Character}, true
}

// translateLineNumbers translates the given line number based on the number of additions and deletions
// that occur before that line. This function returns a boolean flag indicating that the
// translation is successful. A translation fails when the given line has been edited.
func translateLineNumbers(hunks []*diff.Hunk, line int) (int, bool) {
	// Translate from bundle/lsp zero-index to git diff one-index
	line = line + 1

	hunk := findHunk(hunks, line)
	if hunk == nil {
		// Trivial case, no changes before this line
		return line - 1, true
	}

	// If the hunk ends before this line, we can simply set the line offset by the
	// relative difference between the line offsets in each file after this hunk.
	if line >= int(hunk.OrigStartLine+hunk.OrigLines) {
		endOfSourceHunk := int(hunk.OrigStartLine + hunk.OrigLines)
		endOfTargetHunk := int(hunk.NewStartLine + hunk.NewLines)
		targetCommitLineNumber := line + (endOfTargetHunk - endOfSourceHunk)

		// Translate from git diff one-index to bundle/lsp zero-index
		return targetCommitLineNumber - 1, true
	}

	// These offsets start at the beginning of the hunk's delta. The following loop will
	// process the delta line-by-line. For each line that exists the source (orig) or
	// target (new) file, the corresponding offset will be bumped. The values of these
	// offsets once we hit our target line will determine the relative offset between
	// the two files.
	sourceOffset := int(hunk.OrigStartLine)
	targetOffset := int(hunk.NewStartLine)

	for _, deltaLine := range strings.Split(string(hunk.Body), "\n") {
		isAdded := strings.HasPrefix(deltaLine, "+")
		isRemoved := strings.HasPrefix(deltaLine, "-")

		// A line exists in the source file if it wasn't added in the delta. We set
		// this before the next condition so that our comparison with our target line
		// is correct.
		if !isAdded {
			sourceOffset++
		}

		// Hit our target line
		if sourceOffset-1 == line {
			// This particular line was (1) edited; (2) removed, or (3) added.
			// If it was removed, there is nothing to point to in the target file.
			// If it was added, then we don't have any index information for it in
			// our source file. In any case, we won't have a precise translation.
			if isAdded || isRemoved {
				return 0, false
			}

			// Translate from git diff one-index to bundle/lsp zero-index
			return targetOffset - 1, true
		}

		// A line exists in the target file if it wasn't deleted in the delta. We set
		// this after the previous condition so we don't have to re-set the target offset
		// within the exit conditions (this adjustment is only necessary for future iterations).
		if !isRemoved {
			targetOffset++
		}
	}

	// This should never happen unless the git diff content is malformed. We know
	// the target line occurs within the hunk, but iteration of the hunk's body did
	// not contain enough lines attributed to the original file.
	panic("Malformed hunk body")
}

func makeKey(parts ...string) string {
	return strings.Join(parts, ":")
}
