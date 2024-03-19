package images

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// GCR provides access to Google Cloud Registry API.
type GCR struct {
	token string
	host  string
	org   string
	cache repositoryCache
}

// NewGCR creates a new GCR API client.
func NewGCR(host, org string) *GCR {
	return &GCR{
		org:   org,
		host:  host,
		cache: repositoryCache{},
	}
}

func (r *GCR) Host() string {
	return r.host
}

func (r *GCR) Org() string {
	return r.org
}

// Public returns if the registry is used for public purposes or not.
// Right now, we never use GCR for public releases, so it's always false.
func (r *GCR) Public() bool {
	return false
}

// LoadToken gets the access-token to reach GCR through the environment.
func (r *GCR) LoadToken() error {
	b, err := exec.Command("gcloud", "auth", "print-access-token").Output()
	if err != nil {
		return err
	}
	r.token = strings.TrimSpace(string(b))
	return nil
}

// fetchDigest returns the digest for a given container repository.
func (r *GCR) fetchDigest(repo string, tag string) (digest.Digest, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf(fetchDigestRoute, r.host, r.org+"/"+repo, tag), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", r.token))
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return "", errors.Newf("GCR fetchDigest (%s) %s:%s, got %v: %s", r.host, repo, tag, resp.Status, string(data))
	}
	d := resp.Header.Get("Docker-Content-Digest")
	g, err := digest.Parse(d)
	if err != nil {
		return "", err
	}
	return g, nil

}

// GetByTag returns a container repository, on that registry, for a given service at
// a given tag.
func (r *GCR) GetByTag(name string, tag string) (*Repository, error) {
	if repo, ok := r.cache[cacheKey{name, tag}]; ok {
		return repo, nil
	}
	digest, err := r.fetchDigest(name, tag)
	if err != nil {
		return nil, err
	}
	repo := &Repository{
		registry: r.host,
		name:     name,
		org:      r.org,
		tag:      tag,
		digest:   digest,
	}
	r.cache[cacheKey{name, tag}] = repo
	return repo, err
}

// GetLatest returns the latest container repository on that registry, according
// to the given predicate.
func (r *GCR) GetLatest(name string, latest func([]string) (string, error)) (*Repository, error) {
	if repo, ok := r.cache[cacheKey{name, ""}]; ok {
		return repo, nil
	}
	req, err := http.NewRequest("GET", fmt.Sprintf(listTagRoute, r.host, r.org+"/"+name), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", r.token))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return nil, errors.New(resp.Status + ": " + string(data))
	}
	result := struct {
		Tags []string
	}{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}
	tag, err := latest(result.Tags)
	if err != nil && tag == "" {
		return nil, err
	}

	digest, err := r.fetchDigest(name, tag)
	if err != nil {
		return nil, err
	}
	repo := &Repository{
		registry: r.host,
		name:     name,
		org:      r.org,
		tag:      tag,
		digest:   digest,
	}
	r.cache[cacheKey{name, ""}] = repo
	return repo, err
}
