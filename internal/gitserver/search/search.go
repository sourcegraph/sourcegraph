package search

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"os/exec"
	"strings"

	godiff "github.com/sourcegraph/go-diff/diff"
	"github.com/sourcegraph/log"
	"golang.org/x/sync/errgroup"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/search/result"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Git formatting directives as described in man git-log (see PRETTY FORMATS)
const (
	hash           = "%H"
	refNames       = "%D"
	sourceRefs     = "%S"
	authorName     = "%aN"
	authorEmail    = "%aE"
	authorDate     = "%at"
	committerName  = "%cN"
	committerEmail = "%cE"
	committerDate  = "%ct"
	rawBody        = "%B"
	parentHashes   = "%P"
)

var (
	commitFields = []string{
		hash,
		refNames,
		sourceRefs,
		authorName,
		authorEmail,
		authorDate,
		committerName,
		committerEmail,
		committerDate,
		rawBody,
		parentHashes,
	}

	// commitSeparator is a special ascii code we use to separate each commit, the
	// ASCII record separator:
	// https://www.asciihex.com/character/control/30/0x1E/rs-record-separator. This
	// is required since the number of zero byte separators per commit changes
	// depending on the number of files modified in the commit.
	commitSeparator = []byte("\x1E")

	// Note that we begin each commit with a special string constant. This allows us
	// to easily separate each commit since the number of parts in each commit varies
	// depending on the number of files modified.
	logArgs = []string{
		"log",
		"--decorate=full",
		"-z",
		"--format=format:" + "%x1E" + strings.Join(commitFields, "%x00") + "%x00",
	}

	sep = []byte{0x0}
)

type job struct {
	batch      []*RawCommit
	resultChan chan *protocol.CommitMatch
}

const (
	// The size of a batch of commits sent in each worker job
	batchSize  = 512
	numWorkers = 4
)

type CommitSearcher struct {
	Logger               log.Logger
	RepoDir              string
	Query                MatchTree
	Revisions            []string
	IncludeDiff          bool
	IncludeModifiedFiles bool
	RepoName             api.RepoName
}

// Search runs a search for commits matching the given predicate across the revisions passed in as revisionArgs.
//
// We have some slightly complex logic here in order to run searches in parallel (big benefit to diff searches),
// but also return results in order. We first iterate over all the commits using the hard-coded git log arguments.
// We batch the shallowly-parsed commits, then send them on the jobs channel along with a channel that results for
// that job should be sent down. We then read from the result channels in the same order that the jobs were sent.
// This allows our worker pool to run the jobs in parallel, but we still emit matches in the same order that
// git log outputs them.
func (cs *CommitSearcher) Search(ctx context.Context, onMatch func(*protocol.CommitMatch)) error {
	g, ctx := errgroup.WithContext(ctx)

	jobs := make(chan job, 128)
	resultChans := make(chan chan *protocol.CommitMatch, 128)

	// Start feeder
	g.Go(func() error {
		defer close(resultChans)
		defer close(jobs)
		return cs.feedBatches(ctx, jobs, resultChans)
	})

	// Start workers
	for range numWorkers {
		g.Go(func() error {
			return cs.runJobs(ctx, jobs)
		})
	}

	// Consumer goroutine that consumes results in the order jobs were
	// submitted to the job queue
	g.Go(func() error {
		for resultChan := range resultChans {
			for res := range resultChan {
				onMatch(res)
			}
		}

		return nil
	})

	return g.Wait()
}

func (cs *CommitSearcher) gitArgs() []string {
	revArgs := revsToGitArgs(cs.Revisions)
	args := append(logArgs, revArgs...)
	if cs.IncludeModifiedFiles {
		args = append(args, "--name-status")
	}
	return args
}

func revsToGitArgs(revs []string) []string {
	revArgs := make([]string, 0, len(revs))
	for _, rev := range revs {
		if rev != "" {
			revArgs = append(revArgs, rev)
		} else {
			revArgs = append(revArgs, "HEAD")
		}
	}
	return revArgs
}

func (cs *CommitSearcher) feedBatches(ctx context.Context, jobs chan job, resultChans chan chan *protocol.CommitMatch) (err error) {
	cmd := exec.CommandContext(ctx, "git", cs.gitArgs()...)
	cmd.Dir = cs.RepoDir
	stdoutReader, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return err
	}

	defer func() {
		// Always call cmd.Wait to avoid leaving zombie processes around.
		if e := cmd.Wait(); e != nil {
			err = errors.Append(err, tryInterpretErrorWithStderr(ctx, err, stderrBuf.String(), cs.Logger))
		}
	}()

	batch := make([]*RawCommit, 0, batchSize)
	sendBatch := func() {
		resultChan := make(chan *protocol.CommitMatch, 128)
		resultChans <- resultChan
		jobs <- job{
			batch:      batch,
			resultChan: resultChan,
		}
		batch = make([]*RawCommit, 0, batchSize)
	}

	scanner := NewCommitScanner(stdoutReader)
	for scanner.Scan() {
		if ctx.Err() != nil {
			return nil
		}
		cv := scanner.NextRawCommit()
		batch = append(batch, cv)
		if len(batch) == batchSize {
			sendBatch()
		}
	}

	if len(batch) > 0 {
		sendBatch()
	}

	return scanner.Err()
}

func tryInterpretErrorWithStderr(ctx context.Context, err error, stderr string, logger log.Logger) error {
	if ctx.Err() != nil {
		// Ignore errors when context is cancelled
		return nil
	}
	if strings.Contains(stderr, "does not have any commits yet") {
		// Ignore no commits error error
		return nil
	}
	logger.Warn("git search command exited with non-zero status code", log.String("stderr", stderr))
	return err
}

func getSubRepoFilterFunc(ctx context.Context, checker authz.SubRepoPermissionChecker, repo api.RepoName) func(string) (bool, error) {
	if !authz.SubRepoEnabled(checker) {
		return nil
	}
	a := actor.FromContext(ctx)
	return func(filePath string) (bool, error) {
		return authz.FilterActorPath(ctx, checker, a, repo, filePath)
	}
}

func (cs *CommitSearcher) runJobs(ctx context.Context, jobs chan job) error {
	// Create a new diff fetcher subprocess for each worker
	diffFetcher, err := NewDiffFetcher(cs.RepoDir)
	if err != nil {
		return err
	}
	defer diffFetcher.Stop()

	startBuf := make([]byte, 1024)

	runJob := func(j job) error {
		defer close(j.resultChan)

		for _, cv := range j.batch {
			if ctx.Err() != nil {
				// ignore context error, and don't spend time running the job
				return nil
			}

			lc := &LazyCommit{
				RawCommit:   cv,
				diffFetcher: diffFetcher,
				LowerBuf:    startBuf,
			}
			mergedResult, highlights, err := cs.Query.Match(lc)
			if err != nil {
				return err
			}
			if mergedResult.Satisfies() {
				cm, err := CreateCommitMatch(lc, highlights, cs.IncludeDiff, getSubRepoFilterFunc(ctx, authz.DefaultSubRepoPermsChecker, cs.RepoName))
				if err != nil {
					return err
				}
				j.resultChan <- cm
			}
		}
		return nil
	}

	var errs error
	for j := range jobs {
		errs = errors.Append(errs, runJob(j))
	}
	return errs
}

// RawCommit is a shallow parse of the output of git log
type RawCommit struct {
	Hash           []byte
	RefNames       []byte
	SourceRefs     []byte
	AuthorName     []byte
	AuthorEmail    []byte
	AuthorDate     []byte
	CommitterName  []byte
	CommitterEmail []byte
	CommitterDate  []byte
	Message        []byte
	ParentHashes   []byte
	ModifiedFiles  [][]byte
}

type CommitScanner struct {
	scanner *bufio.Scanner
	next    *RawCommit
	err     error
}

// NewCommitScanner creates a scanner that does a shallow parse of the stdout of git log.
// Like the bufio.Scanner() API, call Scan() to ingest the next result, which will return
// false if it hits an error or EOF, then call NextRawCommit() to get the scanned commit.
func NewCommitScanner(r io.Reader) *CommitScanner {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1024), 1<<22)

	// Split by commit
	scanner.Split(func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		if len(data) == 0 {
			if !atEOF {
				// Read more data
				return 0, nil, nil
			}
			return 0, nil, errors.Errorf("incomplete data")
		}

		if !bytes.HasPrefix(data, commitSeparator) {
			// Each commit should always start with our separator
			return 0, nil, errors.Errorf("expected commit separator")
		}

		// Find the index of the next separator
		idx := bytes.Index(data[1:], commitSeparator)
		if idx == -1 {
			if !atEOF {
				return 0, nil, nil
			}
			return len(data), data[1:], nil
		}
		token = data[1 : idx+1]

		return len(token) + 1, token, nil
	})

	return &CommitScanner{
		scanner: scanner,
	}
}

func (c *CommitScanner) Scan() bool {
	if !c.scanner.Scan() {
		return false
	}

	// Make a copy so the view can outlive the next scan
	buf := make([]byte, len(c.scanner.Bytes()))
	copy(buf, c.scanner.Bytes())

	parts := bytes.Split(buf, sep)
	if len(parts) < len(commitFields) {
		c.err = errors.Errorf("invalid commit log entry: %q", parts)
		return false
	}

	// Filter out empty modified files, which can happen due to how
	// --name-status formats its output. Also trim spaces on the files
	// for the same reason.
	modifiedFiles := parts[11:11]
	for _, part := range parts[11:] {
		if len(part) > 0 {
			modifiedFiles = append(modifiedFiles, bytes.TrimSpace(part))
		}
	}

	c.next = &RawCommit{
		Hash:           parts[0],
		RefNames:       parts[1],
		SourceRefs:     parts[2],
		AuthorName:     parts[3],
		AuthorEmail:    parts[4],
		AuthorDate:     parts[5],
		CommitterName:  parts[6],
		CommitterEmail: parts[7],
		CommitterDate:  parts[8],
		Message:        bytes.TrimSpace(parts[9]),
		ParentHashes:   parts[10],
		ModifiedFiles:  modifiedFiles,
	}

	return true
}

func (c *CommitScanner) NextRawCommit() *RawCommit {
	return c.next
}

func (c *CommitScanner) Err() error {
	return c.err
}

func CreateCommitMatch(lc *LazyCommit, hc MatchedCommit, includeDiff bool, filterFunc func(string) (bool, error)) (*protocol.CommitMatch, error) {
	authorDate, err := lc.AuthorDate()
	if err != nil {
		return nil, err
	}

	committerDate, err := lc.CommitterDate()
	if err != nil {
		return nil, err
	}

	diff := result.MatchedString{}
	if includeDiff {
		rawDiff, err := lc.Diff()
		if err != nil {
			return nil, err
		}
		rawDiff = filterRawDiff(rawDiff, filterFunc)
		diff.Content, diff.MatchedRanges = FormatDiff(rawDiff, hc.Diff)
	}

	commitID, err := api.NewCommitID(string(lc.Hash))
	if err != nil {
		return nil, err
	}

	parentIDs, err := lc.ParentIDs()
	if err != nil {
		return nil, err
	}

	return &protocol.CommitMatch{
		Oid: commitID,
		Author: protocol.Signature{
			Name:  utf8String(lc.AuthorName),
			Email: utf8String(lc.AuthorEmail),
			Date:  authorDate,
		},
		Committer: protocol.Signature{
			Name:  utf8String(lc.CommitterName),
			Email: utf8String(lc.CommitterEmail),
			Date:  committerDate,
		},
		Parents:    parentIDs,
		SourceRefs: lc.SourceRefs(),
		Refs:       lc.RefNames(),
		Message: result.MatchedString{
			Content:       utf8String(lc.Message),
			MatchedRanges: hc.Message,
		},
		Diff:          diff,
		ModifiedFiles: lc.ModifiedFiles(),
	}, nil
}

func utf8String(b []byte) string {
	return string(bytes.ToValidUTF8(b, []byte("�")))
}

func filterRawDiff(rawDiff []*godiff.FileDiff, filterFunc func(string) (bool, error)) []*godiff.FileDiff {
	logger := log.Scoped("filterRawDiff")
	if filterFunc == nil {
		return rawDiff
	}
	filtered := make([]*godiff.FileDiff, 0, len(rawDiff))
	for _, fileDiff := range rawDiff {
		if filterFunc != nil {
			if isAllowed, err := filterFunc(fileDiff.NewName); err != nil {
				logger.Error("error filtering files in raw diff", log.Error(err))
				continue
			} else if !isAllowed {
				continue
			}
		}
		filtered = append(filtered, fileDiff)
	}
	return filtered
}
