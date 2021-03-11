package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	frontendregistry "github.com/sourcegraph/sourcegraph/cmd/frontend/registry/api"
	registry "github.com/sourcegraph/sourcegraph/cmd/frontend/registry/client"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/schema"
)

func init() {
	frontendregistry.HandleRegistry = handleRegistry
}

// Funcs called by serveRegistry to get registry data. If fakeRegistryData is set, it is used as
// the data source instead of the database.
var (
	registryList = func(ctx context.Context, opt dbExtensionsListOptions) ([]*registry.Extension, error) {
		vs, err := dbExtensions{}.List(ctx, opt)
		if err != nil {
			return nil, err
		}

		xs, err := toRegistryAPIExtensionBatch(ctx, vs)
		if err != nil {
			return nil, err
		}

		ys := make([]*registry.Extension, 0, len(xs))
		for _, x := range xs {
			// To be safe, ensure that the JSON can be safely unmarshaled by API clients. If not,
			// skip this extension.
			if x.Manifest != nil {
				var o schema.SourcegraphExtensionManifest
				if err := jsonc.Unmarshal(*x.Manifest, &o); err != nil {
					continue
				}
			}
			ys = append(ys, x)
		}
		return ys, nil
	}

	registryGetByUUID = func(ctx context.Context, uuid string) (*registry.Extension, error) {
		x, err := dbExtensions{}.GetByUUID(ctx, uuid)
		if err != nil {
			return nil, err
		}
		return toRegistryAPIExtension(ctx, x)
	}

	registryGetByExtensionID = func(ctx context.Context, extensionID string) (*registry.Extension, error) {
		x, err := dbExtensions{}.GetByExtensionID(ctx, extensionID)
		if err != nil {
			return nil, err
		}
		return toRegistryAPIExtension(ctx, x)
	}
)

func toRegistryAPIExtension(ctx context.Context, v *dbExtension) (*registry.Extension, error) {
	release, err := getLatestRelease(ctx, v.NonCanonicalExtensionID, v.ID, "release")
	if err != nil {
		return nil, err
	}

	if release == nil {
		return newExtension(v, nil, time.Time{}), nil
	}

	return newExtension(v, &release.Manifest, release.CreatedAt), nil
}

func toRegistryAPIExtensionBatch(ctx context.Context, vs []*dbExtension) ([]*registry.Extension, error) {
	releasesByExtensionID, err := getLatestForBatch(ctx, vs)
	if err != nil {
		return nil, err
	}

	var extensions []*registry.Extension
	for _, v := range vs {
		release, ok := releasesByExtensionID[v.ID]
		if !ok {
			extensions = append(extensions, newExtension(v, nil, time.Time{}))
		} else {
			extensions = append(extensions, newExtension(v, &release.Manifest, release.CreatedAt))
		}
	}
	return extensions, nil
}

func newExtension(v *dbExtension, manifest *string, publishedAt time.Time) *registry.Extension {
	baseURL := strings.TrimSuffix(conf.Get().ExternalURL, "/")
	return &registry.Extension{
		UUID:        v.UUID,
		ExtensionID: v.NonCanonicalExtensionID,
		Publisher: registry.Publisher{
			Name: v.Publisher.NonCanonicalName,
			URL:  baseURL + frontendregistry.PublisherExtensionsURL(v.Publisher.UserID != 0, v.Publisher.OrgID != 0, v.Publisher.NonCanonicalName),
		},
		Name:        v.Name,
		Manifest:    manifest,
		CreatedAt:   v.CreatedAt,
		UpdatedAt:   v.UpdatedAt,
		PublishedAt: publishedAt,
		URL:         baseURL + frontendregistry.ExtensionURL(v.NonCanonicalExtensionID),
	}
}

type responseRecorder struct {
	http.ResponseWriter
	code int
}

func (r *responseRecorder) WriteHeader(code int) {
	r.code = code
	r.ResponseWriter.WriteHeader(code)
}

// handleRegistry serves the external HTTP API for the extension registry.
func handleRegistry(w http.ResponseWriter, r *http.Request) (err error) {
	recorder := &responseRecorder{ResponseWriter: w, code: http.StatusOK}
	w = recorder

	var operation string
	defer func(began time.Time) {
		seconds := time.Since(began).Seconds()
		if err != nil && recorder.code == http.StatusOK {
			recorder.code = http.StatusInternalServerError
		}
		code := strconv.Itoa(recorder.code)
		registryRequestsDuration.WithLabelValues(operation, code).Observe(seconds)
	}(time.Now())

	if conf.Extensions() == nil {
		w.WriteHeader(http.StatusNotFound)
		return nil
	}

	// Identify this response as coming from the registry API.
	w.Header().Set(registry.MediaTypeHeaderName, registry.MediaType)
	w.Header().Set("Vary", registry.MediaTypeHeaderName)

	// Validate API version.
	if v := r.Header.Get("Accept"); v != registry.AcceptHeader {
		http.Error(w, fmt.Sprintf("invalid Accept header: expected %q", registry.AcceptHeader), http.StatusBadRequest)
		return nil
	}

	// This handler can be mounted at either /.internal or /.api.
	urlPath := r.URL.Path
	switch {
	case strings.HasPrefix(urlPath, "/.internal"):
		urlPath = strings.TrimPrefix(urlPath, "/.internal")
	case strings.HasPrefix(urlPath, "/.api"):
		urlPath = strings.TrimPrefix(urlPath, "/.api")
	}

	const extensionsPath = "/registry/extensions"
	var result interface{}
	switch {
	case urlPath == extensionsPath:
		operation = "list"

		query := r.URL.Query().Get("q")
		var opt dbExtensionsListOptions
		opt.Query, opt.Category, opt.Tag = parseExtensionQuery(query)
		xs, err := registryList(r.Context(), opt)
		if err != nil {
			return err
		}
		result = xs

	case strings.HasPrefix(urlPath, extensionsPath+"/"):
		var (
			spec = strings.TrimPrefix(urlPath, extensionsPath+"/")
			x    *registry.Extension
			err  error
		)
		switch {
		case strings.HasPrefix(spec, "uuid/"):
			operation = "get-by-uuid"
			x, err = registryGetByUUID(r.Context(), strings.TrimPrefix(spec, "uuid/"))
		case strings.HasPrefix(spec, "extension-id/"):
			operation = "get-by-extension-id"
			x, err = registryGetByExtensionID(r.Context(), strings.TrimPrefix(spec, "extension-id/"))
		default:
			w.WriteHeader(http.StatusNotFound)
			return nil
		}
		if x == nil || err != nil {
			if x == nil || errcode.IsNotFound(err) {
				w.Header().Set("Cache-Control", "max-age=5, private")
				http.Error(w, "extension not found", http.StatusNotFound)
				return nil
			}
			return err
		}
		result = x

	default:
		w.WriteHeader(http.StatusNotFound)
		return nil
	}

	w.Header().Set("Cache-Control", "max-age=30, private")
	return json.NewEncoder(w).Encode(result)
}

var (
	registryRequestsDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name: "src_registry_requests_duration_seconds",
		Help: "Seconds spent handling a request to the HTTP registry API",
	}, []string{"operation", "code"})
)

func init() {
	// Allow providing fake registry data for local dev (intended for use in local dev only).
	//
	// If FAKE_REGISTRY is set and refers to a valid JSON file (of []*registry.Extension), is used
	// by serveRegistry (instead of the DB) as the source for registry data.
	path := os.Getenv("FAKE_REGISTRY")
	if path == "" {
		return
	}

	readFakeExtensions := func() ([]*registry.Extension, error) {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var xs []*registry.Extension
		if err := json.Unmarshal(data, &xs); err != nil {
			return nil, err
		}
		return xs, nil
	}

	registryList = func(ctx context.Context, opt dbExtensionsListOptions) ([]*registry.Extension, error) {
		xs, err := readFakeExtensions()
		if err != nil {
			return nil, err
		}
		return frontendregistry.FilterRegistryExtensions(xs, opt.Query), nil
	}
	registryGetByUUID = func(ctx context.Context, uuid string) (*registry.Extension, error) {
		xs, err := readFakeExtensions()
		if err != nil {
			return nil, err
		}
		return frontendregistry.FindRegistryExtension(xs, "uuid", uuid), nil
	}
	registryGetByExtensionID = func(ctx context.Context, extensionID string) (*registry.Extension, error) {
		xs, err := readFakeExtensions()
		if err != nil {
			return nil, err
		}
		return frontendregistry.FindRegistryExtension(xs, "extensionID", extensionID), nil
	}
}
