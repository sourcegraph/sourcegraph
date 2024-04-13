package internalapi

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"
	"sync"

	"github.com/sourcegraph/log"

	proto "github.com/sourcegraph/sourcegraph/internal/api/internalapi/v1"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
)

var frontendInternal = func() *url.URL {
	rawURL := env.Get("SRC_FRONTEND_INTERNAL", "sourcegraph-frontend-internal", "HTTP address for internal frontend HTTP API.")
	u := mustParseSourcegraphInternalURL(rawURL)
	println(u.String())
	return u
}()

type internalClient struct {
	// URL is the root to the internal API frontend server.
	URL string

	getConfClient func() (proto.ConfigServiceClient, error)
}

var Client = &internalClient{
	URL: frontendInternal.String(),
	getConfClient: sync.OnceValues(func() (proto.ConfigServiceClient, error) {
		logger := log.Scoped("internalapi")
		conn, err := defaults.Dial(frontendInternal.Host, logger)
		if err != nil {
			return nil, err
		}

		client := &automaticRetryClient{base: proto.NewConfigServiceClient(conn)}
		return client, nil
	}),
}

// MockClientConfiguration mocks (*internalClient).Configuration.
var MockClientConfiguration func() (conftypes.RawUnified, error)

func (c *internalClient) Configuration(ctx context.Context) (conftypes.RawUnified, error) {
	if MockClientConfiguration != nil {
		return MockClientConfiguration()
	}

	cc, err := c.getConfClient()
	if err != nil {
		return conftypes.RawUnified{}, err
	}
	resp, err := cc.GetConfig(ctx, &proto.GetConfigRequest{})
	if err != nil {
		return conftypes.RawUnified{}, err
	}
	var raw conftypes.RawUnified
	raw.FromProto(resp.RawUnified)
	return raw, nil
}

// mustParseSourcegraphInternalURL parses a frontend internal URL string and panics if it is invalid.
//
// The URL will be parsed with a default scheme of "http" and a default port of "80" if no scheme or port is specified.
func mustParseSourcegraphInternalURL(rawURL string) *url.URL {
	u, err := parseAddress(rawURL)
	if err != nil {
		panic(fmt.Sprintf("failed to parse frontend internal URL %q: %s", rawURL, err))
	}

	u = addDefaultScheme(u, "http")
	u = addDefaultPort(u)

	return u
}

// parseAddress parses rawAddress into a URL object. It accommodates cases where the rawAddress is a
// simple host:port pair without a URL scheme (e.g., "example.com:8080").
//
// This function aims to provide a flexible way to parse addresses that may or may not strictly adhere to the URL format.
func parseAddress(rawAddress string) (*url.URL, error) {
	addedScheme := false

	// Temporarily prepend "http://" if no scheme is present
	if !strings.Contains(rawAddress, "://") {
		rawAddress = "http://" + rawAddress
		addedScheme = true
	}

	parsedURL, err := url.Parse(rawAddress)
	if err != nil {
		return nil, err
	}

	// If we added the "http://" scheme, remove it from the final URL
	if addedScheme {
		parsedURL.Scheme = ""
	}

	return parsedURL, nil
}

// addDefaultScheme adds a default scheme to a URL if one is not specified.
//
// The original URL is not mutated. A copy is modified and returned.
func addDefaultScheme(original *url.URL, scheme string) *url.URL {
	if original == nil {
		return nil // don't panic
	}

	if original.Scheme != "" {
		return original
	}

	u := cloneURL(original)
	u.Scheme = scheme

	return u
}

// addDefaultPort adds a default port to a URL if one is not specified.
//
// If the URL scheme is "http" and no port is specified, "80" is used.
// If the scheme is "https", "443" is used.
//
// The original URL is not mutated. A copy is modified and returned.
func addDefaultPort(original *url.URL) *url.URL {
	if original == nil {
		return nil // don't panic
	}

	if original.Scheme == "http" && original.Port() == "" {
		u := cloneURL(original)
		u.Host = net.JoinHostPort(u.Host, "80")
		return u
	}

	if original.Scheme == "https" && original.Port() == "" {
		u := cloneURL(original)
		u.Host = net.JoinHostPort(u.Host, "443")
		return u
	}

	return original
}

// cloneURL returns a copy of the URL. It is safe to mutate the returned URL.
// This is copied from net/http/clone.go
func cloneURL(u *url.URL) *url.URL {
	if u == nil {
		return nil
	}
	u2 := new(url.URL)
	*u2 = *u
	if u.User != nil {
		u2.User = new(url.Userinfo)
		*u2.User = *u.User
	}
	return u2
}
