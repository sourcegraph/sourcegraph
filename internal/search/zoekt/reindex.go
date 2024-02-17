package zoekt

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/search"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// Reindex forces indexserver to reindex the repo immediately.
func Reindex(ctx context.Context, name api.RepoName, id api.RepoID) error {
	u, err := resolveIndexserver(name)
	if err != nil {
		return err
	}

	form := url.Values{}
	form.Add("repo", strconv.Itoa(int(id)))

	u = u.ResolveReference(&url.URL{Path: "/indexserver/debug/reindex"})

	req, err := http.NewRequestWithContext(ctx, "POST", u.String(), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	cli, err := httpcli.NewInternalClientFactory("zoekt").Doer()
	if err != nil {
		return err
	}

	resp, err := cli.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusAccepted:
		return nil
	case http.StatusBadGateway:
		return errors.New("Invalid response from Zoekt indexserver. The most likely cause is a broken socket connection.")
	default:
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return errors.Newf("%s: %q", resp.Status, string(b))
	}
}

type Host struct {
	Name string `json:"hostname"`
}

func GetIndexserverHost(ctx context.Context, name api.RepoName) (Host, error) {
	u, err := resolveIndexserver(name)
	if err != nil {
		return Host{}, err
	}
	u = u.ResolveReference(&url.URL{Path: "/indexserver/debug/host"})

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return Host{}, err
	}

	cli, err := httpcli.NewInternalClientFactory("zoekt").Doer()
	if err != nil {
		return Host{}, err
	}

	resp, err := cli.Do(req)
	if err != nil {
		return Host{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Host{}, errors.Newf("webserver responded with %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return Host{}, err
	}

	h := Host{}
	err = json.Unmarshal(b, &h)
	if err != nil {
		return Host{}, err
	}

	return h, nil
}

// resolveIndexserver returns the Zoekt webserver hosting the index of the repo.
func resolveIndexserver(name api.RepoName) (*url.URL, error) {
	ep, err := search.Indexers().Map.Get(string(name))
	if err != nil {
		return nil, err
	}

	// We add http:// on a best-effort basis, because it is not guaranteed that
	// ep is a valid URL.
	if !strings.HasPrefix(ep, "http://") {
		ep = "http://" + ep
	}

	return url.Parse(ep)
}
