package gitserver

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/semaphore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/sourcegraph/conc/pool"
	"github.com/sourcegraph/go-diff/diff"
	sglog "github.com/sourcegraph/log"
	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	proto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	"github.com/sourcegraph/sourcegraph/internal/grpc/streamio"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/lazyregexp"
	"github.com/sourcegraph/sourcegraph/internal/limiter"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/perforce"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

const git = "git"

var (
	clientFactory  = httpcli.NewInternalClientFactory("gitserver")
	defaultDoer, _ = clientFactory.Doer()
	// defaultLimiter limits concurrent HTTP requests per running process to gitserver.
	defaultLimiter = limiter.New(500)
)

var ClientMocks, emptyClientMocks struct {
	GetObject               func(repo api.RepoName, objectName string) (*gitdomain.GitObject, error)
	Archive                 func(ctx context.Context, repo api.RepoName, opt ArchiveOptions) (_ io.ReadCloser, err error)
	LocalGitserver          bool
	LocalGitCommandReposDir string
}

// conns is the global variable holding a reference to the gitserver connections.
var conns = &atomicGitServerConns{}

// ResetClientMocks clears the mock functions set on Mocks (so that subsequent
// tests don't inadvertently use them).
func ResetClientMocks() {
	ClientMocks = emptyClientMocks
}

var _ Client = &clientImplementor{}

// ClientSource is a source of gitserver.Client instances.
// It allows for mocking out the client source in tests.
type ClientSource interface {
	// ClientForRepo returns a Client for the given repo.
	ClientForRepo(ctx context.Context, userAgent string, repo api.RepoName) (proto.GitserverServiceClient, error)
	// AddrForRepo returns the address of the gitserver for the given repo.
	AddrForRepo(ctx context.Context, userAgent string, repo api.RepoName) string
	// Address the current list of gitserver addresses.
	Addresses() []AddressWithClient
	// GetAddressWithClient returns the address and client for a gitserver instance.
	// It returns nil if there's no server with that address
	GetAddressWithClient(addr string) AddressWithClient
}

// NewClient returns a new gitserver.Client.
func NewClient() Client {
	logger := sglog.Scoped("GitserverClient")
	return &clientImplementor{
		logger:      logger,
		httpClient:  defaultDoer,
		httpLimiter: defaultLimiter,
		// Use the binary name for userAgent. This should effectively identify
		// which service is making the request (excluding requests proxied via the
		// frontend internal API)
		userAgent:           filepath.Base(os.Args[0]),
		operations:          getOperations(),
		clientSource:        conns,
		subRepoPermsChecker: authz.DefaultSubRepoPermsChecker,
	}
}

// NewTestClient returns a test client that will us
func NewTestClient(t testing.TB) TestClient {
	logger := logtest.Scoped(t)

	return &clientImplementor{
		logger:      logger,
		httpClient:  http.DefaultClient,
		httpLimiter: limiter.New(500),
		// Use the binary name for userAgent. This should effectively identify
		// which service is making the request (excluding requests proxied via the
		// frontend internal API)
		userAgent:           filepath.Base(os.Args[0]),
		operations:          newOperations(observation.ContextWithLogger(logger, &observation.TestContext)),
		clientSource:        NewTestClientSource(t, nil),
		subRepoPermsChecker: authz.DefaultSubRepoPermsChecker,
	}
}

type TestClient interface {
	Client
	WithChecker(authz.SubRepoPermissionChecker) TestClient
	WithDoer(httpcli.Doer) TestClient
	WithClientSource(ClientSource) TestClient
}

func (c *clientImplementor) WithChecker(checker authz.SubRepoPermissionChecker) TestClient {
	c.subRepoPermsChecker = checker
	return c
}

func (c *clientImplementor) WithDoer(doer httpcli.Doer) TestClient {
	c.httpClient = doer
	return c
}

func (c *clientImplementor) WithClientSource(cs ClientSource) TestClient {
	c.clientSource = cs
	return c
}

// NewMockClientWithExecReader return new MockClient with provided mocked
// behaviour of ExecReader function.
func NewMockClientWithExecReader(checker authz.SubRepoPermissionChecker, execReader func(context.Context, api.RepoName, []string) (io.ReadCloser, error)) *MockClient {
	client := NewMockClient()
	// NOTE: This hook is the same as DiffFunc, but with `execReader` used above
	client.DiffFunc.SetDefaultHook(func(ctx context.Context, opts DiffOptions) (*DiffFileIterator, error) {
		if opts.Base == DevNullSHA {
			opts.RangeType = ".."
		} else if opts.RangeType != ".." {
			opts.RangeType = "..."
		}

		rangeSpec := opts.Base + opts.RangeType + opts.Head
		if strings.HasPrefix(rangeSpec, "-") || strings.HasPrefix(rangeSpec, ".") {
			return nil, errors.Errorf("invalid diff range argument: %q", rangeSpec)
		}

		// Here is where all the mocking happens!
		rdr, err := execReader(ctx, opts.Repo, append([]string{
			"diff",
			"--find-renames",
			"--full-index",
			"--inter-hunk-context=3",
			"--no-prefix",
			rangeSpec,
			"--",
		}, opts.Paths...))
		if err != nil {
			return nil, errors.Wrap(err, "executing git diff")
		}

		return &DiffFileIterator{
			rdr:            rdr,
			mfdr:           diff.NewMultiFileDiffReader(rdr),
			fileFilterFunc: getFilterFunc(ctx, checker, opts.Repo),
		}, nil
	})

	// NOTE: This hook is the same as DiffPath, but with `execReader` used above
	client.DiffPathFunc.SetDefaultHook(func(ctx context.Context, repo api.RepoName, sourceCommit, targetCommit, path string) ([]*diff.Hunk, error) {
		a := actor.FromContext(ctx)
		if hasAccess, err := authz.FilterActorPath(ctx, checker, a, repo, path); err != nil {
			return nil, err
		} else if !hasAccess {
			return nil, os.ErrNotExist
		}
		// Here is where all the mocking happens!
		reader, err := execReader(ctx, repo, []string{"diff", sourceCommit, targetCommit, "--", path})
		if err != nil {
			return nil, err
		}
		defer reader.Close()

		output, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}
		if len(output) == 0 {
			return nil, nil
		}

		d, err := diff.NewFileDiffReader(bytes.NewReader(output)).Read()
		if err != nil {
			return nil, err
		}
		return d.Hunks, nil
	})

	return client
}

// clientImplementor is a gitserver client.
type clientImplementor struct {
	// Limits concurrency of outstanding HTTP posts
	httpLimiter limiter.Limiter

	// userAgent is a string identifying who the client is. It will be logged in
	// the telemetry in gitserver.
	userAgent string

	// HTTP client to use
	httpClient httpcli.Doer

	// logger is used for all logging and logger creation
	logger sglog.Logger

	// operations are used for internal observability
	operations *operations

	// clientSource is used to get the corresponding gprc client or address for a given repository
	clientSource ClientSource

	// subRepoPermsChecker is sub-repository permissions checker. This will
	// usually be authz.DefaultSubRepoPermsChecker, at least until that global is removed.
	subRepoPermsChecker authz.SubRepoPermissionChecker
}

type RawBatchLogResult struct {
	Stdout string
	Error  error
}
type BatchLogCallback func(repoCommit api.RepoCommit, gitLogResult RawBatchLogResult) error

type HunkReader interface {
	Read() (*Hunk, error)
	Close() error
}

type CommitLog struct {
	AuthorEmail  string
	AuthorName   string
	Timestamp    time.Time
	SHA          string
	ChangedFiles []string
}

type Client interface {
	// AddrForRepo returns the gitserver address to use for the given repo name.
	AddrForRepo(ctx context.Context, repoName api.RepoName) string

	// ArchiveReader streams back the file contents of an archived git repo.
	ArchiveReader(ctx context.Context, repo api.RepoName, options ArchiveOptions) (io.ReadCloser, error)

	// BatchLog invokes the given callback with the `git log` output for a batch of repository
	// and commit pairs. If the invoked callback returns a non-nil error, the operation will begin
	// to abort processing further results.
	BatchLog(ctx context.Context, opts BatchLogOptions, callback BatchLogCallback) error

	// BlameFile returns Git blame information about a file.
	BlameFile(ctx context.Context, repo api.RepoName, path string, opt *BlameOptions) ([]*Hunk, error)

	StreamBlameFile(ctx context.Context, repo api.RepoName, path string, opt *BlameOptions) (HunkReader, error)

	// CreateCommitFromPatch will attempt to create a commit from a patch
	// If possible, the error returned will be of type protocol.CreateCommitFromPatchError
	CreateCommitFromPatch(context.Context, protocol.CreateCommitFromPatchRequest) (*protocol.CreateCommitFromPatchResponse, error)

	// GetDefaultBranch returns the name of the default branch and the commit it's
	// currently at from the given repository. If short is true, then `main` instead
	// of `refs/heads/main` would be returned.
	//
	// If the repository is empty or currently being cloned, empty values and no
	// error are returned.
	GetDefaultBranch(ctx context.Context, repo api.RepoName, short bool) (refName string, commit api.CommitID, err error)

	// GetObject fetches git object data in the supplied repo
	GetObject(ctx context.Context, repo api.RepoName, objectName string) (*gitdomain.GitObject, error)

	// HasCommitAfter indicates the staleness of a repository. It returns a boolean indicating if a repository
	// contains a commit past a specified date.
	HasCommitAfter(ctx context.Context, repo api.RepoName, date string, revspec string) (bool, error)

	// IsRepoCloneable returns nil if the repository is cloneable.
	IsRepoCloneable(context.Context, api.RepoName) error

	// ListRefs returns a list of all refs in the repository.
	ListRefs(ctx context.Context, repo api.RepoName) ([]gitdomain.Ref, error)

	// ListBranches returns a list of all branches in the repository.
	ListBranches(ctx context.Context, repo api.RepoName, opt BranchesOptions) ([]*gitdomain.Branch, error)

	// MergeBase returns the merge base commit for the specified commits.
	MergeBase(ctx context.Context, repo api.RepoName, a, b api.CommitID) (api.CommitID, error)

	// Remove removes the repository clone from gitserver.
	Remove(context.Context, api.RepoName) error

	RepoCloneProgress(context.Context, ...api.RepoName) (*protocol.RepoCloneProgressResponse, error)

	// ResolveRevision will return the absolute commit for a commit-ish spec. If spec is empty, HEAD is
	// used.
	//
	// Error cases:
	// * Repo does not exist: gitdomain.RepoNotExistError
	// * Commit does not exist: gitdomain.RevisionNotFoundError
	// * Empty repository: gitdomain.RevisionNotFoundError
	// * Other unexpected errors.
	ResolveRevision(ctx context.Context, repo api.RepoName, spec string, opt ResolveRevisionOptions) (api.CommitID, error)

	// ResolveRevisions expands a set of RevisionSpecifiers (which may include hashes, globs, refs, or glob exclusions)
	// into an equivalent set of commit hashes
	ResolveRevisions(_ context.Context, repo api.RepoName, _ []protocol.RevisionSpecifier) ([]string, error)

	// RequestRepoUpdate is the new protocol endpoint for synchronous requests
	// with more detailed responses. Do not use this if you are not repo-updater.
	//
	// Repo updates are not guaranteed to occur. If a repo has been updated
	// recently (within the Since duration specified in the request), the
	// update won't happen.
	RequestRepoUpdate(context.Context, api.RepoName, time.Duration) (*protocol.RepoUpdateResponse, error)

	// RequestRepoClone is an asynchronous request to clone a repository.
	RequestRepoClone(context.Context, api.RepoName) (*protocol.RepoCloneResponse, error)

	// Search executes a search as specified by args, streaming the results as
	// it goes by calling onMatches with each set of results it receives in
	// response.
	Search(_ context.Context, _ *protocol.SearchRequest, onMatches func([]protocol.CommitMatch)) (limitHit bool, _ error)

	// Stat returns a FileInfo describing the named file at commit.
	Stat(ctx context.Context, repo api.RepoName, commit api.CommitID, path string) (fs.FileInfo, error)

	// DiffPath returns a position-ordered slice of changes (additions or deletions)
	// of the given path between the given source and target commits.
	DiffPath(ctx context.Context, repo api.RepoName, sourceCommit, targetCommit, path string) ([]*diff.Hunk, error)

	// ReadDir reads the contents of the named directory at commit.
	ReadDir(ctx context.Context, repo api.RepoName, commit api.CommitID, path string, recurse bool) ([]fs.FileInfo, error)

	// NewFileReader returns an io.ReadCloser reading from the named file at commit.
	// The caller should always close the reader after use.
	NewFileReader(ctx context.Context, repo api.RepoName, commit api.CommitID, name string) (io.ReadCloser, error)

	// DiffSymbols performs a diff command which is expected to be parsed by our symbols package
	DiffSymbols(ctx context.Context, repo api.RepoName, commitA, commitB api.CommitID) ([]byte, error)

	// Commits returns all commits matching the options.
	Commits(ctx context.Context, repo api.RepoName, opt CommitsOptions) ([]*gitdomain.Commit, error)

	// FirstEverCommit returns the first commit ever made to the repository.
	FirstEverCommit(ctx context.Context, repo api.RepoName) (*gitdomain.Commit, error)

	// ListTags returns a list of all tags in the repository. If commitObjs is non-empty, only all tags pointing at those commits are returned.
	ListTags(ctx context.Context, repo api.RepoName, commitObjs ...string) ([]*gitdomain.Tag, error)

	// ListDirectoryChildren fetches the list of children under the given directory
	// names. The result is a map keyed by the directory names with the list of files
	// under each.
	ListDirectoryChildren(ctx context.Context, repo api.RepoName, commit api.CommitID, dirnames []string) (map[string][]string, error)

	// Diff returns an iterator that can be used to access the diff between two
	// commits on a per-file basis. The iterator must be closed with Close when no
	// longer required.
	Diff(ctx context.Context, opts DiffOptions) (*DiffFileIterator, error)

	// ReadFile returns the first maxBytes of the named file at commit. If maxBytes <= 0, the entire
	// file is read. (If you just need to check a file's existence, use Stat, not ReadFile.)
	ReadFile(ctx context.Context, repo api.RepoName, commit api.CommitID, name string) ([]byte, error)

	// BranchesContaining returns a map from branch names to branch tip hashes for
	// each branch containing the given commit.
	BranchesContaining(ctx context.Context, repo api.RepoName, commit api.CommitID) ([]string, error)

	// RefDescriptions returns a map from commits to descriptions of the tip of each
	// branch and tag of the given repository.
	RefDescriptions(ctx context.Context, repo api.RepoName, gitObjs ...string) (map[string][]gitdomain.RefDescription, error)

	// CommitExists determines if the given commit exists in the given repository.
	CommitExists(ctx context.Context, repo api.RepoName, id api.CommitID) (bool, error)

	// CommitsExist determines if the given commits exists in the given repositories. This function returns
	// a slice of the same size as the input slice, true indicating that the commit at the symmetric index
	// exists.
	CommitsExist(ctx context.Context, repoCommits []api.RepoCommit) ([]bool, error)

	// Head determines the tip commit of the default branch for the given repository.
	// If no HEAD revision exists for the given repository (which occurs with empty
	// repositories), a false-valued flag is returned along with a nil error and
	// empty revision.
	Head(ctx context.Context, repo api.RepoName) (string, bool, error)

	// CommitDate returns the time that the given commit was committed. If the given
	// revision does not exist, a false-valued flag is returned along with a nil
	// error and zero-valued time.
	CommitDate(ctx context.Context, repo api.RepoName, commit api.CommitID) (string, time.Time, bool, error)

	// CommitGraph returns the commit graph for the given repository as a mapping
	// from a commit to its parents. If a commit is supplied, the returned graph will
	// be rooted at the given commit. If a non-zero limit is supplied, at most that
	// many commits will be returned.
	CommitGraph(ctx context.Context, repo api.RepoName, opts CommitGraphOptions) (_ *gitdomain.CommitGraph, err error)

	CommitLog(ctx context.Context, repo api.RepoName, after time.Time) ([]CommitLog, error)

	// CommitsUniqueToBranch returns a map from commits that exist on a particular
	// branch in the given repository to their committer date. This set of commits is
	// determined by listing `{branchName} ^HEAD`, which is interpreted as: all
	// commits on {branchName} not also on the tip of the default branch. If the
	// supplied branch name is the default branch, then this method instead returns
	// all commits reachable from HEAD.
	CommitsUniqueToBranch(ctx context.Context, repo api.RepoName, branchName string, isDefaultBranch bool, maxAge *time.Time) (map[string]time.Time, error)

	// LsFiles returns the output of `git ls-files`
	LsFiles(ctx context.Context, repo api.RepoName, commit api.CommitID, pathspecs ...gitdomain.Pathspec) ([]string, error)

	// GetCommits returns a git commit object describing each of the given repository and commit pairs. This
	// function returns a slice of the same size as the input slice. Values in the output slice may be nil if
	// their associated repository or commit are unresolvable.
	//
	// If ignoreErrors is true, then errors arising from any single failed git log operation will cause the
	// resulting commit to be nil, but not fail the entire operation.
	GetCommits(ctx context.Context, repoCommits []api.RepoCommit, ignoreErrors bool) ([]*gitdomain.Commit, error)

	// GetCommit returns the commit with the given commit ID, or ErrCommitNotFound if no such commit
	// exists.
	//
	// The remoteURLFunc is called to get the Git remote URL if it's not set in repo and if it is
	// needed. The Git remote URL is only required if the gitserver doesn't already contain a clone of
	// the repository or if the commit must be fetched from the remote.
	GetCommit(ctx context.Context, repo api.RepoName, id api.CommitID, opt ResolveRevisionOptions) (*gitdomain.Commit, error)

	// GetBehindAhead returns the behind/ahead commit counts information for right vs. left (both Git
	// revspecs).
	GetBehindAhead(ctx context.Context, repo api.RepoName, left, right string) (*gitdomain.BehindAhead, error)

	// ContributorCount returns the number of commits grouped by contributor
	ContributorCount(ctx context.Context, repo api.RepoName, opt ContributorOptions) ([]*gitdomain.ContributorCount, error)

	// LogReverseEach runs git log in reverse order and calls the given callback for each entry.
	LogReverseEach(ctx context.Context, repo string, commit string, n int, onLogEntry func(entry gitdomain.LogEntry) error) error

	// RevList makes a git rev-list call and iterates through the resulting commits, calling the provided
	// onCommit function for each.
	RevList(ctx context.Context, repo string, commit string, onCommit func(commit string) (bool, error)) error

	// Addrs returns a list of gitserver addresses associated with the Sourcegraph instance.
	Addrs() []string

	// SystemsInfo returns information about all gitserver instances associated with a Sourcegraph instance.
	SystemsInfo(ctx context.Context) ([]protocol.SystemInfo, error)

	// SystemInfo returns information about the gitserver instance at the given address.
	SystemInfo(ctx context.Context, addr string) (protocol.SystemInfo, error)

	// IsPerforcePathCloneable checks if the given Perforce depot path is cloneable by
	// checking if it is a valid depot and the given user has permission to access it.
	IsPerforcePathCloneable(ctx context.Context, conn protocol.PerforceConnectionDetails, depotPath string) error

	// CheckPerforceCredentials checks if the given Perforce credentials are valid
	CheckPerforceCredentials(ctx context.Context, conn protocol.PerforceConnectionDetails) error

	// PerforceUsers lists all the users known to the given Perforce server.
	PerforceUsers(ctx context.Context, conn protocol.PerforceConnectionDetails) ([]*perforce.User, error)

	// PerforceProtectsForUser returns all protects that apply to the given Perforce user.
	PerforceProtectsForUser(ctx context.Context, conn protocol.PerforceConnectionDetails, username string) ([]*perforce.Protect, error)

	// PerforceProtectsForDepot returns all protects that apply to the given Perforce depot.
	PerforceProtectsForDepot(ctx context.Context, conn protocol.PerforceConnectionDetails, depot string) ([]*perforce.Protect, error)

	// PerforceGroupMembers returns the members of the given Perforce group.
	PerforceGroupMembers(ctx context.Context, conn protocol.PerforceConnectionDetails, group string) ([]string, error)

	// IsPerforceSuperUser checks if the given Perforce user is a super user, and otherwise returns an error.
	IsPerforceSuperUser(ctx context.Context, conn protocol.PerforceConnectionDetails) error

	// PerforceGetChangelist gets the perforce changelist details for the given changelist ID.
	PerforceGetChangelist(ctx context.Context, conn protocol.PerforceConnectionDetails, changelist string) (*perforce.Changelist, error)
}

func (c *clientImplementor) SystemsInfo(ctx context.Context) ([]protocol.SystemInfo, error) {
	addresses := c.clientSource.Addresses()

	wg := pool.NewWithResults[protocol.SystemInfo]().WithErrors().WithContext(ctx)

	for _, addr := range addresses {
		addr := addr // capture addr
		wg.Go(func(ctx context.Context) (protocol.SystemInfo, error) {
			response, err := c.getDiskInfo(ctx, addr)
			if err != nil {
				return protocol.SystemInfo{}, err
			}
			return protocol.SystemInfo{
				Address:     addr.Address(),
				FreeSpace:   response.GetFreeSpace(),
				TotalSpace:  response.GetTotalSpace(),
				PercentUsed: response.GetPercentUsed(),
			}, nil
		})
	}

	return wg.Wait()
}

func (c *clientImplementor) SystemInfo(ctx context.Context, addr string) (protocol.SystemInfo, error) {
	ac := c.clientSource.GetAddressWithClient(addr)
	if ac == nil {
		return protocol.SystemInfo{}, errors.Newf("no client for address: %s", addr)
	}

	response, err := c.getDiskInfo(ctx, ac)
	if err != nil {
		return protocol.SystemInfo{}, err
	}

	return protocol.SystemInfo{
		Address:    ac.Address(),
		FreeSpace:  response.FreeSpace,
		TotalSpace: response.TotalSpace,
	}, nil
}

func (c *clientImplementor) getDiskInfo(ctx context.Context, addr AddressWithClient) (*proto.DiskInfoResponse, error) {
	if conf.IsGRPCEnabled(ctx) {
		client, err := addr.GRPCClient()
		if err != nil {
			return nil, err
		}
		resp, err := client.DiskInfo(ctx, &proto.DiskInfoRequest{})
		if err != nil {
			return nil, err
		}
		return resp, nil
	}

	uri := fmt.Sprintf("http://%s/disk-info", addr.Address())
	rs, err := c.do(ctx, "", uri, nil)
	if err != nil {
		return nil, err
	}
	defer rs.Body.Close()
	if rs.StatusCode != http.StatusOK {
		return nil, errors.Newf("http status %d: %s", rs.StatusCode, readResponseBody(io.LimitReader(rs.Body, 200)))
	}

	body, err := io.ReadAll(rs.Body)
	if err != nil {
		return nil, errors.Wrap(err, "reading read disk info response body")
	}

	var resp proto.DiskInfoResponse
	if err := protojson.Unmarshal(body, &resp); err != nil {
		return nil, errors.Wrap(err, "parsing disk info response body")
	}

	return &resp, nil
}

func (c *clientImplementor) Addrs() []string {
	address := c.clientSource.Addresses()

	addrs := make([]string, 0, len(address))
	for _, addr := range address {
		addrs = append(addrs, addr.Address())
	}
	return addrs
}

func (c *clientImplementor) AddrForRepo(ctx context.Context, repo api.RepoName) string {
	return c.clientSource.AddrForRepo(ctx, c.userAgent, repo)
}

func (c *clientImplementor) ClientForRepo(ctx context.Context, repo api.RepoName) (proto.GitserverServiceClient, error) {
	return c.clientSource.ClientForRepo(ctx, c.userAgent, repo)
}

// ArchiveOptions contains options for the Archive func.
type ArchiveOptions struct {
	Treeish   string               // the tree or commit to produce an archive for
	Format    ArchiveFormat        // format of the resulting archive (usually "tar" or "zip")
	Pathspecs []gitdomain.Pathspec // if nonempty, only include these pathspecs.
}

func (a *ArchiveOptions) Attrs() []attribute.KeyValue {
	specs := make([]string, len(a.Pathspecs))
	for i, pathspec := range a.Pathspecs {
		specs[i] = string(pathspec)
	}
	return []attribute.KeyValue{
		attribute.String("treeish", a.Treeish),
		attribute.String("format", string(a.Format)),
		attribute.StringSlice("pathspecs", specs),
	}
}

func (o *ArchiveOptions) FromProto(x *proto.ArchiveRequest) {
	protoPathSpecs := x.GetPathspecs()
	pathSpecs := make([]gitdomain.Pathspec, 0, len(protoPathSpecs))

	for _, path := range protoPathSpecs {
		pathSpecs = append(pathSpecs, gitdomain.Pathspec(path))
	}

	*o = ArchiveOptions{
		Treeish:   x.GetTreeish(),
		Format:    ArchiveFormat(x.GetFormat()),
		Pathspecs: pathSpecs,
	}
}

func (o *ArchiveOptions) ToProto(repo string) *proto.ArchiveRequest {
	protoPathSpecs := make([]string, 0, len(o.Pathspecs))

	for _, path := range o.Pathspecs {
		protoPathSpecs = append(protoPathSpecs, string(path))
	}

	return &proto.ArchiveRequest{
		Repo:      repo,
		Treeish:   o.Treeish,
		Format:    string(o.Format),
		Pathspecs: protoPathSpecs,
	}
}

type BatchLogOptions protocol.BatchLogRequest

func (opts BatchLogOptions) Attrs() []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.Int("numRepoCommits", len(opts.RepoCommits)),
		attribute.String("Format", opts.Format),
	}
}

// archiveReader wraps the StdoutReader yielded by gitserver's
// RemoteGitCommand.StdoutReader with one that knows how to report a repository-not-found
// error more carefully.
type archiveReader struct {
	base io.ReadCloser
	repo api.RepoName
	spec string
}

// Read checks the known output behavior of the StdoutReader.
func (a *archiveReader) Read(p []byte) (int, error) {
	n, err := a.base.Read(p)
	if err != nil {
		// handle the special case where git archive failed because of an invalid spec
		if isRevisionNotFound(err.Error()) {
			return 0, &gitdomain.RevisionNotFoundError{Repo: a.repo, Spec: a.spec}
		}
	}
	return n, err
}

func (a *archiveReader) Close() error {
	return a.base.Close()
}

// archiveURL returns a URL from which an archive of the given Git repository can
// be downloaded from.
func (c *clientImplementor) archiveURL(ctx context.Context, repo api.RepoName, opt ArchiveOptions) *url.URL {
	q := url.Values{
		"repo":    {string(repo)},
		"treeish": {opt.Treeish},
		"format":  {string(opt.Format)},
	}

	for _, pathspec := range opt.Pathspecs {
		q.Add("path", string(pathspec))
	}

	addrForRepo := c.AddrForRepo(ctx, repo)
	return &url.URL{
		Scheme:   "http",
		Host:     addrForRepo,
		Path:     "/archive",
		RawQuery: q.Encode(),
	}
}

type badRequestError struct{ error }

func (e badRequestError) BadRequest() bool { return true }

func (c *RemoteGitCommand) sendExec(ctx context.Context) (_ io.ReadCloser, err error) {
	ctx, cancel := context.WithCancel(ctx)
	ctx, _, endObservation := c.execOp.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		c.repo.Attr(),
		attribute.StringSlice("args", c.args[1:]),
	}})
	done := func() {
		cancel()
		endObservation(1, observation.Args{})
	}
	defer func() {
		if err != nil {
			done()
		}
	}()

	repoName := protocol.NormalizeRepo(c.repo)

	// Check that ctx is not expired.
	if err := ctx.Err(); err != nil {
		deadlineExceededCounter.Inc()
		return nil, err
	}

	if conf.IsGRPCEnabled(ctx) {
		client, err := c.execer.ClientForRepo(ctx, repoName)
		if err != nil {
			return nil, err
		}

		req := &proto.ExecRequest{
			Repo:      string(repoName),
			Args:      stringsToByteSlices(c.args[1:]),
			Stdin:     c.stdin,
			NoTimeout: c.noTimeout,

			// 🚨Warning🚨: There is no guarantee that EnsureRevision is a valid utf-8 string.
			EnsureRevision: []byte(c.EnsureRevision()),
		}

		stream, err := client.Exec(ctx, req)
		if err != nil {
			return nil, err
		}
		r := streamio.NewReader(func() ([]byte, error) {
			msg, err := stream.Recv()
			if status.Code(err) == codes.Canceled {
				return nil, context.Canceled
			} else if err != nil {
				return nil, err
			}
			return msg.GetData(), nil
		})

		return &readCloseWrapper{r: r, closeFn: done}, nil

	} else {
		req := &protocol.ExecRequest{
			Repo:           repoName,
			EnsureRevision: c.EnsureRevision(),
			Args:           c.args[1:],
			Stdin:          c.stdin,
			NoTimeout:      c.noTimeout,
		}
		resp, err := c.execer.httpPost(ctx, repoName, "exec", req)
		if err != nil {
			return nil, err
		}

		switch resp.StatusCode {
		case http.StatusOK:
			return &cmdReader{rc: &readCloseWrapper{r: resp.Body, closeFn: done}, trailer: resp.Trailer}, nil

		case http.StatusNotFound:
			var payload protocol.NotFoundPayload
			if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
				resp.Body.Close()
				return nil, err
			}
			resp.Body.Close()
			return nil, &gitdomain.RepoNotExistError{Repo: repoName, CloneInProgress: payload.CloneInProgress, CloneProgress: payload.CloneProgress}

		default:
			resp.Body.Close()
			return nil, errors.Errorf("unexpected status code: %d", resp.StatusCode)
		}
	}
}

type readCloseWrapper struct {
	r       io.Reader
	closeFn func()
}

func (r *readCloseWrapper) Read(p []byte) (int, error) {
	n, err := r.r.Read(p)
	if err != nil {
		err = convertGRPCErrorToGitDomainError(err)
	}

	return n, err
}

func (r *readCloseWrapper) Close() error {
	r.closeFn()
	return nil
}

// convertGRPCErrorToGitDomainError translates a GRPC error to a gitdomain error.
// If the error is not a GRPC error, it is returned as-is.
func convertGRPCErrorToGitDomainError(err error) error {
	st, ok := status.FromError(err)
	if !ok {
		return err
	}

	if st.Code() == codes.Canceled {
		return context.Canceled
	}

	if st.Code() == codes.DeadlineExceeded {
		return context.DeadlineExceeded
	}

	for _, detail := range st.Details() {
		switch payload := detail.(type) {

		case *proto.ExecStatusPayload:
			return &CommandStatusError{
				Message:    st.Message(),
				Stderr:     payload.Stderr,
				StatusCode: payload.StatusCode,
			}

		case *proto.NotFoundPayload:
			return &gitdomain.RepoNotExistError{
				Repo:            api.RepoName(payload.Repo),
				CloneInProgress: payload.CloneInProgress,
				CloneProgress:   payload.CloneProgress,
			}
		}
	}

	return err
}

type CommandStatusError struct {
	Message    string
	StatusCode int32
	Stderr     string
}

func (c *CommandStatusError) Error() string {
	stderr := c.Stderr
	if len(stderr) > 100 {
		stderr = stderr[:100] + "... (truncated)"
	}
	if c.Message != "" {
		return fmt.Sprintf("%s (stderr: %q)", c.Message, stderr)
	}
	if c.StatusCode != 0 {
		return fmt.Sprintf("non-zero exit status: %d (stderr: %q)", c.StatusCode, stderr)
	}
	return stderr
}

func isRevisionNotFound(err string) bool {
	// error message is lowercased in to handle case insensitive error messages
	loweredErr := strings.ToLower(err)
	return strings.Contains(loweredErr, "not a valid object")
}

func (c *clientImplementor) Search(ctx context.Context, args *protocol.SearchRequest, onMatches func([]protocol.CommitMatch)) (limitHit bool, err error) {
	ctx, _, endObservation := c.operations.search.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		args.Repo.Attr(),
		attribute.Stringer("query", args.Query),
		attribute.Bool("diff", args.IncludeDiff),
		attribute.Int("limit", args.Limit),
	}})
	defer endObservation(1, observation.Args{})

	repoName := protocol.NormalizeRepo(args.Repo)

	if conf.IsGRPCEnabled(ctx) {
		client, err := c.ClientForRepo(ctx, repoName)
		if err != nil {
			return false, err
		}

		cs, err := client.Search(ctx, args.ToProto())
		if err != nil {
			return false, convertGitserverError(err)
		}

		limitHit := false
		for {
			msg, err := cs.Recv()
			if err != nil {
				return limitHit, convertGitserverError(err)
			}

			switch m := msg.Message.(type) {
			case *proto.SearchResponse_LimitHit:
				limitHit = limitHit || m.LimitHit
			case *proto.SearchResponse_Match:
				onMatches([]protocol.CommitMatch{protocol.CommitMatchFromProto(m.Match)})
			default:
				return false, errors.Newf("unknown message type %T", m)
			}
		}
	}

	addrForRepo := c.AddrForRepo(ctx, repoName)

	protocol.RegisterGob()
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	if err := enc.Encode(args); err != nil {
		return false, err
	}

	uri := "http://" + addrForRepo + "/search"
	resp, err := c.do(ctx, repoName, uri, buf.Bytes())
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	var (
		decodeErr error
		eventDone protocol.SearchEventDone
	)
	dec := StreamSearchDecoder{
		OnMatches: func(e protocol.SearchEventMatches) {
			onMatches(e)
		},
		OnDone: func(e protocol.SearchEventDone) {
			eventDone = e
		},
		OnUnknown: func(event, _ []byte) {
			decodeErr = errors.Errorf("unknown event %s", event)
		},
	}

	if err := dec.ReadAll(resp.Body); err != nil {
		return false, err
	}

	if decodeErr != nil {
		return false, decodeErr
	}

	return eventDone.LimitHit, eventDone.Err()
}

func convertGitserverError(err error) error {
	if errors.Is(err, io.EOF) {
		return nil
	}

	st, ok := status.FromError(err)
	if !ok {
		return err
	}

	if st.Code() == codes.Canceled {
		return context.Canceled
	}

	if st.Code() == codes.DeadlineExceeded {
		return context.DeadlineExceeded
	}

	for _, detail := range st.Details() {
		if notFound, ok := detail.(*proto.NotFoundPayload); ok {
			return &gitdomain.RepoNotExistError{
				Repo:            api.RepoName(notFound.GetRepo()),
				CloneProgress:   notFound.GetCloneProgress(),
				CloneInProgress: notFound.GetCloneInProgress(),
			}
		}
	}

	return err
}

var deadlineExceededCounter = promauto.NewCounter(prometheus.CounterOpts{
	Name: "src_gitserver_client_deadline_exceeded",
	Help: "Times that Client.sendExec() returned context.DeadlineExceeded",
})

// BatchLog invokes the given callback with the `git log` output for a batch of repository
// and commit pairs. If the invoked callback returns a non-nil error, the operation will begin
// to abort processing further results.
func (c *clientImplementor) BatchLog(ctx context.Context, opts BatchLogOptions, callback BatchLogCallback) (err error) {
	ctx, _, endObservation := c.operations.batchLog.With(ctx, &err, observation.Args{Attrs: opts.Attrs()})
	defer endObservation(1, observation.Args{})

	type clientAndError struct {
		client  proto.GitserverServiceClient
		dialErr error // non-nil if there was an error dialing the client
	}

	// Make a request to a single gitserver shard and feed the results to the user-supplied
	// callback. This function is invoked multiple times (and concurrently) in the loops below
	// this function definition.
	performLogRequestToShard := func(ctx context.Context, addr string, grpcClient clientAndError, repoCommits []api.RepoCommit) (err error) {
		var numProcessed int
		repoNames := repoNamesFromRepoCommits(repoCommits)

		ctx, logger, endObservation := c.operations.batchLogSingle.With(ctx, &err, observation.Args{
			Attrs: []attribute.KeyValue{
				attribute.String("addr", addr),
				attribute.Int("numRepos", len(repoNames)),
				attribute.Int("numRepoCommits", len(repoCommits)),
			},
		})
		defer func() {
			endObservation(1, observation.Args{
				Attrs: []attribute.KeyValue{
					attribute.Int("numProcessed", numProcessed),
				},
			})
		}()

		request := protocol.BatchLogRequest{
			RepoCommits: repoCommits,
			Format:      opts.Format,
		}

		var response protocol.BatchLogResponse

		if conf.IsGRPCEnabled(ctx) {
			client, err := grpcClient.client, grpcClient.dialErr
			if err != nil {
				return err
			}

			resp, err := client.BatchLog(ctx, request.ToProto())
			if err != nil {
				return err
			}

			response.FromProto(resp)
			logger.AddEvent("read response", attribute.Int("numResults", len(response.Results)))
		} else {
			var buf bytes.Buffer
			if err := json.NewEncoder(&buf).Encode(request); err != nil {
				return err
			}

			uri := "http://" + addr + "/batch-log"
			resp, err := c.do(ctx, api.RepoName(strings.Join(repoNames, ",")), uri, buf.Bytes())
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			logger.AddEvent("POST", attribute.Int("resp.StatusCode", resp.StatusCode))

			if resp.StatusCode != http.StatusOK {
				return errors.Newf("http status %d: %s", resp.StatusCode, readResponseBody(io.LimitReader(resp.Body, 200)))
			}

			if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
				return err
			}
			logger.AddEvent("read response", attribute.Int("numResults", len(response.Results)))
		}

		for _, result := range response.Results {
			var err error
			if result.CommandError != "" {
				err = errors.New(result.CommandError)
			}

			rawResult := RawBatchLogResult{
				Stdout: result.CommandOutput,
				Error:  err,
			}
			if err := callback(result.RepoCommit, rawResult); err != nil {
				return errors.Wrap(err, "commitLogCallback")
			}

			numProcessed++
		}

		return nil
	}

	// Construct batches of requests keyed by the address of the server that will receive the batch.
	// The results from gitserver will have to be re-interlaced before returning to the client, so we
	// don't need to be particularly concerned about order here.

	batches := make(map[string][]api.RepoCommit, len(opts.RepoCommits))
	addrsByName := make(map[api.RepoName]string, len(opts.RepoCommits))

	for _, repoCommit := range opts.RepoCommits {
		addr, ok := addrsByName[repoCommit.Repo]
		if !ok {
			addr = c.AddrForRepo(ctx, repoCommit.Repo)
			addrsByName[repoCommit.Repo] = addr
		}

		batches[addr] = append(batches[addr], api.RepoCommit{
			Repo:     repoCommit.Repo,
			CommitID: repoCommit.CommitID,
		})
	}

	// Perform each batch request concurrently up to a maximum limit of 32 requests
	// in-flight at one time.
	//
	// This limit will be useless in practice most of the  time as we should only be
	// making one request per shard and instances should _generally_ have fewer than
	// 32 gitserver shards. This condition is really to catch unexpected bad behavior.
	// At the time this limit was chosen, we have 20 gitserver shards on our Cloud
	// environment, which holds a large proportion of GitHub repositories.
	//
	// This operation returns partial results in the case of a malformed or missing
	// repository or a bad commit reference, but does not attempt to return partial
	// results when an entire shard is down. Any of these operations failing will
	// cause an error to be returned from the entire BatchLog function.

	sem := semaphore.NewWeighted(int64(32))
	g, ctx := errgroup.WithContext(ctx)

	for addr, repoCommits := range batches {
		// avoid capturing loop variable below
		addr, repoCommits := addr, repoCommits

		if err := sem.Acquire(ctx, 1); err != nil {
			return err
		}

		client, err := c.ClientForRepo(ctx, repoCommits[0].Repo)
		if err != nil {
			err = errors.Wrapf(err, "getting gRPC client for repository %q", repoCommits[0].Repo)
		}

		ce := clientAndError{client: client, dialErr: err}

		g.Go(func() (err error) {
			defer sem.Release(1)

			return performLogRequestToShard(ctx, addr, ce, repoCommits)
		})
	}

	return g.Wait()
}

func repoNamesFromRepoCommits(repoCommits []api.RepoCommit) []string {
	repoNames := make([]string, 0, len(repoCommits))
	repoNameSet := make(map[api.RepoName]struct{}, len(repoCommits))

	for _, rc := range repoCommits {
		if _, ok := repoNameSet[rc.Repo]; ok {
			continue
		}

		repoNameSet[rc.Repo] = struct{}{}
		repoNames = append(repoNames, string(rc.Repo))
	}

	return repoNames
}

func (c *clientImplementor) gitCommand(repo api.RepoName, arg ...string) GitCommand {
	if ClientMocks.LocalGitserver {
		cmd := NewLocalGitCommand(repo, arg...)
		if ClientMocks.LocalGitCommandReposDir != "" {
			cmd.ReposDir = ClientMocks.LocalGitCommandReposDir
		}
		return cmd
	}
	return &RemoteGitCommand{
		repo:   repo,
		execer: c,
		args:   append([]string{git}, arg...),
		execOp: c.operations.exec,
	}
}

func (c *clientImplementor) RequestRepoUpdate(ctx context.Context, repo api.RepoName, since time.Duration) (*protocol.RepoUpdateResponse, error) {
	req := &protocol.RepoUpdateRequest{
		Repo:  repo,
		Since: since,
	}

	if conf.IsGRPCEnabled(ctx) {
		client, err := c.ClientForRepo(ctx, repo)
		if err != nil {
			return nil, err
		}

		resp, err := client.RepoUpdate(ctx, req.ToProto())
		if err != nil {
			return nil, err
		}

		var info protocol.RepoUpdateResponse
		info.FromProto(resp)

		return &info, nil

	} else {
		resp, err := c.httpPost(ctx, repo, "repo-update", req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, &url.Error{
				URL: resp.Request.URL.String(),
				Op:  "RepoUpdate",
				Err: errors.Errorf("RepoUpdate: http status %d: %s", resp.StatusCode, readResponseBody(io.LimitReader(resp.Body, 200))),
			}
		}

		var info protocol.RepoUpdateResponse
		err = json.NewDecoder(resp.Body).Decode(&info)
		return &info, err
	}
}

// RequestRepoClone requests that the gitserver does an asynchronous clone of the repository.
func (c *clientImplementor) RequestRepoClone(ctx context.Context, repo api.RepoName) (*protocol.RepoCloneResponse, error) {
	if conf.IsGRPCEnabled(ctx) {
		client, err := c.ClientForRepo(ctx, repo)
		if err != nil {
			return nil, err
		}

		req := proto.RepoCloneRequest{
			Repo: string(repo),
		}

		resp, err := client.RepoClone(ctx, &req)
		if err != nil {
			return nil, err
		}

		var info protocol.RepoCloneResponse
		info.FromProto(resp)
		return &info, nil

	} else {

		req := &protocol.RepoCloneRequest{
			Repo: repo,
		}
		resp, err := c.httpPost(ctx, repo, "repo-clone", req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return nil, &url.Error{
				URL: resp.Request.URL.String(),
				Op:  "RepoInfo",
				Err: errors.Errorf("RepoInfo: http status %d: %s", resp.StatusCode, readResponseBody(io.LimitReader(resp.Body, 200))),
			}
		}

		var info *protocol.RepoCloneResponse
		err = json.NewDecoder(resp.Body).Decode(&info)
		return info, err
	}
}

// MockIsRepoCloneable mocks (*Client).IsRepoCloneable for tests.
var MockIsRepoCloneable func(api.RepoName) error

func (c *clientImplementor) IsRepoCloneable(ctx context.Context, repo api.RepoName) error {
	if MockIsRepoCloneable != nil {
		return MockIsRepoCloneable(repo)
	}

	var resp protocol.IsRepoCloneableResponse

	if conf.IsGRPCEnabled(ctx) {
		client, err := c.ClientForRepo(ctx, repo)
		if err != nil {
			return err
		}

		req := &proto.IsRepoCloneableRequest{
			Repo: string(repo),
		}

		r, err := client.IsRepoCloneable(ctx, req)
		if err != nil {
			return err
		}

		resp.FromProto(r)
	} else {
		req := &protocol.IsRepoCloneableRequest{
			Repo: repo,
		}
		r, err := c.httpPost(ctx, repo, "is-repo-cloneable", req)
		if err != nil {
			return err
		}
		defer r.Body.Close()
		if r.StatusCode != http.StatusOK {
			return errors.Errorf("gitserver error (status code %d): %s", r.StatusCode, readResponseBody(r.Body))
		}

		if err := json.NewDecoder(r.Body).Decode(&resp); err != nil {
			return err
		}
	}

	if resp.Cloneable {
		return nil
	}

	// Treat all 4xx errors as not found, since we have more relaxed
	// requirements on what a valid URL is we should treat bad requests,
	// etc as not found.
	notFound := strings.Contains(resp.Reason, "not found") || strings.Contains(resp.Reason, "The requested URL returned error: 4")
	return &RepoNotCloneableErr{
		repo:     repo,
		reason:   resp.Reason,
		notFound: notFound,
		cloned:   resp.Cloned,
	}
}

// RepoNotCloneableErr is the error that happens when a repository can not be cloned.
type RepoNotCloneableErr struct {
	repo     api.RepoName
	reason   string
	notFound bool
	cloned   bool // Has the repo ever been cloned in the past
}

// NotFound returns true if the repo could not be cloned because it wasn't found.
// This may be because the repo doesn't exist, or because the repo is private and
// there are insufficient permissions.
func (e *RepoNotCloneableErr) NotFound() bool {
	return e.notFound
}

func (e *RepoNotCloneableErr) Error() string {
	msg := "unable to clone repo"
	if e.cloned {
		msg = "unable to update repo"
	}
	return fmt.Sprintf("%s (name=%q notfound=%v) because %s", msg, e.repo, e.notFound, e.reason)
}

func (c *clientImplementor) RepoCloneProgress(ctx context.Context, repos ...api.RepoName) (*protocol.RepoCloneProgressResponse, error) {
	numPossibleShards := len(c.clientSource.Addresses())

	if conf.IsGRPCEnabled(ctx) {
		shards := make(map[proto.GitserverServiceClient]*proto.RepoCloneProgressRequest, (len(repos)/numPossibleShards)*2) // 2x because it may not be a perfect division
		for _, r := range repos {
			client, err := c.ClientForRepo(ctx, r)
			if err != nil {
				return nil, err
			}

			shard := shards[client]
			if shard == nil {
				shard = new(proto.RepoCloneProgressRequest)
				shards[client] = shard
			}

			shard.Repos = append(shard.Repos, string(r))
		}

		p := pool.NewWithResults[*proto.RepoCloneProgressResponse]().WithContext(ctx)

		for client, req := range shards {
			client := client
			req := req
			p.Go(func(ctx context.Context) (*proto.RepoCloneProgressResponse, error) {
				return client.RepoCloneProgress(ctx, req)

			})
		}

		res, err := p.Wait()
		if err != nil {
			return nil, err
		}

		result := &protocol.RepoCloneProgressResponse{
			Results: make(map[api.RepoName]*protocol.RepoCloneProgress),
		}
		for _, r := range res {

			for repo, info := range r.Results {
				var rp protocol.RepoCloneProgress
				rp.FromProto(info)
				result.Results[api.RepoName(repo)] = &rp
			}

		}

		return result, nil

	} else {

		shards := make(map[string]*protocol.RepoCloneProgressRequest, (len(repos)/numPossibleShards)*2) // 2x because it may not be a perfect division

		for _, r := range repos {
			addr := c.AddrForRepo(ctx, r)
			shard := shards[addr]

			if shard == nil {
				shard = new(protocol.RepoCloneProgressRequest)
				shards[addr] = shard
			}

			shard.Repos = append(shard.Repos, r)
		}

		type op struct {
			req *protocol.RepoCloneProgressRequest
			res *protocol.RepoCloneProgressResponse
			err error
		}

		ch := make(chan op, len(shards))
		for _, req := range shards {
			go func(o op) {
				var resp *http.Response
				resp, o.err = c.httpPost(ctx, o.req.Repos[0], "repo-clone-progress", o.req)
				if o.err != nil {
					ch <- o
					return
				}

				defer resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					o.err = &url.Error{
						URL: resp.Request.URL.String(),
						Op:  "RepoCloneProgress",
						Err: errors.Errorf("RepoCloneProgress: http status %d", resp.StatusCode),
					}
					ch <- o
					return // we never get an error status code AND result
				}

				o.res = new(protocol.RepoCloneProgressResponse)
				o.err = json.NewDecoder(resp.Body).Decode(o.res)
				ch <- o
			}(op{req: req})
		}

		var err error
		res := protocol.RepoCloneProgressResponse{
			Results: make(map[api.RepoName]*protocol.RepoCloneProgress),
		}

		for i := 0; i < cap(ch); i++ {
			o := <-ch

			if o.err != nil {
				err = errors.Append(err, o.err)
				continue
			}

			for repo, info := range o.res.Results {
				res.Results[repo] = info
			}
		}
		return &res, err
	}
}

func (c *clientImplementor) Remove(ctx context.Context, repo api.RepoName) error {
	// In case the repo has already been deleted from the database we need to pass
	// the old name in order to land on the correct gitserver instance
	undeletedName := api.UndeletedRepoName(repo)

	if conf.IsGRPCEnabled(ctx) {
		client, err := c.ClientForRepo(ctx, undeletedName)
		if err != nil {
			return err
		}
		_, err = client.RepoDelete(ctx, &proto.RepoDeleteRequest{
			Repo: string(repo),
		})
		return err
	}

	addr := c.AddrForRepo(ctx, undeletedName)
	return c.removeFrom(ctx, undeletedName, addr)
}

func (c *clientImplementor) removeFrom(ctx context.Context, repo api.RepoName, from string) error {
	b, err := json.Marshal(&protocol.RepoDeleteRequest{
		Repo: repo,
	})
	if err != nil {
		return err
	}

	uri := "http://" + from + "/delete"
	resp, err := c.do(ctx, repo, uri, b)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &url.Error{
			URL: resp.Request.URL.String(),
			Op:  "RepoRemove",
			Err: errors.Errorf("RepoRemove: http status %d: %s", resp.StatusCode, readResponseBody(io.LimitReader(resp.Body, 200))),
		}
	}
	return nil
}

func (c *clientImplementor) IsPerforcePathCloneable(ctx context.Context, conn protocol.PerforceConnectionDetails, depotPath string) error {
	if conf.IsGRPCEnabled(ctx) {
		// depotPath is not actually a repo name, but it will spread the load of isPerforcePathCloneable
		// a bit over the different gitserver instances. It's really just used as a consistent hashing
		// key here.
		client, err := c.ClientForRepo(ctx, api.RepoName(depotPath))
		if err != nil {
			return err
		}
		_, err = client.IsPerforcePathCloneable(ctx, &proto.IsPerforcePathCloneableRequest{
			ConnectionDetails: conn.ToProto(),
			DepotPath:         depotPath,
		})
		if err != nil {
			// Unwrap proto errors for nicer error messages.
			if s, ok := status.FromError(err); ok {
				return errors.New(s.Message())
			}
			return err
		}

		return nil
	}

	addr := c.AddrForRepo(ctx, api.RepoName(depotPath))
	b, err := json.Marshal(&protocol.IsPerforcePathCloneableRequest{
		P4Port:    conn.P4Port,
		P4User:    conn.P4User,
		P4Passwd:  conn.P4Passwd,
		DepotPath: depotPath,
	})
	if err != nil {
		return err
	}

	uri := "http://" + addr + "/is-perforce-path-cloneable"
	resp, err := c.do(ctx, "is-perforce-path-cloneable", uri, b)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &url.Error{
			URL: resp.Request.URL.String(),
			Op:  "IsPerforcePathCloneable",
			Err: errors.Errorf("IsPerforcePathCloneable: http status %d: %s", resp.StatusCode, readResponseBody(io.LimitReader(resp.Body, 200))),
		}
	}

	return nil
}

func (c *clientImplementor) CheckPerforceCredentials(ctx context.Context, conn protocol.PerforceConnectionDetails) error {
	if conf.IsGRPCEnabled(ctx) {
		// p4port is not actually a repo name, but it will spread the load of CheckPerforceCredentials
		// a bit over the different gitserver instances. It's really just used as a consistent hashing
		// key here.
		client, err := c.ClientForRepo(ctx, api.RepoName(conn.P4Port))
		if err != nil {
			return err
		}
		_, err = client.CheckPerforceCredentials(ctx, &proto.CheckPerforceCredentialsRequest{
			ConnectionDetails: conn.ToProto(),
		})
		if err != nil {
			// Unwrap proto errors for nicer error messages.
			if s, ok := status.FromError(err); ok {
				return errors.New(s.Message())
			}
			return err
		}

		return nil
	}

	addr := c.AddrForRepo(ctx, api.RepoName(conn.P4Port))
	b, err := json.Marshal(&protocol.CheckPerforceCredentialsRequest{
		P4Port:   conn.P4Port,
		P4User:   conn.P4User,
		P4Passwd: conn.P4Passwd,
	})
	if err != nil {
		return err
	}

	uri := "http://" + addr + "/check-perforce-credentials"
	resp, err := c.do(ctx, "check-perforce-credentials", uri, b)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &url.Error{
			URL: resp.Request.URL.String(),
			Op:  "CheckPerforceCredentials",
			Err: errors.Errorf("CheckPerforceCredentials: http status %d: %s", resp.StatusCode, readResponseBody(io.LimitReader(resp.Body, 200))),
		}
	}

	return nil
}

func (c *clientImplementor) PerforceUsers(ctx context.Context, conn protocol.PerforceConnectionDetails) ([]*perforce.User, error) {
	if conf.IsGRPCEnabled(ctx) {
		// p4port is not actually a repo name, but it will spread the load of CheckPerforceCredentials
		// a bit over the different gitserver instances. It's really just used as a consistent hashing
		// key here.
		client, err := c.ClientForRepo(ctx, api.RepoName(conn.P4Port))
		if err != nil {
			return nil, err
		}
		resp, err := client.PerforceUsers(ctx, &proto.PerforceUsersRequest{
			ConnectionDetails: conn.ToProto(),
		})
		if err != nil {
			return nil, err
		}

		users := make([]*perforce.User, len(resp.GetUsers()))
		for i, u := range resp.GetUsers() {
			users[i] = perforce.UserFromProto(u)
		}
		return users, nil
	}

	addr := c.AddrForRepo(ctx, api.RepoName(conn.P4Port))
	b, err := json.Marshal(&protocol.PerforceUsersRequest{
		P4Port:   conn.P4Port,
		P4User:   conn.P4User,
		P4Passwd: conn.P4Passwd,
	})
	if err != nil {
		return nil, err
	}

	uri := "http://" + addr + "/perforce-users"
	resp, err := c.do(ctx, "perforce-users", uri, b)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &url.Error{
			URL: resp.Request.URL.String(),
			Op:  "PerforceUsers",
			Err: errors.Errorf("PerforceUsers: http status %d: %s", resp.StatusCode, readResponseBody(io.LimitReader(resp.Body, 200))),
		}
	}

	var payload protocol.PerforceUsersResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	users := make([]*perforce.User, len(payload.Users))
	for i, u := range payload.Users {
		users[i] = &perforce.User{
			Username: u.Username,
			Email:    u.Email,
		}
	}

	return users, nil
}

func (c *clientImplementor) PerforceProtectsForUser(ctx context.Context, conn protocol.PerforceConnectionDetails, username string) ([]*perforce.Protect, error) {
	if conf.IsGRPCEnabled(ctx) {
		// p4port is not actually a repo name, but it will spread the load of CheckPerforceCredentials
		// a bit over the different gitserver instances. It's really just used as a consistent hashing
		// key here.
		client, err := c.ClientForRepo(ctx, api.RepoName(conn.P4Port))
		if err != nil {
			return nil, err
		}
		resp, err := client.PerforceProtectsForUser(ctx, &proto.PerforceProtectsForUserRequest{
			ConnectionDetails: conn.ToProto(),
			Username:          username,
		})
		if err != nil {
			return nil, err
		}

		protects := make([]*perforce.Protect, len(resp.GetProtects()))
		for i, p := range resp.GetProtects() {
			protects[i] = perforce.ProtectFromProto(p)
		}
		return protects, nil
	}

	addr := c.AddrForRepo(ctx, api.RepoName(conn.P4Port))
	b, err := json.Marshal(&protocol.PerforceProtectsForUserRequest{
		P4Port:   conn.P4Port,
		P4User:   conn.P4User,
		P4Passwd: conn.P4Passwd,
		Username: username,
	})
	if err != nil {
		return nil, err
	}

	uri := "http://" + addr + "/perforce-protects-for-user"
	resp, err := c.do(ctx, "perforce-protects-for-user", uri, b)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &url.Error{
			URL: resp.Request.URL.String(),
			Op:  "PerforceProtectsForUser",
			Err: errors.Errorf("PerforceProtectsForUser: http status %d: %s", resp.StatusCode, readResponseBody(io.LimitReader(resp.Body, 200))),
		}
	}

	var payload protocol.PerforceProtectsForUserResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	protects := make([]*perforce.Protect, len(payload.Protects))
	for i, p := range payload.Protects {
		protects[i] = &perforce.Protect{
			Level:       p.Level,
			EntityType:  p.EntityType,
			EntityName:  p.EntityName,
			Match:       p.Match,
			IsExclusion: p.IsExclusion,
			Host:        p.Host,
		}
	}

	return protects, nil
}

func (c *clientImplementor) PerforceProtectsForDepot(ctx context.Context, conn protocol.PerforceConnectionDetails, depot string) ([]*perforce.Protect, error) {
	if conf.IsGRPCEnabled(ctx) {
		// p4port is not actually a repo name, but it will spread the load of CheckPerforceCredentials
		// a bit over the different gitserver instances. It's really just used as a consistent hashing
		// key here.
		client, err := c.ClientForRepo(ctx, api.RepoName(conn.P4Port))
		if err != nil {
			return nil, err
		}
		resp, err := client.PerforceProtectsForDepot(ctx, &proto.PerforceProtectsForDepotRequest{
			ConnectionDetails: conn.ToProto(),
			Depot:             depot,
		})
		if err != nil {
			return nil, err
		}

		protects := make([]*perforce.Protect, len(resp.GetProtects()))
		for i, p := range resp.GetProtects() {
			protects[i] = perforce.ProtectFromProto(p)
		}
		return protects, nil
	}

	addr := c.AddrForRepo(ctx, api.RepoName(conn.P4Port))
	b, err := json.Marshal(&protocol.PerforceProtectsForDepotRequest{
		P4Port:   conn.P4Port,
		P4User:   conn.P4User,
		P4Passwd: conn.P4Passwd,
		Depot:    depot,
	})
	if err != nil {
		return nil, err
	}

	uri := "http://" + addr + "/perforce-protects-for-depot"
	resp, err := c.do(ctx, "perforce-protects-for-depot", uri, b)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &url.Error{
			URL: resp.Request.URL.String(),
			Op:  "PerforceProtectsForDepot",
			Err: errors.Errorf("PerforceProtectsForDepot: http status %d: %s", resp.StatusCode, readResponseBody(io.LimitReader(resp.Body, 200))),
		}
	}

	var payload protocol.PerforceProtectsForDepotResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	protects := make([]*perforce.Protect, len(payload.Protects))
	for i, p := range payload.Protects {
		protects[i] = &perforce.Protect{
			Level:       p.Level,
			EntityType:  p.EntityType,
			EntityName:  p.EntityName,
			Match:       p.Match,
			IsExclusion: p.IsExclusion,
			Host:        p.Host,
		}
	}

	return protects, nil
}

func (c *clientImplementor) PerforceGroupMembers(ctx context.Context, conn protocol.PerforceConnectionDetails, group string) ([]string, error) {
	if conf.IsGRPCEnabled(ctx) {
		// p4port is not actually a repo name, but it will spread the load of CheckPerforceCredentials
		// a bit over the different gitserver instances. It's really just used as a consistent hashing
		// key here.
		client, err := c.ClientForRepo(ctx, api.RepoName(conn.P4Port))
		if err != nil {
			return nil, err
		}
		resp, err := client.PerforceGroupMembers(ctx, &proto.PerforceGroupMembersRequest{
			ConnectionDetails: conn.ToProto(),
			Group:             group,
		})
		if err != nil {
			return nil, err
		}

		return resp.GetUsernames(), nil
	}

	addr := c.AddrForRepo(ctx, api.RepoName(conn.P4Port))
	b, err := json.Marshal(&protocol.PerforceGroupMembersRequest{
		P4Port:   conn.P4Port,
		P4User:   conn.P4User,
		P4Passwd: conn.P4Passwd,
		Group:    group,
	})
	if err != nil {
		return nil, err
	}

	uri := "http://" + addr + "/perforce-group-members"
	resp, err := c.do(ctx, "perforce-group-members", uri, b)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &url.Error{
			URL: resp.Request.URL.String(),
			Op:  "PerforceGroupMembers",
			Err: errors.Errorf("PerforceGroupMembers: http status %d: %s", resp.StatusCode, readResponseBody(io.LimitReader(resp.Body, 200))),
		}
	}

	var payload protocol.PerforceGroupMembersResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	return payload.Usernames, nil
}

func (c *clientImplementor) IsPerforceSuperUser(ctx context.Context, conn protocol.PerforceConnectionDetails) error {
	if conf.IsGRPCEnabled(ctx) {
		// p4port is not actually a repo name, but it will spread the load of CheckPerforceCredentials
		// a bit over the different gitserver instances. It's really just used as a consistent hashing
		// key here.
		client, err := c.ClientForRepo(ctx, api.RepoName(conn.P4Port))
		if err != nil {
			return err
		}
		_, err = client.IsPerforceSuperUser(ctx, &proto.IsPerforceSuperUserRequest{
			ConnectionDetails: conn.ToProto(),
		})
		return err
	}

	addr := c.AddrForRepo(ctx, api.RepoName(conn.P4Port))
	b, err := json.Marshal(&protocol.IsPerforceSuperUserRequest{
		P4Port:   conn.P4Port,
		P4User:   conn.P4User,
		P4Passwd: conn.P4Passwd,
	})
	if err != nil {
		return err
	}

	uri := "http://" + addr + "/is-perforce-super-user"
	resp, err := c.do(ctx, "is-perforce-super-user", uri, b)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &url.Error{
			URL: resp.Request.URL.String(),
			Op:  "IsPerforceSuperUser",
			Err: errors.Errorf("IsPerforceSuperUser: http status %d: %s", resp.StatusCode, readResponseBody(io.LimitReader(resp.Body, 200))),
		}
	}

	return nil
}

func (c *clientImplementor) PerforceGetChangelist(ctx context.Context, conn protocol.PerforceConnectionDetails, changelist string) (*perforce.Changelist, error) {
	if conf.IsGRPCEnabled(ctx) {
		// p4port is not actually a repo name, but it will spread the load of CheckPerforceCredentials
		// a bit over the different gitserver instances. It's really just used as a consistent hashing
		// key here.
		client, err := c.ClientForRepo(ctx, api.RepoName(conn.P4Port))
		if err != nil {
			return nil, err
		}
		resp, err := client.PerforceGetChangelist(ctx, &proto.PerforceGetChangelistRequest{
			ConnectionDetails: conn.ToProto(),
			ChangelistId:      changelist,
		})
		if err != nil {
			return nil, err
		}

		return perforce.ChangelistFromProto(resp.GetChangelist()), nil
	}

	addr := c.AddrForRepo(ctx, api.RepoName(conn.P4Port))
	b, err := json.Marshal(&protocol.PerforceGetChangelistRequest{
		P4Port:       conn.P4Port,
		P4User:       conn.P4User,
		P4Passwd:     conn.P4Passwd,
		ChangelistID: changelist,
	})
	if err != nil {
		return nil, err
	}

	uri := "http://" + addr + "/perforce-get-changelist"
	resp, err := c.do(ctx, "perforce-get-changelist", uri, b)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, &url.Error{
			URL: resp.Request.URL.String(),
			Op:  "PerforceGetChangelist",
			Err: errors.Errorf("PerforceGetChangelist: http status %d: %s", resp.StatusCode, readResponseBody(io.LimitReader(resp.Body, 200))),
		}
	}

	var payload protocol.PerforceGetChangelistResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	cl := &perforce.Changelist{
		ID:           payload.Changelist.ID,
		CreationDate: payload.Changelist.CreationDate,
		State:        perforce.ChangelistState(payload.Changelist.State),
		Author:       payload.Changelist.Author,
		Title:        payload.Changelist.Title,
		Message:      payload.Changelist.Message,
	}

	return cl, nil
}

// httpPost will apply the MD5 hashing scheme on the repo name to determine the gitserver instance
// to which the HTTP POST request is sent.
func (c *clientImplementor) httpPost(ctx context.Context, repo api.RepoName, op string, payload any) (resp *http.Response, err error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	addrForRepo := c.AddrForRepo(ctx, repo)
	uri := "http://" + addrForRepo + "/" + op
	return c.do(ctx, repo, uri, b)
}

// do performs a request to a gitserver instance based on the address in the uri
// argument.
//
// repoForTracing parameter is optional. If it is provided, then "repo" attribute is added
// to trace span.
func (c *clientImplementor) do(ctx context.Context, repoForTracing api.RepoName, uri string, payload []byte) (resp *http.Response, err error) {
	method := http.MethodPost
	parsedURL, err := url.ParseRequestURI(uri)
	if err != nil {
		return nil, errors.Wrap(err, "do")
	}

	ctx, trLogger, endObservation := c.operations.do.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		repoForTracing.Attr(),
		attribute.String("method", method),
		attribute.String("path", parsedURL.Path),
	}})
	defer endObservation(1, observation.Args{})

	req, err := http.NewRequestWithContext(ctx, method, uri, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.userAgent)

	// Set header so that the server knows the request is from us.
	req.Header.Set("X-Requested-With", "Sourcegraph")

	c.httpLimiter.Acquire()
	defer c.httpLimiter.Release()

	trLogger.AddEvent("Acquired HTTP limiter")

	return c.httpClient.Do(req)
}

func (c *clientImplementor) CreateCommitFromPatch(ctx context.Context, req protocol.CreateCommitFromPatchRequest) (*protocol.CreateCommitFromPatchResponse, error) {
	if conf.IsGRPCEnabled(ctx) {
		client, err := c.ClientForRepo(ctx, req.Repo)
		if err != nil {
			return nil, err
		}

		cc, err := client.CreateCommitFromPatchBinary(ctx)
		if err != nil {
			st, ok := status.FromError(err)
			if ok {
				for _, detail := range st.Details() {
					switch dt := detail.(type) {
					case *proto.CreateCommitFromPatchError:
						var e protocol.CreateCommitFromPatchError
						e.FromProto(dt)
						return nil, &e
					}
				}
			}
			return nil, err
		}

		// Send the metadata event first.
		if err := cc.Send(&proto.CreateCommitFromPatchBinaryRequest{Payload: &proto.CreateCommitFromPatchBinaryRequest_Metadata_{
			Metadata: req.ToMetadataProto(),
		}}); err != nil {
			return nil, errors.Wrap(err, "sending metadata")
		}

		// Then create a writer that sends data in chunks that won't exceed the maximum
		// message size of gRPC of the patch in separate events.
		w := streamio.NewWriter(func(p []byte) error {
			req := &proto.CreateCommitFromPatchBinaryRequest{
				Payload: &proto.CreateCommitFromPatchBinaryRequest_Patch_{
					Patch: &proto.CreateCommitFromPatchBinaryRequest_Patch{
						Data: p,
					},
				},
			}
			return cc.Send(req)
		})

		if _, err := w.Write(req.Patch); err != nil {
			return nil, errors.Wrap(err, "writing chunk of patch")
		}

		resp, err := cc.CloseAndRecv()
		if err != nil {
			st, ok := status.FromError(err)
			if !ok {
				return nil, err
			}

			for _, detail := range st.Details() {
				switch dt := detail.(type) {
				case *proto.CreateCommitFromPatchError:
					var e protocol.CreateCommitFromPatchError
					e.FromProto(dt)
					return nil, &e
				}
			}

			return nil, err
		}

		var res protocol.CreateCommitFromPatchResponse
		res.FromProto(resp, nil)

		return &res, nil
	}

	resp, err := c.httpPost(ctx, req.Repo, "create-commit-from-patch-binary", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read response body")
	}
	var res protocol.CreateCommitFromPatchResponse
	if err := json.Unmarshal(body, &res); err != nil {
		c.logger.Warn("decoding gitserver create-commit-from-patch response", sglog.Error(err))
		return nil, &url.Error{
			URL: resp.Request.URL.String(),
			Op:  "CreateCommitFromPatch",
			Err: errors.Errorf("CreateCommitFromPatch: http status %d, %s", resp.StatusCode, string(body)),
		}
	}

	if res.Error != nil {
		return &res, res.Error
	}
	return &res, nil
}

func (c *clientImplementor) GetObject(ctx context.Context, repo api.RepoName, objectName string) (*gitdomain.GitObject, error) {
	if ClientMocks.GetObject != nil {
		return ClientMocks.GetObject(repo, objectName)
	}

	req := protocol.GetObjectRequest{
		Repo:       repo,
		ObjectName: objectName,
	}
	if conf.IsGRPCEnabled(ctx) {
		client, err := c.ClientForRepo(ctx, req.Repo)
		if err != nil {
			return nil, err
		}

		grpcResp, err := client.GetObject(ctx, req.ToProto())
		if err != nil {

			return nil, err
		}

		var res protocol.GetObjectResponse
		res.FromProto(grpcResp)

		return &res.Object, nil

	} else {
		resp, err := c.httpPost(ctx, req.Repo, "commands/get-object", req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			c.logger.Warn("reading gitserver get-object response", sglog.Error(err))
			return nil, &url.Error{
				URL: resp.Request.URL.String(),
				Op:  "GetObject",
				Err: errors.Errorf("GetObject: http status %d, %s", resp.StatusCode, readResponseBody(resp.Body)),
			}
		}
		var res protocol.GetObjectResponse
		if err = json.NewDecoder(resp.Body).Decode(&res); err != nil {
			c.logger.Warn("decoding gitserver get-object response", sglog.Error(err))
			return nil, &url.Error{
				URL: resp.Request.URL.String(),
				Op:  "GetObject",
				Err: errors.Errorf("GetObject: http status %d, failed to decode response body: %v", resp.StatusCode, err),
			}
		}

		return &res.Object, nil
	}
}

var ambiguousArgPattern = lazyregexp.New(`ambiguous argument '([^']+)'`)

func (c *clientImplementor) ResolveRevisions(ctx context.Context, repo api.RepoName, revs []protocol.RevisionSpecifier) ([]string, error) {
	args := append([]string{"rev-parse"}, revsToGitArgs(revs)...)

	cmd := c.gitCommand(repo, args...)
	stdout, stderr, err := cmd.DividedOutput(ctx)
	if err != nil {
		if gitdomain.IsRepoNotExist(err) {
			return nil, err
		}
		if match := ambiguousArgPattern.FindSubmatch(stderr); match != nil {
			return nil, &gitdomain.RevisionNotFoundError{Repo: repo, Spec: string(match[1])}
		}
		return nil, errors.WithMessage(err, fmt.Sprintf("git command %v failed (stderr: %q)", cmd.Args(), stderr))
	}

	return strings.Fields(string(stdout)), nil
}

func revsToGitArgs(revSpecs []protocol.RevisionSpecifier) []string {
	args := make([]string, 0, len(revSpecs))
	for _, r := range revSpecs {
		if r.RevSpec != "" {
			args = append(args, r.RevSpec)
		} else if r.RefGlob != "" {
			args = append(args, "--glob="+r.RefGlob)
		} else if r.ExcludeRefGlob != "" {
			args = append(args, "--exclude="+r.ExcludeRefGlob)
		} else {
			args = append(args, "HEAD")
		}
	}

	// If revSpecs is empty, git treats it as equivalent to HEAD
	if len(revSpecs) == 0 {
		args = append(args, "HEAD")
	}
	return args
}

// readResponseBody will attempt to read the body of the HTTP response and return it as a
// string. However, in the unlikely scenario that it fails to read the body, it will encode and
// return the error message as a string.
//
// This allows us to use this function directly without yet another if err != nil check. As a
// result, this function should **only** be used when we're attempting to return the body's content
// as part of an error. In such scenarios we don't need to return the potential error from reading
// the body, but can get away with returning that error as a string itself.
//
// This is an unusual pattern of not returning an error. Be careful of replicating this in other
// parts of the code.
func readResponseBody(body io.Reader) string {
	content, err := io.ReadAll(body)
	if err != nil {
		return fmt.Sprintf("failed to read response body, error: %v", err)
	}

	// strings.TrimSpace is needed to remove trailing \n characters that is added by the
	// server. We use http.Error in the server which in turn uses fmt.Fprintln to format
	// the error message. And in translation that newline gets escaped into a \n
	// character.  For what the error message would look in the UI without
	// strings.TrimSpace, see attached screenshots in this pull request:
	// https://github.com/sourcegraph/sourcegraph/pull/39358.
	return strings.TrimSpace(string(content))
}

func stringsToByteSlices(in []string) [][]byte {
	res := make([][]byte, len(in))
	for i, s := range in {
		res[i] = []byte(s)
	}
	return res
}
