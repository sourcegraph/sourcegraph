package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/google/uuid"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/internal/httpcli"
)

const (
	// APIVersion is a string that uniquely identifies this API version.
	APIVersion = "20180621"

	// AcceptHeader is the value of the "Accept" HTTP request header sent by the client.
	AcceptHeader = "application/vnd.sourcegraph.api+json;version=" + APIVersion

	// MediaTypeHeaderName is the name of the HTTP response header whose value the client expects to
	// equal MediaType.
	MediaTypeHeaderName = "X-Sourcegraph-Media-Type"

	// MediaType is the client's expected value for the MediaTypeHeaderName HTTP response header.
	MediaType = "sourcegraph.v" + APIVersion + "; format=json"
)

// List lists extensions on the remote registry matching the query (or all if the query is empty).
func List(ctx context.Context, registry *url.URL, query string) ([]*Extension, error) {
	var q url.Values
	if query != "" {
		q = url.Values{"q": []string{query}}
	}

	var xs []*Extension
	err := httpGet(ctx, "registry.List", toURL(registry, "extensions", q), &xs)
	return xs, err
}

// GetByUUID gets the extension from the remote registry with the given UUID. If the remote registry reports
// that the extension is not found, the returned error implements errcode.NotFounder.
func GetByUUID(ctx context.Context, registry *url.URL, uuidStr string) (*Extension, error) {
	// Loosely validate the UUID here, to avoid potential security risks if it contains a ".." path
	// component. Note that this does not normalize the UUID; for example, a UUID with prefix
	// "urn:uuid:" would be accepted (and it would be harmless and result in a not-found error).
	if _, err := uuid.Parse(uuidStr); err != nil {
		return nil, err
	}
	return getBy(ctx, registry, "registry.GetByUUID", "uuid", uuidStr)
}

// GetByExtensionID gets the extension from the remote registry with the given extension ID. If the
// remote registry reports that the extension is not found, the returned error implements
// errcode.NotFounder.
func GetByExtensionID(ctx context.Context, registry *url.URL, extensionID string) (*Extension, error) {
	return getBy(ctx, registry, "registry.GetByExtensionID", "extension-id", extensionID)
}

func getBy(ctx context.Context, registry *url.URL, op, field, value string) (*Extension, error) {
	var x *Extension
	if err := httpGet(ctx, op, toURL(registry, path.Join("extensions", field, value), nil), &x); err != nil {
		if e, ok := err.(*url.Error); ok && e.Err == httpError(http.StatusNotFound) {
			err = &notFoundError{field: field, value: value}
		}
		return nil, err
	}
	return x, nil
}

type notFoundError struct{ field, value string }

func (notFoundError) NotFound() bool { return true }
func (e *notFoundError) Error() string {
	return fmt.Sprintf("extension not found with %s %q", e.field, e.value)
}

func httpGet(ctx context.Context, op, urlStr string, result interface{}) (err error) {
	defer func() { err = errors.Wrap(err, remoteRegistryErrorMessage) }()

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", AcceptHeader)
	req.Header.Set("User-Agent", "Sourcegraph registry client v"+APIVersion)
	req = req.WithContext(ctx)

	resp, err := httpcli.ExternalDoer().Do(req)
	if err != nil {
		return err
	}
	if v := strings.TrimSpace(resp.Header.Get(MediaTypeHeaderName)); v != MediaType {
		return &url.Error{Op: op, URL: urlStr, Err: fmt.Errorf("not a valid Sourcegraph registry (invalid media type %q, expected %q)", v, MediaType)}
	}
	if resp.StatusCode != http.StatusOK {
		return &url.Error{Op: op, URL: urlStr, Err: httpError(resp.StatusCode)}
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return &url.Error{Op: op, URL: urlStr, Err: err}
	}
	return nil
}

const remoteRegistryErrorMessage = "unable to contact extension registry"

// IsRemoteRegistryError reports whether the err is (likely) from this package's interaction with
// the remote registry.
func IsRemoteRegistryError(err error) bool {
	return err != nil && strings.Contains(err.Error(), remoteRegistryErrorMessage)
}

type httpError int

func (e httpError) Error() string { return fmt.Sprintf("HTTP error %d", e) }

// Name returns the registry name given its URL.
func Name(registry *url.URL) string {
	return registry.Host
}

func toURL(registry *url.URL, pathStr string, query url.Values) string {
	return registry.ResolveReference(&url.URL{Path: path.Join(registry.Path, pathStr), RawQuery: query.Encode()}).String()
}
