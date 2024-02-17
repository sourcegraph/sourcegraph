package httpcli

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"reflect"
	"sync"
	"sync/atomic"

	"code.gitea.io/gitea/modules/hostmatcher"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

// ExternalTransportOpt returns an Opt that ensures the http.Client.Transport
// can contact non-Sourcegraph services. For example Admins can configure
// TLS/SSL settings. This adds filtering for external requests based on
// predefined deny lists. Can be extended using the EXTERNAL_DENY_LIST
// environment variable.
func ExternalTransportOpt(cli *http.Client) error {
	tr, err := getTransportForMutation(cli)
	if err != nil {
		return errors.Wrap(err, "httpcli.ExternalTransportOpt")
	}

	var denyMatchList = hostmatcher.ParseHostMatchList("EXTERNAL_DENY_LIST", externalDenyList)

	denyList := defaultDenylist
	if env.InsecureDev {
		denyList = localDevDenylist
	}

	for _, rule := range denyList {
		if rule.builtin != "" {
			denyMatchList.AppendBuiltin(rule.builtin)
		} else if rule.pattern != "" {
			denyMatchList.AppendPattern(rule.pattern)
		}
	}

	// this dialer will match resolved domain names against the deny list
	tr.DialContext = hostmatcher.NewDialContext("", nil, denyMatchList)
	cli.Transport = WrapTransport(&externalTransport{base: tr}, tr)
	return nil
}

var externalDenyList = env.Get("EXTERNAL_DENY_LIST", "", "Deny list for outgoing requests")

type denyRule struct {
	pattern string
	builtin string
}

var defaultDenylist = []denyRule{
	{builtin: "loopback"},
	{pattern: "169.254.169.254"},
}

var localDevDenylist = []denyRule{
	{pattern: "169.254.169.254"},
}

type externalTransport struct {
	base      *http.Transport
	mu        sync.RWMutex
	config    *schema.TlsExternal
	effective *http.Transport
}

var tlsExternalConfig struct {
	sync.RWMutex
	*schema.TlsExternal
}

var outboundRequestLogLimit atomic.Int32
var redactOutboundRequestHeaders atomic.Bool

// SetTLSExternalConfig is called by the conf package whenever TLSExternalConfig changes.
// This is needed to avoid circular imports.
func SetTLSExternalConfig(c *schema.TlsExternal) {
	tlsExternalConfig.Lock()
	tlsExternalConfig.TlsExternal = c
	tlsExternalConfig.Unlock()
}

// TLSExternalConfig returns the current value of the global TLS external config.
func TLSExternalConfig() *schema.TlsExternal {
	tlsExternalConfig.RLock()
	defer tlsExternalConfig.RUnlock()
	return tlsExternalConfig.TlsExternal
}

// SetOutboundRequestLogLimit is called by the conf package whenever OutboundRequestLogLimit changes.
// This is needed to avoid circular imports.
func SetOutboundRequestLogLimit(i int32) {
	outboundRequestLogLimit.Store(i)
}

// OutboundRequestLogLimit returns the current value of the global OutboundRequestLogLimit value.
func OutboundRequestLogLimit() int32 {
	return outboundRequestLogLimit.Load()
}

// SetRedactOutboundRequestHeaders is called by the conf package whenever the RedactOutboundRequestHeaders setting changes.
// This is needed to avoid circular imports.
func SetRedactOutboundRequestHeaders(b bool) {
	redactOutboundRequestHeaders.Store(b)
}

// RedactOutboundRequestHeaders returns the current value of the global RedactOutboundRequestHeaders setting.
func RedactOutboundRequestHeaders() bool {
	return redactOutboundRequestHeaders.Load()
}

func (t *externalTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	t.mu.RLock()
	config, effective := t.config, t.effective
	t.mu.RUnlock()

	if current := TLSExternalConfig(); current == nil {
		return t.base.RoundTrip(r)
	} else if !reflect.DeepEqual(config, current) {
		effective = t.update(r.Context(), current)
	}

	return effective.RoundTrip(r)
}

func (t *externalTransport) update(ctx context.Context, config *schema.TlsExternal) *http.Transport {
	// No function calls here use the context further
	tr, _ := trace.New(ctx, "externalTransport.update")
	defer tr.End()

	t.mu.Lock()
	defer t.mu.Unlock()

	effective := t.base.Clone()

	if effective.TLSClientConfig == nil {
		effective.TLSClientConfig = new(tls.Config)
	}

	effective.TLSClientConfig.InsecureSkipVerify = config.InsecureSkipVerify

	for _, cert := range config.Certificates {
		// There is no exposed Clone function for CertPools. So if a certificate
		// is removed it will continue to be accepted since we are mutating base's
		// RootCAs. This is an acceptable tradeoff since it would be quite tricky
		// to avoid this.
		if effective.TLSClientConfig.RootCAs == nil {
			pool, err := x509.SystemCertPool() // safe to mutate, a clone is returned
			if err != nil {
				tr.AddEvent("failed to load SystemCertPool",
					trace.Error(err),
					attribute.String("warning", "communication with external HTTPS APIs may fail"))

				pool = x509.NewCertPool()
			}
			effective.TLSClientConfig.RootCAs = pool
		}
		// TODO(keegancsmith) ensure we validate these certs somewhere
		effective.TLSClientConfig.RootCAs.AppendCertsFromPEM([]byte(cert))
	}

	t.config = config
	t.effective = effective
	return effective
}
