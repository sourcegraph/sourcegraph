package images

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/docker/docker-credential-helpers/credentials"
	"github.com/opencontainers/go-digest"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/docker"
	"github.com/sourcegraph/sourcegraph/dev/sg/internal/std"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// DockerHub provides access to DockerHub Registry API.
//
// The main difference with GCR, and what causes to have a different implementation,
// is that DockerHub access tokens are unique to a given container repository, whereas
// GCR access tokens operates at the org level.
type DockerHub struct {
	username string
	password string
	host     string
	org      string
	cache    repositoryCache
	once     sync.Once
}

// NewDockerHub creates a new DockerHub API client.
func NewDockerHub(org, username, password string) *DockerHub {
	d := &DockerHub{
		username: username,
		password: password,
		host:     "index.docker.io",
		org:      org,
		cache:    repositoryCache{},
	}

	return d
}

func (r *DockerHub) Host() string {
	return r.host
}

func (r *DockerHub) Org() string {
	return r.org
}

// Public returns if the registry is used for public purposes or not.
// Right now, we only use DockerHub for public releases, so it's always true.
func (r *DockerHub) Public() bool {
	return true
}

func (r *DockerHub) tryLoadingCredsFromStore() error {
	dockerCredentials := &credentials.Credentials{
		ServerURL: "https://registry.hub.docker.com/v2",
	}

	creds, err := docker.GetCredentialsFromStore(dockerCredentials.ServerURL)
	if err != nil {
		std.Out.WriteWarningf("Registry credentials are not provided and could not be retrieved from docker config.")
		std.Out.WriteWarningf("You will be using anonymous requests and may be subject to rate limiting by Docker Hub.")
		return errors.Wrapf(err, "cannot load docker creds from store")
	}

	r.username = creds.Username
	r.password = creds.Secret
	return nil
}

// loadToken gets the access-token to reach DockerHub for that specific repo,
// by performing a basic auth first.
func (r *DockerHub) loadToken(repo string) (string, error) {
	var err error
	r.once.Do(func() {
		if r.username == "" || r.password == "" {
			err = r.tryLoadingCredsFromStore()
		}
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s/%s:pull", r.org, repo), nil)
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(r.username, r.password)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return "", errors.New(resp.Status + ": " + string(data))
	}

	result := struct {
		AccessToken string `json:"access_token"`
	}{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", err
	}
	token := strings.TrimSpace(result.AccessToken)
	return token, nil
}

// fetchDigest returns the digest for a given container repository.
func (r *DockerHub) fetchDigest(repo string, tag string) (digest.Digest, error) {
	token, err := r.loadToken(repo)
	if err != nil {
		return "", err
	}
	url := fmt.Sprintf(fetchDigestRoute, r.host, r.org+"/"+repo, tag)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("Accept", "application/vnd.docker.distribution.manifest.v2+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return "", errors.Newf("DockerHub fetchDigest (%s) %s:%s, got %v: %s", r.host, repo, tag, resp.Status, string(data))
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
func (r *DockerHub) GetByTag(name string, tag string) (*Repository, error) {
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
func (r *DockerHub) GetLatest(name string, latest func([]string) (string, error)) (*Repository, error) {
	if repo, ok := r.cache[cacheKey{name, ""}]; ok {
		return repo, nil
	}
	token, err := r.loadToken(name)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", fmt.Sprintf(listTagRoute, r.host, r.org+"/"+name), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
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
	if len(result.Tags) == 0 {
		return nil, errors.Newf("not tags found for %s", name)
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
