package result

import (
	"cmp"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/search/filter"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

// Match is *FileMatch | *RepoMatch | *CommitMatch. We have a private method
// to ensure only those types implement Match.
type Match interface {
	ResultCount() int

	// Limit truncates the match such that, after limiting,
	// `Match.ResultCount() == limit`. It should never be called with
	// `limit <= 0`, since a single match cannot be truncated to zero results.
	Limit(int) int

	Select(filter.SelectPath) Match
	RepoName() types.MinimalRepo

	// Key returns a key which uniquely identifies this match.
	Key() Key

	// ensure only types in this package can be a Match.
	searchResultMarker()
}

// Guard to ensure all match types implement the interface
var (
	_ Match = (*FileMatch)(nil)
	_ Match = (*RepoMatch)(nil)
	_ Match = (*CommitMatch)(nil)
	_ Match = (*CommitDiffMatch)(nil)
	_ Match = (*OwnerMatch)(nil)
)

// Match ranks are used for sorting the different match types.
// Match types with lower ranks will be sorted before match types
// with higher ranks.
const (
	rankFileMatch   = 0
	rankCommitMatch = 1
	rankDiffMatch   = 2
	rankRepoMatch   = 3
	rankOwnerMatch  = 4
)

// Key is a sorting or deduplicating key for a Match. It contains all the
// identifying information for the Match. Keys must be comparable by struct
// equality. If two matches have keys that are equal by struct equality, they
// will be treated as the same result for the purpose of deduplication/merging
// in and/or queries.
type Key struct {
	// Repo is the name of the repo the match belongs to
	Repo api.RepoName

	// Rev is the revision associated with the repo if it exists
	Rev string

	// AuthorDate is the date a commit was authored if this key is for
	// a commit match.
	//
	// NOTE(@camdencheek): this should probably use committer date,
	// but the CommitterField on our CommitMatch type is possibly null,
	// so using AuthorDate here preserves previous sorting behavior.
	AuthorDate time.Time

	// Commit is the commit hash of the commit the match belongs to.
	// Empty if there is no commit associated with the match (e.g. RepoMatch)
	Commit api.CommitID

	// Path is the path of the file the match belongs to.
	// Empty if there is no file associated with the match (e.g. RepoMatch or CommitMatch)
	Path string

	// OwnerMetadata gives uniquely identifying information about an owner.
	// Empty if this is not a Key for an OwnerMatch.
	OwnerMetadata string

	// TypeRank is the sorting rank of the type this key belongs to.
	TypeRank int
}

// Less compares one key to another for sorting
func (k Key) Less(other Key) bool {
	return k.Compare(other) < 0
}

// Compare k to other for sorting
func (k Key) Compare(other Key) int {
	if v := cmp.Compare(k.Repo, other.Repo); v != 0 {
		return v
	}

	if v := cmp.Compare(k.Rev, other.Rev); v != 0 {
		return v
	}

	if v := k.AuthorDate.Compare(other.AuthorDate); v != 0 {
		return v
	}

	if v := cmp.Compare(k.Commit, other.Commit); v != 0 {
		return v
	}

	if v := cmp.Compare(k.Path, other.Path); v != 0 {
		return v
	}

	if v := cmp.Compare(k.OwnerMetadata, other.OwnerMetadata); v != 0 {
		return v
	}

	return cmp.Compare(k.TypeRank, other.TypeRank)
}

// Matches implements sort.Interface
type Matches []Match

func (m Matches) Len() int           { return len(m) }
func (m Matches) Less(i, j int) bool { return m[i].Key().Less(m[j].Key()) }
func (m Matches) Swap(i, j int)      { m[i], m[j] = m[j], m[i] }

// Limit truncates the slice of matches such that, after limiting, `m.ResultCount() == limit`
func (m *Matches) Limit(limit int) int {
	for i, match := range *m {
		if limit <= 0 {
			*m = (*m)[:i]
			return 0
		}
		limit = match.Limit(limit)
	}
	return limit
}

// ResultCount returns the sum of the result counts of each match in the slice
func (m Matches) ResultCount() int {
	count := 0
	for _, match := range m {
		count += match.ResultCount()
	}
	return count
}

// Deduper is a convenience type to deduplicate matches
type Deduper map[Key]struct{}

func NewDeduper() Deduper {
	return make(map[Key]struct{})
}

func (d Deduper) Add(m Match) {
	d[m.Key()] = struct{}{}
}

func (d Deduper) Seen(m Match) bool {
	_, ok := d[m.Key()]
	return ok
}
