// Code for interfacing with Javascript and Typescript package registries such
// as npmjs.com.
package npm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/conf/reposource"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type Client interface {
	// GetPackageInfo gets a package's data from the registry, including versions.
	//
	// It is preferable to use this method instead of calling GetDependencyInfo for
	// multiple versions of a package in a loop.
	GetPackageInfo(ctx context.Context, pkg *reposource.NpmPackageName) (*PackageInfo, error)

	// GetDependencyInfo gets a dependency's data from the registry.
	GetDependencyInfo(ctx context.Context, dep *reposource.NpmVersionedPackage) (*DependencyInfo, error)

	// FetchTarball fetches the sources in .tar.gz format for a dependency.
	//
	// The caller should close the returned reader after reading.
	FetchTarball(ctx context.Context, dep *reposource.NpmVersionedPackage) (io.ReadCloser, error)
}

func FetchSources(ctx context.Context, client Client, dependency *reposource.NpmVersionedPackage) (_ io.ReadCloser, err error) {
	operations := getOperations()

	ctx, _, endObservation := operations.fetchSources.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("dependency", dependency.VersionedPackageSyntax()),
	}})
	defer endObservation(1, observation.Args{})

	return client.FetchTarball(ctx, dependency)
}

type HTTPClient struct {
	registryURL string
	cf          *httpcli.Factory
	limiter     *ratelimit.InstrumentedLimiter
	credentials string
}

var _ Client = &HTTPClient{}

func NewHTTPClient(urn string, registryURL string, credentials string, httpfactory *httpcli.Factory) *HTTPClient {
	return &HTTPClient{
		registryURL: registryURL,
		cf:          httpfactory,
		limiter:     ratelimit.NewInstrumentedLimiter(urn, ratelimit.NewGlobalRateLimiter(log.Scoped("NPMClient"), urn)),
		credentials: credentials,
	}
}

type PackageInfo struct {
	Description string                     `json:"description"`
	Versions    map[string]*DependencyInfo `json:"versions"`
}

func (client *HTTPClient) GetPackageInfo(ctx context.Context, pkg *reposource.NpmPackageName) (info *PackageInfo, err error) {
	doer, err := client.cf.Doer()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/%s", client.registryURL, pkg.PackageSyntax())
	body, err := client.makeGetRequest(ctx, doer, url)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var pkgInfo *PackageInfo
	if err := json.NewDecoder(body).Decode(&pkgInfo); err != nil {
		return nil, err
	}

	if len(pkgInfo.Versions) == 0 {
		return nil, errors.Newf("npm returned empty list of versions")
	}
	return pkgInfo, nil
}

type DependencyInfo struct {
	Description string             `json:"description"`
	Dist        DependencyInfoDist `json:"dist"`
}

type DependencyInfoDist struct {
	TarballURL string `json:"tarball"`
}

type illFormedJSONError struct {
	url string
}

func (i illFormedJSONError) Error() string {
	return fmt.Sprintf("unexpected JSON output from npm request: url=%s", i.url)
}

type npmError struct {
	statusCode int
	err        error
}

func (n npmError) Error() string {
	if 100 <= n.statusCode && n.statusCode <= 599 {
		return fmt.Sprintf("npm HTTP response %d: %s", n.statusCode, n.err.Error())
	}
	return n.err.Error()
}

func (n npmError) NotFound() bool {
	return n.statusCode == http.StatusNotFound
}

func (client *HTTPClient) makeGetRequest(ctx context.Context, doer httpcli.Doer, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	if client.credentials != "" {
		req.Header.Set("Authorization", "Bearer "+client.credentials)
	}

	do := func() (_ *http.Response, err error) {
		tr, ctx := trace.New(ctx, "npm")
		defer tr.EndWithErr(&err)
		req = req.WithContext(ctx)

		if err := client.limiter.Wait(ctx); err != nil {
			return nil, err
		}
		return doer.Do(req)
	}

	resp, err := do()
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()

		bs, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, npmError{resp.StatusCode, errors.Newf("failed to read non-200 body: %s", bs)}
		}
		return nil, npmError{resp.StatusCode, errors.New(string(bs))}
	}

	return resp.Body, nil
}

func (client *HTTPClient) GetDependencyInfo(ctx context.Context, dep *reposource.NpmVersionedPackage) (*DependencyInfo, error) {
	doer, err := client.cf.Doer(httpcli.CachedTransportOpt)
	if err != nil {
		return nil, err
	}

	// https://github.com/npm/registry/blob/master/docs/REGISTRY-API.md#getVersionedPackage
	url := fmt.Sprintf("%s/%s/%s", client.registryURL, dep.PackageSyntax(), dep.Version)
	body, err := client.makeGetRequest(ctx, doer, url)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	var info DependencyInfo
	if json.NewDecoder(body).Decode(&info) != nil {
		return nil, illFormedJSONError{url: url}
	}

	return &info, nil
}

func (client *HTTPClient) FetchTarball(ctx context.Context, dep *reposource.NpmVersionedPackage) (io.ReadCloser, error) {
	if dep.TarballURL == "" {
		return nil, errors.New("empty TarballURL")
	}

	doer, err := client.cf.Doer()
	if err != nil {
		return nil, err
	}

	return client.makeGetRequest(ctx, doer, dep.TarballURL)
}
