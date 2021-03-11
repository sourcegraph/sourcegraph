package gitserver

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hashicorp/go-multierror"
	"github.com/inconshreveable/log15"
	"github.com/neelance/parallel"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go/ext"
	otlog "github.com/opentracing/opentracing-go/log"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitolite"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/metrics"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"
	"github.com/sourcegraph/sourcegraph/internal/vcs"
)

var requestMeter = metrics.NewRequestMeter("gitserver", "Total number of requests sent to gitserver.")

// defaultTransport is the default transport used in the default client and the
// default reverse proxy. ot.Transport will propagate opentracing spans.
var defaultTransport = &ot.Transport{
	RoundTripper: requestMeter.Transport(&http.Transport{
		// Default is 2, but we can send many concurrent requests
		MaxIdleConnsPerHost: 500,
	}, func(u *url.URL) string {
		// break it down by API function call (ie "/archive", "/exec", "/is-repo-cloneable", etc)
		return u.Path
	}),
}

// DefaultClient is the default Client. Unless overwritten it is connected to servers specified by SRC_GIT_SERVERS.
var DefaultClient = NewClient(&http.Client{Transport: defaultTransport})

// NewClient returns a new gitserver.Client instantiated with default arguments
// and httpcli.Doer.
func NewClient(cli httpcli.Doer) *Client {
	return &Client{
		Addrs: func() []string {
			return conf.Get().ServiceConnections.GitServers
		},
		HTTPClient:  cli,
		HTTPLimiter: parallel.NewRun(500),
		// Use the binary name for UserAgent. This should effectively identify
		// which service is making the request (excluding requests proxied via the
		// frontend internal API)
		UserAgent: filepath.Base(os.Args[0]),
	}
}

// Client is a gitserver client.
type Client struct {
	// HTTP client to use
	HTTPClient httpcli.Doer

	// Limits concurrency of outstanding HTTP posts
	HTTPLimiter *parallel.Run

	// Addrs is a function which should return the addresses for gitservers. It
	// is called each time a request is made. The function must be safe for
	// concurrent use. It may return different results at different times.
	Addrs func() []string

	// UserAgent is a string identifying who the client is. It will be logged in
	// the telemetry in gitserver.
	UserAgent string
}

// AddrForRepo returns the gitserver address to use for the given repo name.
func (c *Client) AddrForRepo(repo api.RepoName) string {
	addrs := c.Addrs()
	if len(addrs) == 0 {
		panic("unexpected state: no gitserver addresses")
	}
	return AddrForRepo(repo, addrs)
}

// addrForKey returns the gitserver address to use for the given string key,
// which is hashed for sharding purposes.
func (c *Client) addrForKey(key string) string {
	addrs := c.Addrs()
	if len(addrs) == 0 {
		panic("unexpected state: no gitserver addresses")
	}
	return addrForKey(key, addrs)
}

// AddrForRepo returns the gitserver address to use for the given repo name.
// It should never be called with an empty slice.
func AddrForRepo(repo api.RepoName, addrs []string) string {
	repo = protocol.NormalizeRepo(repo) // in case the caller didn't already normalize it
	return addrForKey(string(repo), addrs)
}

// addrForKey returns the gitserver address to use for the given string key,
// which is hashed for sharding purposes.
func addrForKey(key string, addrs []string) string {
	sum := md5.Sum([]byte(key))
	serverIndex := binary.BigEndian.Uint64(sum[:]) % uint64(len(addrs))
	return addrs[serverIndex]
}

// ArchiveOptions contains options for the Archive func.
type ArchiveOptions struct {
	Treeish string   // the tree or commit to produce an archive for
	Format  string   // format of the resulting archive (usually "tar" or "zip")
	Paths   []string // if nonempty, only include these paths
}

// archiveReader wraps the StdoutReader yielded by gitserver's
// Cmd.StdoutReader with one that knows how to report a repository-not-found
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
		if strings.Contains(err.Error(), "Not a valid object") {
			return 0, &RevisionNotFoundError{Repo: a.repo, Spec: a.spec}
		}
	}
	return n, err
}

func (a *archiveReader) Close() error {
	return a.base.Close()
}

// ArchiveURL returns a URL from which an archive of the given Git repository can
// be downloaded from.
func (c *Client) ArchiveURL(repo api.RepoName, opt ArchiveOptions) *url.URL {
	q := url.Values{
		"repo":    {string(repo)},
		"treeish": {opt.Treeish},
		"format":  {opt.Format},
	}

	for _, path := range opt.Paths {
		q.Add("path", path)
	}

	return &url.URL{
		Scheme:   "http",
		Host:     c.AddrForRepo(repo),
		Path:     "/archive",
		RawQuery: q.Encode(),
	}
}

// Archive produces an archive from a Git repository.
func (c *Client) Archive(ctx context.Context, repo api.RepoName, opt ArchiveOptions) (_ io.ReadCloser, err error) {
	span, ctx := ot.StartSpanFromContext(ctx, "Git: Archive")
	span.SetTag("Repo", repo)
	span.SetTag("Treeish", opt.Treeish)
	defer func() {
		if err != nil {
			ext.Error.Set(span, true)
			span.LogFields(otlog.Error(err))
		}
		span.Finish()
	}()

	// Check that ctx is not expired.
	if err := ctx.Err(); err != nil {
		deadlineExceededCounter.Inc()
		return nil, err
	}

	u := c.ArchiveURL(repo, opt)
	resp, err := c.do(ctx, repo, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return &archiveReader{
			base: &cmdReader{
				rc:      resp.Body,
				trailer: resp.Trailer,
			},
			repo: repo,
			spec: opt.Treeish,
		}, nil
	case http.StatusNotFound:
		var payload protocol.NotFoundPayload
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			resp.Body.Close()
			return nil, err
		}
		resp.Body.Close()
		return nil, &badRequestError{
			error: &vcs.RepoNotExistError{
				Repo:            repo,
				CloneInProgress: payload.CloneInProgress,
				CloneProgress:   payload.CloneProgress,
			},
		}
	default:
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
}

type badRequestError struct{ error }

func (e badRequestError) BadRequest() bool { return true }

func (c *Cmd) sendExec(ctx context.Context) (_ io.ReadCloser, _ http.Header, errRes error) {
	repoName := protocol.NormalizeRepo(c.Repo)

	span, ctx := ot.StartSpanFromContext(ctx, "Client.sendExec")
	defer func() {
		if errRes != nil {
			ext.Error.Set(span, true)
			span.SetTag("err", errRes.Error())
		}
		span.Finish()
	}()
	span.SetTag("request", "Exec")
	span.SetTag("repo", c.Repo)
	span.SetTag("args", c.Args[1:])

	// Check that ctx is not expired.
	if err := ctx.Err(); err != nil {
		deadlineExceededCounter.Inc()
		return nil, nil, err
	}

	req := &protocol.ExecRequest{
		Repo:           repoName,
		EnsureRevision: c.EnsureRevision,
		Args:           c.Args[1:],
	}
	resp, err := c.client.httpPost(ctx, repoName, "exec", req)
	if err != nil {
		return nil, nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return resp.Body, resp.Trailer, nil

	case http.StatusNotFound:
		var payload protocol.NotFoundPayload
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			resp.Body.Close()
			return nil, nil, err
		}
		resp.Body.Close()
		return nil, nil, &vcs.RepoNotExistError{Repo: repoName, CloneInProgress: payload.CloneInProgress, CloneProgress: payload.CloneProgress}

	default:
		resp.Body.Close()
		return nil, nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
}

// P4Exec sends a p4 command with given arguments and returns an io.ReadCloser for the output.
func (c *Client) P4Exec(ctx context.Context, host, user, password string, args ...string) (_ io.ReadCloser, _ http.Header, errRes error) {
	span, ctx := ot.StartSpanFromContext(ctx, "Client.P4Exec")
	defer func() {
		if errRes != nil {
			ext.Error.Set(span, true)
			span.SetTag("err", errRes.Error())
		}
		span.Finish()
	}()
	span.SetTag("request", "P4Exec")
	span.SetTag("host", host)
	span.SetTag("args", args)

	// Check that ctx is not expired.
	if err := ctx.Err(); err != nil {
		deadlineExceededCounter.Inc()
		return nil, nil, err
	}

	req := &protocol.P4ExecRequest{
		P4Port:   host,
		P4User:   user,
		P4Passwd: password,
		Args:     args,
	}
	resp, err := c.httpPost(ctx, "", "p4-exec", req)
	if err != nil {
		return nil, nil, err
	}

	switch resp.StatusCode {
	case http.StatusOK:
		return resp.Body, resp.Trailer, nil

	default:
		// Read response body at best effort
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, nil, fmt.Errorf("unexpected status code: %d - %s", resp.StatusCode, body)
	}
}

var deadlineExceededCounter = promauto.NewCounter(prometheus.CounterOpts{
	Name: "src_gitserver_client_deadline_exceeded",
	Help: "Times that Client.sendExec() returned context.DeadlineExceeded",
})

// Cmd represents a command to be executed remotely.
type Cmd struct {
	client *Client

	Args           []string
	Repo           api.RepoName // the repository to execute the command in
	EnsureRevision string
	ExitStatus     int
}

// Command creates a new Cmd. Command name must be 'git',
// otherwise it panics.
func (c *Client) Command(name string, arg ...string) *Cmd {
	if name != "git" {
		panic("gitserver: command name must be 'git'")
	}
	return &Cmd{
		client: c,
		Args:   append([]string{"git"}, arg...),
	}
}

// DividedOutput runs the command and returns its standard output and standard error.
func (c *Cmd) DividedOutput(ctx context.Context) ([]byte, []byte, error) {
	rc, trailer, err := c.sendExec(ctx)
	if err != nil {
		return nil, nil, err
	}

	stdout, err := ioutil.ReadAll(rc)
	rc.Close()
	if err != nil {
		return nil, nil, err
	}

	c.ExitStatus, err = strconv.Atoi(trailer.Get("X-Exec-Exit-Status"))
	if err != nil {
		return nil, nil, err
	}

	stderr := []byte(trailer.Get("X-Exec-Stderr"))
	if errorMsg := trailer.Get("X-Exec-Error"); errorMsg != "" {
		return stdout, stderr, errors.New(errorMsg)
	}

	return stdout, stderr, nil
}

// Run starts the specified command and waits for it to complete.
func (c *Cmd) Run(ctx context.Context) error {
	_, _, err := c.DividedOutput(ctx)
	return err
}

// Output runs the command and returns its standard output.
func (c *Cmd) Output(ctx context.Context) ([]byte, error) {
	stdout, _, err := c.DividedOutput(ctx)
	return stdout, err
}

// CombinedOutput runs the command and returns its combined standard output and standard error.
func (c *Cmd) CombinedOutput(ctx context.Context) ([]byte, error) {
	stdout, stderr, err := c.DividedOutput(ctx)
	return append(stdout, stderr...), err
}

func (c *Cmd) String() string { return fmt.Sprintf("%q", c.Args) }

// StdoutReader returns an io.ReadCloser of stdout of c. If the command has a
// non-zero return value, Read returns a non io.EOF error. Do not pass in a
// started command.
func StdoutReader(ctx context.Context, c *Cmd) (io.ReadCloser, error) {
	rc, trailer, err := c.sendExec(ctx)
	if err != nil {
		return nil, err
	}

	return &cmdReader{
		rc:      rc,
		trailer: trailer,
	}, nil
}

type cmdReader struct {
	rc      io.ReadCloser
	trailer http.Header
}

func (c *cmdReader) Read(p []byte) (int, error) {
	n, err := c.rc.Read(p)
	if err == io.EOF {
		stderr := c.trailer.Get("X-Exec-Stderr")
		if len(stderr) > 100 {
			stderr = stderr[:100] + "... (truncated)"
		}
		if errorMsg := c.trailer.Get("X-Exec-Error"); errorMsg != "" {
			return 0, fmt.Errorf("%s (stderr: %q)", errorMsg, stderr)
		}
		if exitStatus := c.trailer.Get("X-Exec-Exit-Status"); exitStatus != "0" {
			return 0, fmt.Errorf("non-zero exit status: %s (stderr: %q)", exitStatus, stderr)
		}
	}
	return n, err
}

func (c *cmdReader) Close() error {
	return c.rc.Close()
}

// WaitForGitServers retries a noop request to all gitserver instances until
// getting back a successful response.
func (c *Client) WaitForGitServers(ctx context.Context) error {
	for {
		if errs := c.pingAll(ctx); len(errs) == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func (c *Client) pingAll(ctx context.Context) []error {
	addrs := c.Addrs()

	ch := make(chan error, len(addrs))
	for _, addr := range addrs {
		go func(addr string) {
			ch <- c.ping(ctx, addr)
		}(addr)
	}

	errs := make([]error, 0, len(addrs))
	for i := 0; i < cap(ch); i++ {
		if err := <-ch; err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

func (c *Client) ping(ctx context.Context, addr string) error {
	req, err := http.NewRequest("GET", "http://"+addr+"/ping", nil)
	if err != nil {
		return err
	}

	resp, err := c.HTTPClient.Do(req.WithContext(ctx))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ping: bad HTTP response status %d", resp.StatusCode)
	}

	return nil
}

// ListGitolite lists Gitolite repositories.
func (c *Client) ListGitolite(ctx context.Context, gitoliteHost string) (list []*gitolite.Repo, err error) {
	// The gitserver calls the shared Gitolite server in response to this request, so
	// we need to only call a single gitserver (or else we'd get duplicate results).
	addr := c.addrForKey(gitoliteHost)
	req, err := http.NewRequest("GET", "http://"+addr+"/list-gitolite?gitolite="+url.QueryEscape(gitoliteHost), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	err = json.NewDecoder(resp.Body).Decode(&list)
	return list, err
}

// ListCloned lists all cloned repositories
func (c *Client) ListCloned(ctx context.Context) ([]string, error) {
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		err   error
		repos []string
	)
	addrs := c.Addrs()
	for _, addr := range addrs {
		wg.Add(1)
		go func(addr string) {
			defer wg.Done()
			r, e := c.doListOne(ctx, "?cloned", addr)

			// Only include repos that belong on addr.
			if len(r) > 0 {
				filtered := r[:0]
				for _, repo := range r {
					if addrForKey(repo, addrs) == addr {
						filtered = append(filtered, repo)
					}
				}
				r = filtered
			}
			mu.Lock()
			if e != nil {
				err = e
			}
			repos = append(repos, r...)
			mu.Unlock()
		}(addr)
	}
	wg.Wait()
	return repos, err
}

// GetGitolitePhabricatorMetadata returns Phabricator metadata for a Gitolite repository fetched via
// a user-provided command.
func (c *Client) GetGitolitePhabricatorMetadata(ctx context.Context, gitoliteHost string, repoName api.RepoName) (*protocol.GitolitePhabricatorMetadataResponse, error) {
	u := "http://" + c.addrForKey(gitoliteHost) +
		"/getGitolitePhabricatorMetadata?gitolite=" + url.QueryEscape(gitoliteHost) +
		"&repo=" + url.QueryEscape(string(repoName))

	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var metadata protocol.GitolitePhabricatorMetadataResponse
	err = json.NewDecoder(resp.Body).Decode(&metadata)
	return &metadata, err
}

func (c *Client) doListOne(ctx context.Context, urlSuffix, addr string) ([]string, error) {
	req, err := http.NewRequest("GET", "http://"+addr+"/list"+urlSuffix, nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req.WithContext(ctx))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var list []string
	err = json.NewDecoder(resp.Body).Decode(&list)
	return list, err
}

// RequestRepoUpdate is the new protocol endpoint for synchronous requests
// with more detailed responses. Do not use this if you are not repo-updater.
//
// Repo updates are not guaranteed to occur. If a repo has been updated
// recently (within the Since duration specified in the request), the
// update won't happen.
func (c *Client) RequestRepoUpdate(ctx context.Context, repo api.RepoName, since time.Duration) (*protocol.RepoUpdateResponse, error) {
	req := &protocol.RepoUpdateRequest{
		Repo:  repo,
		Since: since,
	}
	resp, err := c.httpPost(ctx, repo, "repo-update", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 200))
		return nil, &url.Error{URL: resp.Request.URL.String(), Op: "RepoInfo", Err: fmt.Errorf("RepoInfo: http status %d: %s", resp.StatusCode, body)}
	}

	var info *protocol.RepoUpdateResponse
	err = json.NewDecoder(resp.Body).Decode(&info)
	return info, err
}

// MockIsRepoCloneable mocks (*Client).IsRepoCloneable for tests.
var MockIsRepoCloneable func(api.RepoName) error

// IsRepoCloneable returns nil if the repository is cloneable.
func (c *Client) IsRepoCloneable(ctx context.Context, repo api.RepoName) error {
	if MockIsRepoCloneable != nil {
		return MockIsRepoCloneable(repo)
	}

	req := &protocol.IsRepoCloneableRequest{
		Repo: repo,
	}
	r, err := c.httpPost(ctx, repo, "is-repo-cloneable", req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	if r.StatusCode != http.StatusOK {
		return fmt.Errorf("gitserver error (status code %d): %s", r.StatusCode, string(body))
	}

	var resp protocol.IsRepoCloneableResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return err
	}

	if resp.Cloneable {
		return nil
	}

	// Treat all 4xx errors as not found, since we have more relaxed
	// requirements on what a valid URL is we should treat bad requests,
	// etc as not found.
	notFound := strings.Contains(resp.Reason, "not found") || strings.Contains(resp.Reason, "The requested URL returned error: 4")
	return &RepoNotCloneableErr{repo: repo, reason: resp.Reason, notFound: notFound}
}

// RepoNotCloneableErr is the error that happens when a repository can not be cloned.
type RepoNotCloneableErr struct {
	repo     api.RepoName
	reason   string
	notFound bool
}

// NotFound returns true if the repo could not be cloned because it wasn't found.
// This may be because the repo doesn't exist, or because the repo is private and
// there are insufficient permissions.
func (e *RepoNotCloneableErr) NotFound() bool {
	return e.notFound
}

func (e *RepoNotCloneableErr) Error() string {
	return fmt.Sprintf("repo not found (name=%s notfound=%v) because %s", e.repo, e.notFound, e.reason)
}

func (c *Client) IsRepoCloned(ctx context.Context, repo api.RepoName) (bool, error) {
	req := &protocol.IsRepoClonedRequest{
		Repo: repo,
	}
	resp, err := c.httpPost(ctx, repo, "is-repo-cloned", req)
	if err != nil {
		return false, err
	}
	// no need to defer, we aren't using the body.
	resp.Body.Close()
	var cloned bool
	if resp.StatusCode == http.StatusOK {
		cloned = true
	}
	return cloned, nil
}

func (c *Client) RepoCloneProgress(ctx context.Context, repos ...api.RepoName) (*protocol.RepoCloneProgressResponse, error) {
	numPossibleShards := len(c.Addrs())
	shards := make(map[string]*protocol.RepoCloneProgressRequest, (len(repos)/numPossibleShards)*2) // 2x because it may not be a perfect division

	for _, r := range repos {
		addr := c.AddrForRepo(r)
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

	err := new(multierror.Error)
	res := protocol.RepoCloneProgressResponse{
		Results: make(map[api.RepoName]*protocol.RepoCloneProgress),
	}

	for i := 0; i < cap(ch); i++ {
		o := <-ch

		if o.err != nil {
			err = multierror.Append(err, o.err)
			continue
		}

		for repo, info := range o.res.Results {
			res.Results[repo] = info
		}
	}

	return &res, err.ErrorOrNil()
}

// RepoInfo retrieves information about one or more repositories on gitserver.
//
// The repository not existing is not an error; in that case, RepoInfoResponse.Results[i].Cloned
// will be false and the error will be nil.
//
// If multiple errors occurred, an incomplete result is returned along with a
// *multierror.Error.
func (c *Client) RepoInfo(ctx context.Context, repos ...api.RepoName) (*protocol.RepoInfoResponse, error) {
	numPossibleShards := len(c.Addrs())
	shards := make(map[string]*protocol.RepoInfoRequest, (len(repos)/numPossibleShards)*2) // 2x because it may not be a perfect division

	for _, r := range repos {
		addr := c.AddrForRepo(r)
		shard := shards[addr]

		if shard == nil {
			shard = new(protocol.RepoInfoRequest)
			shards[addr] = shard
		}

		shard.Repos = append(shard.Repos, r)
	}

	type op struct {
		req *protocol.RepoInfoRequest
		res *protocol.RepoInfoResponse
		err error
	}

	ch := make(chan op, len(shards))
	for _, req := range shards {
		go func(o op) {
			var resp *http.Response
			resp, o.err = c.httpPost(ctx, o.req.Repos[0], "repos", o.req)
			if o.err != nil {
				ch <- o
				return
			}

			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				o.err = &url.Error{
					URL: resp.Request.URL.String(),
					Op:  "RepoInfo",
					Err: errors.Errorf("RepoInfo: http status %d", resp.StatusCode),
				}
				ch <- o
				return // we never get an error status code AND result
			}

			o.res = new(protocol.RepoInfoResponse)
			o.err = json.NewDecoder(resp.Body).Decode(o.res)
			ch <- o
		}(op{req: req})
	}

	err := new(multierror.Error)
	res := protocol.RepoInfoResponse{
		Results: make(map[api.RepoName]*protocol.RepoInfo),
	}

	for i := 0; i < cap(ch); i++ {
		o := <-ch

		if o.err != nil {
			err = multierror.Append(err, o.err)
			continue
		}

		for repo, info := range o.res.Results {
			res.Results[repo] = info
		}
	}

	return &res, err.ErrorOrNil()
}

// ReposStats will return a map of the ReposStats for each gitserver in a
// map. If we fail to fetch a stat from a gitserver, it won't be in the
// returned map and will be appended to the error. If no errors occur err will
// be nil.
//
// Note: If the statistics for a gitserver have not been computed, the
// UpdatedAt field will be zero. This can happen for new gitservers.
func (c *Client) ReposStats(ctx context.Context) (map[string]*protocol.ReposStats, error) {
	stats := map[string]*protocol.ReposStats{}
	var allErr error
	for _, addr := range c.Addrs() {
		stat, err := c.doReposStats(ctx, addr)
		if err != nil {
			allErr = multierror.Append(allErr, err)
		} else {
			stats[addr] = stat
		}
	}
	return stats, allErr
}

func (c *Client) doReposStats(ctx context.Context, addr string) (*protocol.ReposStats, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "http://"+addr+"/repos-stats", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var stats protocol.ReposStats
	err = json.NewDecoder(resp.Body).Decode(&stats)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}

// Remove removes the repository clone from gitserver.
func (c *Client) Remove(ctx context.Context, repo api.RepoName) error {
	req := &protocol.RepoDeleteRequest{
		Repo: repo,
	}
	resp, err := c.httpPost(ctx, repo, "delete", req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// best-effort inclusion of body in error message
		body, _ := ioutil.ReadAll(io.LimitReader(resp.Body, 200))
		return &url.Error{URL: resp.Request.URL.String(), Op: "RepoRemove", Err: fmt.Errorf("RepoRemove: http status %d: %s", resp.StatusCode, string(body))}
	}
	return nil
}

func (c *Client) httpPost(ctx context.Context, repo api.RepoName, op string, payload interface{}) (resp *http.Response, err error) {
	return c.do(ctx, repo, "POST", op, payload)
}

// do performs a request to a gitserver, sharding based on the given
// repo name (the repo name is otherwise not used).
func (c *Client) do(ctx context.Context, repo api.RepoName, method, op string, payload interface{}) (resp *http.Response, err error) {
	span, ctx := ot.StartSpanFromContext(ctx, "Client.do")
	defer func() {
		span.LogKV("repo", string(repo), "method", method, "op", op)
		if err != nil {
			ext.Error.Set(span, true)
			span.SetTag("err", err.Error())
		}
		span.Finish()
	}()

	reqBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	uri := op
	if !strings.HasPrefix(op, "http") {
		uri = "http://" + c.AddrForRepo(repo) + "/" + op
	}

	req, err := http.NewRequest(method, uri, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("X-Sourcegraph-Actor", userFromContext(ctx))
	req = req.WithContext(ctx)

	if c.HTTPLimiter != nil {
		c.HTTPLimiter.Acquire()
		defer c.HTTPLimiter.Release()
		span.LogKV("event", "Acquired HTTP limiter")
	}

	req, ht := nethttp.TraceRequest(span.Tracer(), req,
		nethttp.OperationName("Gitserver Client"),
		nethttp.ClientTrace(false))
	defer ht.Finish()

	return c.HTTPClient.Do(req)
}

func userFromContext(ctx context.Context) string {
	a := actor.FromContext(ctx)
	if a == nil {
		return "0"
	}
	if a.Internal {
		return "internal"
	}
	return a.UIDString()
}

// CreateCommitFromPatch will attempt to create a commit from a patch
// If possible, the error returned will be of type protocol.CreateCommitFromPatchError
func (c *Client) CreateCommitFromPatch(ctx context.Context, req protocol.CreateCommitFromPatchRequest) (string, error) {
	resp, err := c.httpPost(ctx, req.Repo, "create-commit-from-patch", req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log15.Warn("reading gitserver create-commit-from-patch response", "err", err.Error())
		return "", &url.Error{URL: resp.Request.URL.String(), Op: "CreateCommitFromPatch", Err: fmt.Errorf("CreateCommitFromPatch: http status %d %s", resp.StatusCode, err.Error())}
	}

	var res protocol.CreateCommitFromPatchResponse
	err = json.Unmarshal(data, &res)
	if err != nil {
		log15.Warn("decoding gitserver create-commit-from-patch response", "err", err.Error())
		return "", &url.Error{URL: resp.Request.URL.String(), Op: "CreateCommitFromPatch", Err: fmt.Errorf("CreateCommitFromPatch: http status %d %s", resp.StatusCode, string(data))}
	}

	if res.Error != nil {
		return res.Rev, res.Error
	}
	return res.Rev, nil
}
