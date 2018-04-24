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
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/neelance/parallel"
	"github.com/opentracing-contrib/go-stdlib/nethttp"
	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/ext"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/sourcegraph/sourcegraph/pkg/api"
	"github.com/sourcegraph/sourcegraph/pkg/env"
	"github.com/sourcegraph/sourcegraph/pkg/gitserver/protocol"
	"github.com/sourcegraph/sourcegraph/pkg/vcs"
	"golang.org/x/net/context/ctxhttp"
)

var gitservers = env.Get("SRC_GIT_SERVERS", "gitserver:3178", "addresses of the remote gitservers")

// DefaultClient is the default Client. Unless overwritten it is connected to servers specified by SRC_GIT_SERVERS.
var DefaultClient = &Client{
	Addrs: strings.Fields(gitservers),
	HTTPClient: &http.Client{
		// nethttp.Transport will propagate opentracing spans
		Transport: &nethttp.Transport{
			RoundTripper: &http.Transport{
				// Default is 2, but we can send many concurrent requests
				MaxIdleConnsPerHost: 500,
			},
		},
	},
	HTTPLimiter: parallel.NewRun(500),
}

// Client is a gitserver client.
type Client struct {
	// HTTP client to use
	HTTPClient *http.Client

	// Limits concurrency of outstanding HTTP posts
	HTTPLimiter *parallel.Run

	Addrs []string
}

// addrForRepo returns the gitserver address to use for the given repo URI.
func (c *Client) addrForRepo(repo api.RepoURI) string {
	repo = protocol.NormalizeRepo(repo) // in case the caller didn't already normalize it
	sum := md5.Sum([]byte(repo))
	serverIndex := binary.BigEndian.Uint64(sum[:]) % uint64(len(c.Addrs))
	return c.Addrs[serverIndex]
}

func (c *Cmd) sendExec(ctx context.Context) (_ io.ReadCloser, _ http.Header, errRes error) {
	repoURI := protocol.NormalizeRepo(c.Repo.Name)

	span, ctx := opentracing.StartSpanFromContext(ctx, "Client.sendExec")
	defer func() {
		if errRes != nil {
			ext.Error.Set(span, true)
			span.SetTag("err", errRes.Error())
		}
		span.Finish()
	}()
	span.SetTag("request", "Exec")
	span.SetTag("repo", c.Repo.Name)
	span.SetTag("remoteURL", c.Repo.URL)
	span.SetTag("args", c.Args[1:])

	// Check that ctx is not expired.
	if err := ctx.Err(); err != nil {
		deadlineExceededCounter.Inc()
		return nil, nil, err
	}

	req := &protocol.ExecRequest{
		Repo:           repoURI,
		URL:            c.Repo.URL,
		EnsureRevision: c.EnsureRevision,
		Args:           c.Args[1:],
	}
	resp, err := c.client.httpPost(ctx, repoURI, "exec", req)
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
		return nil, nil, &vcs.RepoNotExistError{Repo: repoURI, CloneInProgress: payload.CloneInProgress}

	default:
		resp.Body.Close()
		return nil, nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
}

var deadlineExceededCounter = prometheus.NewCounter(prometheus.CounterOpts{
	Namespace: "src",
	Subsystem: "gitserver",
	Name:      "client_deadline_exceeded",
	Help:      "Times that Client.sendExec() returned context.DeadlineExceeded",
})

func init() {
	prometheus.MustRegister(deadlineExceededCounter)
}

// Cmd represents a command to be executed remotely.
type Cmd struct {
	client *Client

	Args           []string
	Repo           // the repository to execute the command in
	EnsureRevision string
	ExitStatus     int
}

// Repo represents a repository on gitserver. It contains the information necessary to identify and
// create/clone it.
type Repo struct {
	Name api.RepoURI // the repository's URI

	// URL is the repository's Git remote URL. If the gitserver already has cloned the repository,
	// this field is optional (it will use the last-used Git remote URL). If the repository is not
	// cloned on the gitserver, the request will fail.
	URL string
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

// ListGitolite lists Gitolite repositories.
func (c *Client) ListGitolite(ctx context.Context, gitoliteHost string) ([]string, error) {
	// The gitserver calls the shared Gitolite server in response to this request, so
	// we need to only call a single gitserver (or else we'd get duplicate results).
	return doListOne(ctx, "?gitolite="+url.QueryEscape(gitoliteHost), c.Addrs[0])
}

// GetGitolitePhabricatorMetadata returns Phabricator metadata for a
// Gitolite repository fetched via a user-provided command.
func (c *Client) GetGitolitePhabricatorMetadata(ctx context.Context, gitoliteHost string, repo string) (*protocol.GitolitePhabricatorMetadataResponse, error) {
	u := "http://" + c.Addrs[0] + "/getGitolitePhabricatorMetadata?gitolite=" + url.QueryEscape(gitoliteHost) + "&repo=" + url.QueryEscape(repo)
	resp, err := ctxhttp.Get(ctx, nil, u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var metadata protocol.GitolitePhabricatorMetadataResponse
	err = json.NewDecoder(resp.Body).Decode(&metadata)
	return &metadata, err
}

func doListOne(ctx context.Context, urlSuffix string, addr string) ([]string, error) {
	resp, err := ctxhttp.Get(ctx, nil, "http://"+addr+"/list"+urlSuffix)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var list []string
	err = json.NewDecoder(resp.Body).Decode(&list)
	return list, err
}

func (c *Client) EnqueueRepoUpdate(ctx context.Context, repo Repo) error {
	req := &protocol.RepoUpdateRequest{
		Repo: repo.Name,
		URL:  repo.URL,
	}
	_, err := c.httpPost(ctx, repo.Name, "enqueue-repo-update", req)
	if err != nil {
		return err
	}
	return nil
}

// MockIsRepoCloneable mocks (*Client).IsRepoCloneable for tests.
var MockIsRepoCloneable func(Repo) error

// IsRepoCloneable returns nil if the repository is cloneable.
func (c *Client) IsRepoCloneable(ctx context.Context, repo Repo) error {
	if MockIsRepoCloneable != nil {
		return MockIsRepoCloneable(repo)
	}

	req := &protocol.IsRepoCloneableRequest{
		Repo: repo.Name,
		URL:  repo.URL,
	}
	r, err := c.httpPost(ctx, repo.Name, "is-repo-cloneable", req)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	// Try unmarshaling new response format (?v=2) first.
	var resp protocol.IsRepoCloneableResponse
	if err := json.Unmarshal(body, &resp); err == nil {
		if resp.Cloneable {
			return nil
		}
		notFound := strings.Contains(resp.Reason, "not found")
		return &RepoNotCloneableErr{repo: repo, reason: resp.Reason, notFound: notFound}
	}

	// Backcompat (gitserver is old, does not recognize ?v=2)
	//
	// TODO(sqs): remove when unneeded
	var cloneable bool
	if err := json.Unmarshal(body, &cloneable); err != nil {
		return err
	}
	if cloneable {
		return nil
	}
	return &RepoNotCloneableErr{}
}

// RepoNotCloneableErr is the error that happens when a repository can not be cloned.
type RepoNotCloneableErr struct {
	repo     Repo
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
	return fmt.Sprintf("repo not found (name=%s url=%s) because %s", e.repo.Name, e.repo.URL, e.reason)
}

func (c *Client) IsRepoCloned(ctx context.Context, repo api.RepoURI) (bool, error) {
	req := &protocol.IsRepoClonedRequest{
		Repo: repo,
	}
	resp, err := c.httpPost(ctx, repo, "is-repo-cloned", req)
	if err != nil {
		return false, err
	}
	var cloned bool
	if resp.StatusCode == http.StatusOK {
		cloned = true
	}
	return cloned, nil
}

// RepoInfo retrieves information about the repository on gitserver.
//
// The repository not existing is not an error; in that case, RepoInfoResponse.Cloned will be false
// and the error will be nil.
func (c *Client) RepoInfo(ctx context.Context, repo api.RepoURI) (*protocol.RepoInfoResponse, error) {
	req := &protocol.RepoInfoRequest{
		Repo: repo,
	}
	resp, err := c.httpPost(ctx, repo, "repo", req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, &url.Error{URL: resp.Request.URL.String(), Op: "RepoInfo", Err: fmt.Errorf("RepoInfo: http status %d", resp.StatusCode)}
	}

	var info *protocol.RepoInfoResponse
	err = json.NewDecoder(resp.Body).Decode(&info)
	return info, err
}

func (c *Client) httpPost(ctx context.Context, repo api.RepoURI, method string, payload interface{}) (resp *http.Response, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Client.httpPost")
	defer func() {
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

	addr := c.addrForRepo(repo)
	req, err := http.NewRequest("POST", "http://"+addr+"/"+method, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(ctx)

	if c.HTTPLimiter != nil {
		span.LogKV("event", "Waiting on HTTP limiter")
		c.HTTPLimiter.Acquire()
		defer c.HTTPLimiter.Release()
		span.LogKV("event", "Acquired HTTP limiter")
	}

	req, ht := nethttp.TraceRequest(opentracing.GlobalTracer(), req,
		nethttp.OperationName("Gitserver Client"),
		nethttp.ClientTrace(false))
	defer ht.Finish()

	if c.HTTPClient != nil {
		return c.HTTPClient.Do(req)
	} else {
		return http.DefaultClient.Do(req)
	}
}

func (c *Client) UploadPack(repoURI api.RepoURI, w http.ResponseWriter, r *http.Request) {
	repoURI = protocol.NormalizeRepo(repoURI)
	addr := c.addrForRepo(repoURI)

	u, err := url.Parse("http://" + addr + "/upload-pack?repo=" + url.QueryEscape(string(repoURI)))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	(&httputil.ReverseProxy{
		Director: func(r *http.Request) {
			r.URL = u
		},
		ErrorLog: uploadPackErrorLog,
	}).ServeHTTP(w, r)
}

var uploadPackErrorLog = log.New(env.DebugOut, "git upload-pack proxy: ", log.LstdFlags)
