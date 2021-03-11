package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/honey"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"
)

type Commit struct {
	ID        api.CommitID `json:"ID,omitempty"`
	Author    Signature    `json:"Author"`
	Committer *Signature   `json:"Committer,omitempty"`
	Message   Message      `json:"Message,omitempty"`
	// Parents are the commit IDs of this commit's parent commits.
	Parents []api.CommitID `json:"Parents,omitempty"`
}

type Message string

// Subject returns the first line of the commit message
func (m Message) Subject() string {
	message := string(m)
	i := strings.Index(message, "\n")
	if i == -1 {
		return strings.TrimSpace(message)
	}
	return strings.TrimSpace(message[:i])
}

// Body returns the contents of the Git commit message after the subject.
func (m Message) Body() string {
	message := string(m)
	i := strings.Index(message, "\n")
	if i == -1 {
		return ""
	}
	return strings.TrimSpace(message[i:])
}

type Signature struct {
	Name  string    `json:"Name,omitempty"`
	Email string    `json:"Email,omitempty"`
	Date  time.Time `json:"Date"`
}

// CommitsOptions specifies options for (Repository).Commits (Repository).CommitCount.
type CommitsOptions struct {
	Range string // commit range (revspec, "A..B", "A...B", etc.)

	N    uint // limit the number of returned commits to this many (0 means no limit)
	Skip uint // skip this many commits at the beginning

	MessageQuery string // include only commits whose commit message contains this substring

	Author string // include only commits whose author matches this
	After  string // include only commits after this date
	Before string // include only commits before this date

	Reverse   bool // Whether or not commits should be given in reverse order (optional)
	DateOrder bool // Whether or not commits should be sorted by date (optional)

	Path string // only commits modifying the given path are selected (optional)

	// When true we opt out of attempting to fetch missing revisions
	NoEnsureRevision bool
}

// logEntryPattern is the regexp pattern that matches entries in the output of the `git shortlog
// -sne` command.
var logEntryPattern = lazyregexp.New(`^\s*([0-9]+)\s+(.*)$`)

var recordGetCommitQueries = os.Getenv("RECORD_GET_COMMIT_QUERIES") == "1"

// getCommit returns the commit with the given id.
func getCommit(ctx context.Context, repo api.RepoName, id api.CommitID, opt ResolveRevisionOptions) (_ *Commit, err error) {
	if Mocks.GetCommit != nil {
		return Mocks.GetCommit(id)
	}

	if honey.Enabled() && recordGetCommitQueries {
		defer func() {
			ev := honey.Event("getCommit")
			ev.SampleRate = 10 // 1 in 10
			ev.AddField("repo", repo)
			ev.AddField("commit", id)
			ev.AddField("no_ensure_revision", opt.NoEnsureRevision)
			ev.AddField("actor", actor.FromContext(ctx).UIDString())

			q, _ := ctx.Value("graphql-query").(string)
			ev.AddField("query", q)

			if err != nil {
				ev.AddField("error", err.Error())
			}

			_ = ev.Send()
		}()
	}

	if err := checkSpecArgSafety(string(id)); err != nil {
		return nil, err
	}

	commitOptions := CommitsOptions{
		Range:            string(id),
		N:                1,
		NoEnsureRevision: opt.NoEnsureRevision,
	}

	commits, err := commitLog(ctx, repo, commitOptions)
	if err != nil {
		return nil, err
	}

	if len(commits) != 1 {
		return nil, fmt.Errorf("git log: expected 1 commit, got %d", len(commits))
	}

	return commits[0], nil
}

// GetCommit returns the commit with the given commit ID, or ErrCommitNotFound if no such commit
// exists.
//
// The remoteURLFunc is called to get the Git remote URL if it's not set in repo and if it is
// needed. The Git remote URL is only required if the gitserver doesn't already contain a clone of
// the repository or if the commit must be fetched from the remote.
func GetCommit(ctx context.Context, repo api.RepoName, id api.CommitID, opt ResolveRevisionOptions) (*Commit, error) {
	span, ctx := ot.StartSpanFromContext(ctx, "Git: GetCommit")
	span.SetTag("Commit", id)
	defer span.Finish()

	return getCommit(ctx, repo, id, opt)
}

// Commits returns all commits matching the options.
func Commits(ctx context.Context, repo api.RepoName, opt CommitsOptions) ([]*Commit, error) {
	if Mocks.Commits != nil {
		return Mocks.Commits(repo, opt)
	}

	span, ctx := ot.StartSpanFromContext(ctx, "Git: Commits")
	span.SetTag("Opt", opt)
	defer span.Finish()

	if err := checkSpecArgSafety(opt.Range); err != nil {
		return nil, err
	}

	return commitLog(ctx, repo, opt)
}

// HasCommitAfter indicates the staleness of a repository. It returns a boolean indicating if a repository
// contains a commit past a specified date.
func HasCommitAfter(ctx context.Context, repo api.RepoName, date string, revspec string) (bool, error) {
	span, ctx := ot.StartSpanFromContext(ctx, "Git: HasCommitAfter")
	span.SetTag("Date", date)
	span.SetTag("RevSpec", revspec)
	defer span.Finish()

	if revspec == "" {
		revspec = "HEAD"
	}

	commitid, err := ResolveRevision(ctx, repo, revspec, ResolveRevisionOptions{NoEnsureRevision: true})
	if err != nil {
		return false, err
	}

	n, err := CommitCount(ctx, repo, CommitsOptions{
		N:     1,
		After: date,
		Range: string(commitid),
	})
	return n > 0, err
}

func isBadObjectErr(output, obj string) bool {
	return output == "fatal: bad object "+obj
}

func isInvalidRevisionRangeError(output, obj string) bool {
	return strings.HasPrefix(output, "fatal: Invalid revision range "+obj)
}

// commitLog returns a list of commits.
//
// The caller is responsible for doing checkSpecArgSafety on opt.Head and opt.Base.
func commitLog(ctx context.Context, repo api.RepoName, opt CommitsOptions) (commits []*Commit, err error) {
	args, err := commitLogArgs([]string{"log", logFormatWithoutRefs}, opt)
	if err != nil {
		return nil, err
	}

	cmd := gitserver.DefaultClient.Command("git", args...)
	cmd.Repo = repo
	if !opt.NoEnsureRevision {
		cmd.EnsureRevision = opt.Range
	}
	return runCommitLog(ctx, cmd, opt)
}

// runCommitLog sends the git command to gitserver. It interprets missing
// revision responses and converts them into RevisionNotFoundError.
// It is declared as a variable so that we can swap it out in tests
var runCommitLog = func(ctx context.Context, cmd *gitserver.Cmd, opt CommitsOptions) ([]*Commit, error) {
	data, stderr, err := cmd.DividedOutput(ctx)
	if err != nil {
		data = bytes.TrimSpace(data)
		if isBadObjectErr(string(stderr), opt.Range) {
			return nil, &gitserver.RevisionNotFoundError{Repo: cmd.Repo, Spec: opt.Range}
		}
		return nil, errors.WithMessage(err, fmt.Sprintf("git command %v failed (output: %q)", cmd.Args, data))
	}

	allParts := bytes.Split(data, []byte{'\x00'})
	numCommits := len(allParts) / partsPerCommit
	commits := make([]*Commit, 0, numCommits)
	for len(data) > 0 {
		var commit *Commit
		var err error
		commit, _, data, err = parseCommitFromLog(data)
		if err != nil {
			return nil, err
		}
		commits = append(commits, commit)
	}
	return commits, nil
}

func commitLogArgs(initialArgs []string, opt CommitsOptions) (args []string, err error) {
	if err := checkSpecArgSafety(opt.Range); err != nil {
		return nil, err
	}

	args = initialArgs
	if opt.N != 0 {
		args = append(args, "-n", strconv.FormatUint(uint64(opt.N), 10))
	}
	if opt.Skip != 0 {
		args = append(args, "--skip="+strconv.FormatUint(uint64(opt.Skip), 10))
	}

	if opt.Author != "" {
		args = append(args, "--fixed-strings", "--author="+opt.Author)
	}

	if opt.After != "" {
		args = append(args, "--after="+opt.After)
	}
	if opt.Before != "" {
		args = append(args, "--before="+opt.Before)
	}
	if opt.Reverse {
		args = append(args, "--reverse")
	}
	if opt.DateOrder {
		args = append(args, "--date-order")
	}

	if opt.MessageQuery != "" {
		args = append(args, "--fixed-strings", "--regexp-ignore-case", "--grep="+opt.MessageQuery)
	}

	if opt.Range != "" {
		args = append(args, opt.Range)
	}

	if opt.Path != "" {
		args = append(args, "--", opt.Path)
	}
	return args, nil
}

// CommitCount returns the number of commits that would be returned by Commits.
func CommitCount(ctx context.Context, repo api.RepoName, opt CommitsOptions) (uint, error) {
	span, ctx := ot.StartSpanFromContext(ctx, "Git: CommitCount")
	span.SetTag("Opt", opt)
	defer span.Finish()

	args, err := commitLogArgs([]string{"rev-list", "--count"}, opt)
	if err != nil {
		return 0, err
	}

	cmd := gitserver.DefaultClient.Command("git", args...)
	cmd.Repo = repo
	if opt.Path != "" {
		// This doesn't include --follow flag because rev-list doesn't support it, so the number may be slightly off.
		cmd.Args = append(cmd.Args, "--", opt.Path)
	}
	out, err := cmd.CombinedOutput(ctx)
	if err != nil {
		return 0, errors.WithMessage(err, fmt.Sprintf("git command %v failed (output: %q)", cmd.Args, out))
	}
	out = bytes.TrimSpace(out)
	n, err := strconv.ParseUint(string(out), 10, 64)
	return uint(n), err
}

// FirstEverCommit returns the first commit ever made to the repository.
func FirstEverCommit(ctx context.Context, repo api.RepoName) (*Commit, error) {
	span, ctx := ot.StartSpanFromContext(ctx, "Git: FirstEverCommit")
	defer span.Finish()

	args := []string{"rev-list", "--max-count=1", "--max-parents=0", "HEAD"}
	cmd := gitserver.DefaultClient.Command("git", args...)
	cmd.Repo = repo
	out, err := cmd.CombinedOutput(ctx)
	if err != nil {
		return nil, errors.WithMessage(err, fmt.Sprintf("git command %v failed (output: %q)", args, out))
	}
	id := api.CommitID(bytes.TrimSpace(out))
	return GetCommit(ctx, repo, id, ResolveRevisionOptions{NoEnsureRevision: true})
}

// FindNearestCommit finds the commit in the given repository revSpec (e.g. `HEAD` or `mybranch`)
// whose author date most closely matches the target time.
//
// Can return a commit very far away if no nearby one exists.
// Can theoretically return nil, nil if no commits at all are found.
func FindNearestCommit(ctx context.Context, repoName api.RepoName, revSpec string, target time.Time) (*Commit, error) {
	if revSpec == "" {
		revSpec = "HEAD"
	}
	// Resolve e.g. the branch we're looking at.
	branchCommit, err := ResolveRevision(ctx, repoName, revSpec, ResolveRevisionOptions{NoEnsureRevision: true})
	if err != nil {
		return nil, err
	}

	// Find the next closest commit on or after our target time.
	commitsAfter, err := Commits(ctx, repoName, CommitsOptions{
		After:     target.Add(-1 * time.Second).Format(time.RFC3339),
		Range:     string(branchCommit),
		Reverse:   true,
		DateOrder: true,
	})
	if err != nil {
		return nil, err
	}
	var commitOnOrAfter *Commit
	if len(commitsAfter) > 0 {
		commitOnOrAfter = commitsAfter[0]
	}

	// Find the next closest commit before our target time.
	commitsBefore, err := Commits(ctx, repoName, CommitsOptions{
		N:         1,
		Before:    target.Format(time.RFC3339),
		Range:     string(branchCommit),
		DateOrder: true,
	})
	if err != nil {
		return nil, err
	}
	var commitBefore *Commit
	if len(commitsBefore) > 0 {
		commitBefore = commitsBefore[0]
	}

	switch {
	case commitOnOrAfter == nil && commitBefore == nil:
		return nil, nil
	case commitOnOrAfter == nil:
		return commitBefore, nil
	case commitBefore == nil:
		return commitOnOrAfter, nil
	default:
		// Get absolute distance of each commit to target.
		distanceToAfter := commitOnOrAfter.Author.Date.Sub(target)
		if distanceToAfter < 0 {
			distanceToAfter = -distanceToAfter
		}
		distanceToBefore := commitBefore.Author.Date.Sub(target)
		if distanceToBefore < 0 {
			distanceToBefore = -distanceToBefore
		}

		// Return whichever commit is closer.
		if distanceToAfter < distanceToBefore {
			return commitOnOrAfter, nil
		}
		return commitBefore, nil
	}
}

const (
	partsPerCommit = 10 // number of \x00-separated fields per commit

	// include refs (slow on repos with many refs)
	logFormatWithRefs = "--format=format:%H%x00%D%x00%aN%x00%aE%x00%at%x00%cN%x00%cE%x00%ct%x00%B%x00%P%x00"

	// don't include refs (faster, should be used if refs are not needed)
	logFormatWithoutRefs = "--format=format:%H%x00%x00%aN%x00%aE%x00%at%x00%cN%x00%cE%x00%ct%x00%B%x00%P%x00"
)

// parseCommitFromLog parses the next commit from data and returns the commit and the remaining
// data. The data arg is a byte array that contains NUL-separated log fields as formatted by
// logFormatFlag.
func parseCommitFromLog(data []byte) (commit *Commit, refs []string, rest []byte, err error) {
	parts := bytes.SplitN(data, []byte{'\x00'}, partsPerCommit+1)
	if len(parts) < partsPerCommit {
		return nil, nil, nil, fmt.Errorf("invalid commit log entry: %q", parts)
	}

	// log outputs are newline separated, so all but the 1st commit ID part
	// has an erroneous leading newline.
	parts[0] = bytes.TrimPrefix(parts[0], []byte{'\n'})
	commitID := api.CommitID(parts[0])

	authorTime, err := strconv.ParseInt(string(parts[4]), 10, 64)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parsing git commit author time: %s", err)
	}
	committerTime, err := strconv.ParseInt(string(parts[7]), 10, 64)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("parsing git commit committer time: %s", err)
	}

	var parents []api.CommitID
	if parentPart := parts[9]; len(parentPart) > 0 {
		parentIDs := bytes.Split(parentPart, []byte{' '})
		parents = make([]api.CommitID, len(parentIDs))
		for i, id := range parentIDs {
			parents[i] = api.CommitID(id)
		}
	}

	if len(parts[1]) > 0 {
		refs = strings.Split(string(parts[1]), ", ")
	}

	commit = &Commit{
		ID:        commitID,
		Author:    Signature{Name: string(parts[2]), Email: string(parts[3]), Date: time.Unix(authorTime, 0).UTC()},
		Committer: &Signature{Name: string(parts[5]), Email: string(parts[6]), Date: time.Unix(committerTime, 0).UTC()},
		Message:   Message(strings.TrimSuffix(string(parts[8]), "\n")),
		Parents:   parents,
	}

	if len(parts) == partsPerCommit+1 {
		rest = parts[10]
	}

	return commit, refs, rest, nil
}

// onelineCommit contains (a subset of the) information about a commit returned
// by `git log --oneline --source`.
type onelineCommit struct {
	sha1      string // sha1 commit ID
	sourceRef string // `git log --source` source ref
}

// errLogOnelineBatchScannerClosed is returned if a read is attempted on a
// closed logOnelineBatchScanner.
var errLogOnelineBatchScannerClosed = errors.New("logOnelineBatchScanner closed")

// logOnelineBatchScanner wraps logOnelineScanner to batch up reads.
//
// next will return at least 1 commit and at most maxBatchSize entries. After
// debounce time it will return what has been batched so far (or wait until an
// entry is available). The last response from next will return a non-nil
// error.
//
// cleanup must be called when done. This function creates a goroutine to
// batch up calls to scan.
func logOnelineBatchScanner(scan func() (*onelineCommit, error), maxBatchSize int, debounce time.Duration) (next func() ([]*onelineCommit, error), cleanup func()) {
	// done is closed to indicate cleaning up
	done := make(chan struct{})
	cleanup = func() {
		close(done)
	}

	// We can't call scan with a timeout. So we start a goroutine which sends
	// the output of scan down a channel. When done is closed, this goroutine
	// will stop running.
	type result struct {
		commit *onelineCommit
		err    error
	}
	// have a cap of maxBatchSize so we potentially have maxBatchSize ready to
	// send.
	resultC := make(chan result, maxBatchSize)
	go func() {
		defer close(resultC)
		for {
			commit, err := scan()
			select {
			case resultC <- result{commit: commit, err: err}:
			case <-done:
				return
			}
			if err != nil {
				return
			}
		}
	}()

	var err error
	next = func() ([]*onelineCommit, error) {
		// The previous call may have had an error but also had commits to
		// return. In that case we didn't return the error, so need to return
		// it now.
		if err != nil {
			return nil, err
		}

		// Stop scanning if we haven't found maxBatchSize by deadline.
		deadline := time.NewTimer(debounce)
		defer deadline.Stop()

		// We always want 1 result. Ignore deadline for the first result.
		r, ok := <-resultC
		if !ok {
			return nil, errLogOnelineBatchScannerClosed
		} else if r.err != nil {
			err = r.err
			return nil, r.err
		}

		// Add to commits until we hit maxBatchSize or we are passed
		// deadline. If we encounter an error set err.
		commits := []*onelineCommit{r.commit}
		timedout := false
		for err == nil && len(commits) < maxBatchSize && !timedout {
			select {
			case r, ok := <-resultC:
				if !ok {
					err = errLogOnelineBatchScannerClosed
				} else if r.err != nil {
					err = r.err
				} else {
					commits = append(commits, r.commit)
				}
			case <-deadline.C:
				timedout = true
			}
		}

		// Note: err can be non-nil here. The next call to this
		// function will return err.
		return commits, nil
	}

	return next, cleanup
}

// logOnelineScanner parses the commits from the reader of:
//
//   git log --pretty='format:%H %S' -z --source --no-patch
//
// Once it returns an error the scanner should be disregarded. io.EOF is
// returned when there is no more data to read.
func logOnelineScanner(r io.Reader) func() (*onelineCommit, error) {
	// Each "log line" contains a source ref. I could not find a bound on the
	// size of a git ref, so each line can get arbitrarily large. So we use a
	// bufio.Scanner instead of a bufio.Reader since a Scanner allows growing
	// the buffer to accomodate the "token" size. This makes the
	// implementation slightly more complicated (needs a split function
	// instead of just using ReadBytes).
	//
	// Note: Not all source refs correspond to direct arguments, eg if you use
	// --glob=refs/* any possible ref can be a source ref.
	//
	// Note: I check the git source for ref limits, there are none I
	// found. Linux does have PATH_MAX (4096), but its quite easy to work
	// around that.
	//
	// Note: Scanner does have a max size it will grow to (64kb). If a repo
	// contains a ref this big, we treat it as an error. This shouldn't happen
	// in practice, but that is likely famous last words.
	scanNull := func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if atEOF && len(data) == 0 {
			return 0, nil, nil
		}
		if i := bytes.IndexByte(data, '\x00'); i >= 0 {
			return i + 1, data[:i], nil
		}
		if atEOF {
			return len(data), data, nil
		}
		// Request more data.
		return 0, nil, nil
	}
	scanner := bufio.NewScanner(r)
	scanner.Split(scanNull)
	return func() (*onelineCommit, error) {
		if !scanner.Scan() {
			err := scanner.Err()
			if err == nil {
				return nil, io.EOF
			} else if strings.Contains(err.Error(), "does not have any commits yet") {
				// Treat empty repositories as having no commits without failing
				return nil, io.EOF
			}
			return nil, err
		}

		e := scanner.Bytes()

		// Format: (40-char SHA) (source ref)
		if len(e) <= 42 {
			return nil, fmt.Errorf("parsing git oneline commit: short entry: %q", e)
		}
		sha1 := e[:40]
		if e[40] != ' ' {
			return nil, fmt.Errorf("parsing git oneline commit: no ' ': %q", e)
		}
		sourceRef := e[41:]
		return &onelineCommit{
			sha1:      string(sha1),
			sourceRef: string(sourceRef),
		}, nil
	}
}
