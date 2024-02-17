// Package phabricator is a package to interact with a Phabricator instance and its Conduit API.
package phabricator

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/uber/gonduit"
	"github.com/uber/gonduit/core"
	"github.com/uber/gonduit/requests"

	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
	Name:    "src_phabricator_request_duration_seconds",
	Help:    "Time (in seconds) spent on request.",
	Buckets: prometheus.DefBuckets,
}, []string{"category", "code"})

type meteredConn struct {
	gonduit.Conn
}

func (mc *meteredConn) CallContext(
	ctx context.Context,
	method string,
	params any,
	result any,
) error {
	start := time.Now()
	err := mc.Conn.CallContext(ctx, method, params, result)
	d := time.Since(start)

	code := "200"
	if err != nil {
		code = "error"
	}
	requestDuration.WithLabelValues(method, code).Observe(d.Seconds())
	return err
}

// A Client provides high level methods to a Phabricator Conduit API.
type Client struct {
	conn *meteredConn
}

// NewClient returns an authenticated Client, using the given URL and
// token. If provided, cli will be used to perform the underlying HTTP requests.
// This constructor needs a context because it calls the Conduit API to negotiate
// capabilities as part of the dial process.
func NewClient(ctx context.Context, phabUrl, token string, cli httpcli.Doer) (*Client, error) {
	if cli == nil {
		cli = httpcli.ExternalDoer
	}

	conn, err := gonduit.DialContext(ctx, phabUrl, &core.ClientOptions{
		APIToken: token,
		Client:   httpcli.HeadersMiddleware("User-Agent", "sourcegraph/phabricator-client")(cli),
	})
	if err != nil {
		return nil, err
	}

	return &Client{conn: &meteredConn{*conn}}, nil
}

// Repo represents a single code repository.
type Repo struct {
	ID           uint64
	PHID         string
	Name         string
	VCS          string
	Callsign     string
	Shortname    string
	Status       string
	DateCreated  time.Time
	DateModified time.Time
	ViewPolicy   string
	EditPolicy   string
	URIs         []*URI
}

// URI of a Repository
type URI struct {
	ID   string
	PHID string

	Display    string
	Effective  string
	Normalized string

	Disabled bool

	BuiltinProtocol   string
	BuiltinIdentifier string

	DateCreated  time.Time
	DateModified time.Time
}

//
// Marshaling types
//

type apiRepo struct {
	ID          uint64             `json:"id"`
	PHID        string             `json:"phid"`
	Fields      apiRepoFields      `json:"fields"`
	Attachments apiRepoAttachments `json:"attachments"`
}

type apiRepoFields struct {
	Name         string        `json:"name"`
	VCS          string        `json:"vcs"`
	Callsign     string        `json:"callsign"`
	Shortname    string        `json:"shortname"`
	Status       string        `json:"status"`
	Policy       apiRepoPolicy `json:"policy"`
	DateCreated  unixTime      `json:"dateCreated"`
	DateModified unixTime      `json:"dateModified"`
}

type apiRepoPolicy struct {
	View string `json:"view"`
	Edit string `json:"edit"`
}

type apiRepoAttachments struct {
	URIs apiURIsContainer `json:"uris"`
}

type apiURIsContainer struct {
	URIs []apiURI `json:"uris"`
}

type apiURI struct {
	ID     string       `json:"id"`
	PHID   string       `json:"phid"`
	Fields apiURIFields `json:"fields"`
}

type apiURIFields struct {
	URI          apiURIs      `json:"uri"`
	Builtin      apiURIBultin `json:"builtin"`
	Disabled     bool         `json:"disabled"`
	DateCreated  unixTime     `json:"dateCreated"`
	DateModified unixTime     `json:"dateModified"`
}

type apiURIs struct {
	Display    string `json:"display"`
	Effective  string `json:"effective"`
	Normalized string `json:"normalized"`
}

type apiURIBultin struct {
	Protocol   string `json:"protocol"`
	Identifier string `json:"identifier"`
}

func (a *apiRepo) ToRepo() *Repo {
	r := &Repo{}

	r.ID = a.ID
	r.PHID = a.PHID
	r.Name = a.Fields.Name
	r.VCS = a.Fields.VCS
	r.Callsign = a.Fields.Callsign
	r.Shortname = a.Fields.Shortname
	r.Status = a.Fields.Status
	r.ViewPolicy = a.Fields.Policy.View
	r.EditPolicy = a.Fields.Policy.Edit
	if created := a.Fields.DateCreated.t; created != nil {
		r.DateCreated = *created
	}
	if modified := a.Fields.DateModified.t; modified != nil {
		r.DateModified = *modified
	}

	r.URIs = make([]*URI, 0, len(a.Attachments.URIs.URIs))
	for _, u := range a.Attachments.URIs.URIs {
		uri := URI{
			ID:                u.ID,
			PHID:              u.PHID,
			Display:           u.Fields.URI.Display,
			Effective:         u.Fields.URI.Effective,
			Normalized:        u.Fields.URI.Normalized,
			Disabled:          u.Fields.Disabled,
			BuiltinProtocol:   u.Fields.Builtin.Protocol,
			BuiltinIdentifier: u.Fields.Builtin.Identifier,
		}

		if t := u.Fields.DateCreated.t; t != nil {
			uri.DateCreated = *t
		}

		if t := u.Fields.DateModified.t; t != nil {
			uri.DateCreated = *t
		}

		r.URIs = append(r.URIs, &uri)
	}

	return r
}

// Cursor represents the pagination cursor on many responses.
type Cursor struct {
	Limit  uint64 `json:"limit,omitempty"`
	After  string `json:"after,omitempty"`
	Before string `json:"before,omitempty"`
	Order  string `json:"order,omitempty"`
}

// ListReposArgs defines the constraints to be satisfied
// by the ListRepos method.
type ListReposArgs struct {
	*Cursor
}

// ListRepos lists all repositories matching the given arguments.
func (c *Client) ListRepos(ctx context.Context, args ListReposArgs) ([]*Repo, *Cursor, error) {
	var req struct {
		requests.Request
		ListReposArgs
		Attachments struct {
			URIs bool `json:"uris"`
		} `json:"attachments"`
	}

	req.ListReposArgs = args
	req.Attachments.URIs = true

	if req.Cursor == nil {
		req.Cursor = new(Cursor)
	}

	if req.Cursor.Order == "" {
		req.Cursor.Order = "oldest"
	}

	if req.Cursor.Limit == 0 {
		req.Cursor.Limit = 100
	}

	var res struct {
		Data   []*apiRepo `json:"data"`
		Cursor Cursor     `json:"cursor"`
	}

	err := c.conn.CallContext(ctx, "diffusion.repository.search", &req, &res)
	if err != nil {
		return nil, nil, err
	}

	repos := make([]*Repo, len(res.Data))
	for i := range res.Data {
		repos[i] = res.Data[i].ToRepo()
	}

	return repos, &res.Cursor, nil
}

// GetRawDiff retrieves the raw diff of the diff with the given id.
func (c *Client) GetRawDiff(ctx context.Context, diffID int) (diff string, err error) {
	type request struct {
		requests.Request
		DiffID int `json:"diffID"`
	}

	req := request{DiffID: diffID}
	err = c.conn.CallContext(ctx, "differential.getrawdiff", &req, &diff)
	if err != nil {
		return "", err
	}

	return diff, nil
}

// DiffInfo contains information for a diff such as the author
type DiffInfo struct {
	Message     string    `json:"description"`
	AuthorName  string    `json:"authorName"`
	AuthorEmail string    `json:"authorEmail"`
	DateCreated string    `json:"dateCreated"`
	Date        time.Time `json:"omitempty"`
}

// GetDiffInfo retrieves the DiffInfo of the diff with the given id.
func (c *Client) GetDiffInfo(ctx context.Context, diffID int) (*DiffInfo, error) {
	type request struct {
		requests.Request
		IDs []int `json:"ids"`
	}

	req := request{IDs: []int{diffID}}

	var res map[string]*DiffInfo
	err := c.conn.CallContext(ctx, "differential.querydiffs", &req, &res)
	if err != nil {
		return nil, err
	}

	info, ok := res[strconv.Itoa(diffID)]
	if !ok {
		return nil, errors.Errorf("phabricator error: no diff info found for diff %d", diffID)
	}

	date, err := ParseDate(info.DateCreated)
	if err != nil {
		return nil, err
	}

	info.Date = *date

	return info, nil
}

type unixTime struct{ t *time.Time }

func (d *unixTime) UnmarshalJSON(data []byte) error {
	ts := string(data)

	// Ignore null, like in the main JSON package.
	if ts == "null" {
		return nil
	}

	t, err := ParseDate(strings.Trim(ts, `"`))
	if err != nil {
		return err
	}

	if d.t == nil {
		d.t = t
	} else {
		*d.t = *t
	}

	return nil
}

// ParseDate parses the given unix timestamp into a time.Time pointer.
func ParseDate(secStr string) (*time.Time, error) {
	seconds, err := strconv.ParseInt(secStr, 10, 64)
	if err != nil {
		return nil, errors.Wrap(err, "phabricator: could not parse date")
	}
	t := time.Unix(seconds, 0).UTC()
	return &t, nil
}
