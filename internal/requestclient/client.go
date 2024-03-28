package requestclient

import (
	"context"
	"strings"
	"sync"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/requestclient/geolocation"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type clientKey struct{}

// Client carries information about the original client of a request.
type Client struct {
	// IP identifies the IP of the client.
	IP string
	// RealIP identifies the original IP of the client, before any proxying based on
	// X-Forwarded-For or similar headers.
	RealIP string
	// ForwardedFor identifies the originating IP address of a client. It can
	// be a comma-separated list of IP addresses.
	//
	// Note: This header can be spoofed and relies on trusted clients/proxies.
	// For sourcegraph.com we use cloudflare headers to avoid spoofing.
	ForwardedFor string
	// UserAgent is value of the User-Agent header:
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/User-Agent
	UserAgent string

	// wafIPCountryCode is a ISO 3166-1 alpha-2 country code for the
	// request client as provided by a WAF (typically Cloudlfare) behind which
	// Sourcegraph is hosted.
	wafIPCountryCode string

	// countryCode and friends are lazily hydrated once by
	// (*Client).OriginCountryCode().
	countryCode      string
	countryCodeError error
	countryCodeOnce  sync.Once
}

// FromContext retrieves the client IP, if available, from context.
func FromContext(ctx context.Context) *Client {
	ip, ok := ctx.Value(clientKey{}).(*Client)
	if !ok || ip == nil {
		return nil
	}
	return ip
}

// WithClient adds client IP information to context for propagation.
func WithClient(ctx context.Context, client *Client) context.Context {
	return context.WithValue(ctx, clientKey{}, client)
}

func (c *Client) LogFields() []log.Field {
	if c == nil {
		return []log.Field{log.String("requestClient", "<nil>")}
	}

	var ccField log.Field
	if cc, err := c.OriginCountryCode(); err != nil {
		ccField = log.NamedError("requestClient.countryCode.error", err)
	} else {
		ccField = log.String("requestClient.countryCode", cc)
	}

	return []log.Field{
		log.String("requestClient.ip", c.IP),
		log.String("requestClient.forwardedFor", c.ForwardedFor),
		log.String("requestClient.userAgent", c.UserAgent),
		ccField,
	}
}

// OriginCountryCode returns a best-effort inference of the ISO 3166-1 alpha-2
// country code indicating the geolocation of the request client.
func (c *Client) OriginCountryCode() (string, error) {
	c.countryCodeOnce.Do(func() {
		c.countryCode, c.countryCodeError = inferOriginCountryCode(c)
	})
	return c.countryCode, c.countryCodeError
}

func inferOriginCountryCode(c *Client) (string, error) {
	// If we have a trusted value already, use that directly.
	if c.wafIPCountryCode != "" {
		return c.wafIPCountryCode, nil
	}

	// If we're able to infer a country code from the forwarded-for header,
	// use that. We're not too worried about spoofing here because country codes
	// are purely for reference and analytics.
	if c.ForwardedFor != "" {
		// Forwarded-for is a comma-separated list of IP addresses, we only
		// want the first one.
		ips := strings.Split(c.ForwardedFor, ",")
		if cc, err := geolocation.InferCountryCode(ips[0]); err == nil {
			return cc, nil
		}
	}

	// Otherwise, we must infer a country code from the IP address.
	cc, err := geolocation.InferCountryCode(c.IP)
	if err != nil {
		return "", errors.Wrap(err, "geolocation.InferCountryCode")
	}
	return cc, nil
}
