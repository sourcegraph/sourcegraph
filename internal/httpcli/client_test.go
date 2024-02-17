package httpcli

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"testing/quick"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sourcegraph/log/logtest"

	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func TestHeadersMiddleware(t *testing.T) {
	headers := []string{"X-Foo", "bar", "X-Bar", "foo"}
	for _, tc := range []struct {
		name    string
		cli     Doer
		headers []string
		err     string
	}{
		{
			name:    "odd number of headers panics",
			headers: headers[:1],
			cli: DoerFunc(func(r *http.Request) (*http.Response, error) {
				t.Fatal("should not be called")
				return nil, nil
			}),
			err: "missing header values",
		},
		{
			name:    "even number of headers are set",
			headers: headers,
			cli: DoerFunc(func(r *http.Request) (*http.Response, error) {
				for i := 0; i < len(headers); i += 2 {
					name := headers[i]
					if have, want := r.Header.Get(name), headers[i+1]; have != want {
						t.Errorf("header %q: have: %q, want: %q", name, have, want)
					}
				}
				return nil, nil
			}),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if tc.err == "" {
				tc.err = "<nil>"
			}

			defer func() {
				if err := recover(); err != nil {
					if have, want := fmt.Sprint(err), tc.err; have != want {
						t.Fatalf("have error: %q\nwant error: %q", have, want)
					}
				}
			}()

			cli := HeadersMiddleware(tc.headers...)(tc.cli)
			req, _ := http.NewRequest("GET", "http://dev/null", nil)

			_, err := cli.Do(req)
			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Fatalf("have error: %q\nwant error: %q", have, want)
			}
		})
	}
}

func TestContextErrorMiddleware(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	for _, tc := range []struct {
		name string
		cli  Doer
		ctx  context.Context
		err  string
	}{
		{
			name: "no context error, no doer error",
			cli:  newFakeClient(http.StatusOK, nil, nil),
			err:  "<nil>",
		},
		{
			name: "no context error, with doer error",
			cli:  newFakeClient(http.StatusOK, nil, errors.New("boom")),
			err:  "boom",
		},
		{
			name: "with context error and no doer error",
			cli:  newFakeClient(http.StatusOK, nil, nil),
			ctx:  cancelled,
			err:  "<nil>",
		},
		{
			name: "with context error and doer error",
			cli:  newFakeClient(http.StatusOK, nil, errors.New("boom")),
			ctx:  cancelled,
			err:  context.Canceled.Error(),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			cli := ContextErrorMiddleware(tc.cli)

			req, _ := http.NewRequest("GET", "http://dev/null", nil)

			if tc.ctx != nil {
				req = req.WithContext(tc.ctx)
			}

			_, err := cli.Do(req)

			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Fatalf("have error: %q\nwant error: %q", have, want)
			}
		})
	}
}

func genCert(subject string) (string, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return "", err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{subject},
		},
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return "", err
	}

	var b strings.Builder
	if err := pem.Encode(&b, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return "", err
	}
	return b.String(), nil
}

func TestNewCertPool(t *testing.T) {
	subject := "newcertpooltest"
	cert, err := genCert(subject)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name   string
		certs  []string
		cli    *http.Client
		assert func(testing.TB, *http.Client)
		err    string
	}{
		{
			name:  "fails if transport isn't an http.Transport",
			cli:   &http.Client{Transport: bogusTransport{}},
			certs: []string{cert},
			err:   "httpcli.NewCertPoolOpt: http.Client.Transport cannot be cast as a *http.Transport: httpcli.bogusTransport",
		},
		{
			name:  "pool is set to what is given",
			cli:   &http.Client{Transport: &http.Transport{}},
			certs: []string{cert},
			assert: func(t testing.TB, cli *http.Client) {
				pool := cli.Transport.(*http.Transport).TLSClientConfig.RootCAs
				for _, have := range pool.Subjects() { //nolint:staticcheck // pool.Subjects, see https://github.com/golang/go/issues/46287
					if bytes.Contains(have, []byte(subject)) {
						return
					}
				}
				t.Fatal("could not find subject in pool")
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := NewCertPoolOpt(tc.certs...)(tc.cli)

			if tc.err == "" {
				tc.err = "<nil>"
			}

			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Fatalf("have error: %q\nwant error: %q", have, want)
			}

			if tc.assert != nil {
				tc.assert(t, tc.cli)
			}
		})
	}
}

func TestNewIdleConnTimeoutOpt(t *testing.T) {
	timeout := 33 * time.Second

	// originalRoundtripper must only be used in one test, set at this scope for
	// convenience.
	originalRoundtripper := &http.Transport{}

	for _, tc := range []struct {
		name    string
		cli     *http.Client
		timeout time.Duration
		assert  func(testing.TB, *http.Client)
		err     string
	}{
		{
			name: "sets default transport if nil",
			cli:  &http.Client{},
			assert: func(t testing.TB, cli *http.Client) {
				if cli.Transport == nil {
					t.Fatal("transport wasn't set")
				}
			},
		},
		{
			name: "fails if transport isn't an http.Transport",
			cli:  &http.Client{Transport: bogusTransport{}},
			err:  "httpcli.NewIdleConnTimeoutOpt: http.Client.Transport cannot be cast as a *http.Transport: httpcli.bogusTransport",
		},
		{
			name:    "IdleConnTimeout is set to what is given",
			cli:     &http.Client{Transport: &http.Transport{}},
			timeout: timeout,
			assert: func(t testing.TB, cli *http.Client) {
				have := cli.Transport.(*http.Transport).IdleConnTimeout
				if want := timeout; !reflect.DeepEqual(have, want) {
					t.Fatal(cmp.Diff(have, want))
				}
			},
		},
		{
			name: "IdleConnTimeout is set to what is given on a wrapped transport",
			cli: func() *http.Client {
				return &http.Client{Transport: WrapTransport(&actor.HTTPTransport{RoundTripper: originalRoundtripper}, originalRoundtripper)}
			}(),
			timeout: timeout,
			assert: func(t testing.TB, cli *http.Client) {
				unwrapped := unwrapAll(cli.Transport.(WrappedTransport))
				have := (*unwrapped).(*http.Transport).IdleConnTimeout

				// Timeout is set on the underlying transport
				if want := timeout; !reflect.DeepEqual(have, want) {
					t.Fatal(cmp.Diff(have, want))
				}

				// Original roundtripper unchanged!
				assert.Equal(t, time.Duration(0), originalRoundtripper.IdleConnTimeout)
			},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			err := NewIdleConnTimeoutOpt(tc.timeout)(tc.cli)

			if tc.err == "" {
				tc.err = "<nil>"
			}

			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Fatalf("have error: %q\nwant error: %q", have, want)
			}

			if tc.assert != nil {
				tc.assert(t, tc.cli)
			}
		})
	}
}

func TestNewTimeoutOpt(t *testing.T) {
	var cli http.Client

	timeout := 42 * time.Second
	err := NewTimeoutOpt(timeout)(&cli)
	if err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	if have, want := cli.Timeout, timeout; have != want {
		t.Errorf("have Timeout %s, want %s", have, want)
	}
}

func TestErrorResilience(t *testing.T) {
	failures := int64(5)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := 0
		switch n := atomic.AddInt64(&failures, -1); n {
		case 4:
			status = 429
		case 3:
			status = 500
		case 2:
			status = 900
		case 1:
			status = 302
			w.Header().Set("Location", "/")
		case 0:
			status = 404
		}
		w.WriteHeader(status)
	}))

	t.Cleanup(srv.Close)

	req, err := http.NewRequest("GET", srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("many", func(t *testing.T) {
		cli, _ := NewFactory(
			NewMiddleware(
				ContextErrorMiddleware,
			),
			NewErrorResilientTransportOpt(
				newRetryPolicy(20, time.Second),
				expJitterDelayOrRetryAfterDelay(50*time.Millisecond, 5*time.Second),
			),
		).Doer()

		res, err := cli.Do(req)
		if err != nil {
			t.Fatal(err)
		}

		if res.StatusCode != 404 {
			t.Fatalf("want status code 404, got: %d", res.StatusCode)
		}
	})

	t.Run("max", func(t *testing.T) {
		atomic.StoreInt64(&failures, 5)

		cli, _ := NewFactory(
			NewMiddleware(
				ContextErrorMiddleware,
			),
			NewErrorResilientTransportOpt(
				newRetryPolicy(0, time.Second), // zero retries
				expJitterDelayOrRetryAfterDelay(50*time.Millisecond, 5*time.Second),
			),
		).Doer()

		res, err := cli.Do(req)
		if err != nil {
			t.Fatal(err)
		}

		if res.StatusCode != 429 {
			t.Fatalf("want status code 429, got: %d", res.StatusCode)
		}
	})

	t.Run("no such host", func(t *testing.T) {
		// spy on policy so we see what decisions it makes
		retries := 0
		policy := newRetryPolicy(5, time.Second) // smaller retries for faster failures
		wrapped := func(a Attempt) bool {
			if policy(a) {
				retries++
				return true
			}
			return false
		}

		cli, _ := NewFactory(
			NewMiddleware(
				ContextErrorMiddleware,
			),
			func(cli *http.Client) error {
				// Some DNS servers do not respect RFC 6761 section 6.4, so we
				// hardcode what go returns for DNS not found to avoid
				// flakiness across machines. However, CI correctly respects
				// this so we continue to run against a real DNS server on CI.
				// TODO(burmudar): Fix DNS infrastructure in Aspect Workflows Infra
				cli.Transport = notFoundTransport{}
				return nil
			},
			NewErrorResilientTransportOpt(
				wrapped,
				expJitterDelayOrRetryAfterDelay(50*time.Millisecond, 5*time.Second),
			),
		).Doer()

		// requests to .invalid will fail DNS lookup. (RFC 6761 section 6.4)
		req, err := http.NewRequest("GET", "http://test.invalid", nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = cli.Do(req)

		var dnsErr *net.DNSError
		if !errors.As(err, &dnsErr) || !dnsErr.IsNotFound {
			t.Fatalf("expected err to be net.DNSError with IsNotFound true: %v", err)
		}

		// policy is on DNS failure to retry 3 times
		if want := 3; retries != want {
			t.Fatalf("expected %d retries, got %d", want, retries)
		}
	})
}

func TestLoggingMiddleware(t *testing.T) {
	failures := int64(3)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		status := 0
		switch n := atomic.AddInt64(&failures, -1); n {
		case 2:
			status = 500
		case 1:
			status = 302
			w.Header().Set("Location", "/")
		case 0:
			status = 404 // last
		}
		w.WriteHeader(status)
	}))

	t.Cleanup(srv.Close)

	req, err := http.NewRequest("GET", srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("log on error", func(t *testing.T) {
		logger, exportLogs := logtest.Captured(t)

		cli, _ := NewFactory(
			NewMiddleware(
				ContextErrorMiddleware,
				NewLoggingMiddleware(logger),
			),
			func(c *http.Client) error {
				c.Transport = &notFoundTransport{} // returns an error
				return nil
			},
		).Doer()

		resp, err := cli.Do(req)
		assert.Error(t, err)
		assert.Nil(t, resp)

		// Check log entries for logged fields about retries
		logEntries := exportLogs()
		require.Len(t, logEntries, 1)
		entry := logEntries[0]
		assert.Contains(t, entry.Scope, "httpcli")
		assert.NotEmpty(t, entry.Fields["error"])
	})

	t.Run("log NewRetryPolicy", func(t *testing.T) {
		logger, exportLogs := logtest.Captured(t)

		cli, _ := NewFactory(
			NewMiddleware(
				ContextErrorMiddleware,
				NewLoggingMiddleware(logger),
			),
			NewErrorResilientTransportOpt(
				newRetryPolicy(20, time.Second),
				expJitterDelayOrRetryAfterDelay(50*time.Millisecond, 5*time.Second),
			),
		).Doer()

		res, err := cli.Do(req)
		if err != nil {
			t.Fatal(err)
		}

		if res.StatusCode != 404 {
			t.Fatalf("want status code 404, got: %d", res.StatusCode)
		}

		// Check log entries for logged fields about retries
		logEntries := exportLogs()
		assert.Greater(t, len(logEntries), 0)
		var attemptsLogged int
		for _, entry := range logEntries {
			// Check for appropriate scope
			if !strings.Contains(entry.Scope, "httpcli") {
				continue
			}

			// Check for retry log fields
			retry := entry.Fields["retry"]
			if retry != nil {
				// Non-zero number of attempts only
				retryFields := retry.(map[string]any)
				assert.NotZero(t, retryFields["attempts"])

				// We must find at least some desired log entries
				attemptsLogged += 1
			}
		}
		assert.NotZero(t, attemptsLogged)
	})

	t.Run("log redisLoggerMiddleware error", func(t *testing.T) {
		const wantErrMessage = "redisLoggingError"
		redisErrorMiddleware := func(next Doer) Doer {
			return DoerFunc(func(req *http.Request) (*http.Response, error) {
				// simplified version of what we do in redisLoggerMiddleware, since
				// we just test that adding and reading the context key/value works
				var middlewareErrors error
				defer func() {
					if middlewareErrors != nil {
						*req = *req.WithContext(context.WithValue(req.Context(),
							redisLoggingMiddlewareErrorKey, middlewareErrors))
					}
				}()

				middlewareErrors = errors.New(wantErrMessage)

				return next.Do(req)
			})
		}

		logger, exportLogs := logtest.Captured(t)

		cli, _ := NewFactory(
			NewMiddleware(
				ContextErrorMiddleware,
				redisErrorMiddleware,
				NewLoggingMiddleware(logger),
			),
		).Doer()

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		t.Cleanup(srv.Close)

		req, _ := http.NewRequest("GET", srv.URL, nil)
		_, err := cli.Do(req)
		if err != nil {
			t.Fatal(err)
		}

		// Check log entries for logged fields about retries
		logEntries := exportLogs()
		assert.Greater(t, len(logEntries), 0)
		var found bool
		for _, entry := range logEntries {
			// Check for appropriate scope
			if !strings.Contains(entry.Scope, "httpcli") {
				continue
			}

			// Check for redisLoggerErr
			errField, ok := entry.Fields["redisLoggerErr"]
			if !ok {
				continue
			}
			if assert.Contains(t, errField, wantErrMessage) {
				found = true
				break
			}
		}
		assert.True(t, found)
	})
}

type notFoundTransport struct{}

func (notFoundTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, &net.DNSError{IsNotFound: true}
}

func TestExpJitterDelayOrRetryAfterDelay(t *testing.T) {
	// Ensure that at least one value is not base.
	var hasNonBase bool

	prop := func(b, m uint32, a uint16) bool {
		base := time.Duration(b)
		max := time.Duration(m)
		for max < base {
			max *= 2
		}
		attempt := int(a)

		delay := expJitterDelayOrRetryAfterDelay(base, max)(Attempt{
			Index: attempt,
		})

		t.Logf("base: %v, max: %v, attempt: %v", base, max, attempt)

		switch {
		case delay > max:
			t.Logf("delay %v > max %v", delay, max)
			return false
		case delay < base:
			t.Logf("delay %v < base %v", delay, base)
			return false
		}

		if delay > base {
			hasNonBase = true
		}

		return true
	}

	err := quick.Check(prop, nil)
	if err != nil {
		t.Fatal(err)
	}
	assert.True(t, hasNonBase, "at least one delay should be greater than base")

	t.Run("respect Retry-After header", func(t *testing.T) {
		for _, tc := range []struct {
			name            string
			base            time.Duration
			max             time.Duration
			responseHeaders http.Header
			wantDelay       time.Duration
		}{
			{
				name:            "seconds: up to max",
				max:             3 * time.Second,
				responseHeaders: http.Header{"Retry-After": []string{"20"}},
				wantDelay:       3 * time.Second,
			},
			{
				name:            "seconds: at least base",
				base:            2 * time.Second,
				max:             3 * time.Second,
				responseHeaders: http.Header{"Retry-After": []string{"1"}},
				wantDelay:       2 * time.Second,
			},
			{
				name:            "seconds: exactly as provided",
				base:            1 * time.Second,
				max:             3 * time.Second,
				responseHeaders: http.Header{"Retry-After": []string{"2"}},
				wantDelay:       2 * time.Second,
			},
		} {
			t.Run(tc.name, func(t *testing.T) {
				assert.Equal(t, tc.wantDelay, expJitterDelayOrRetryAfterDelay(tc.base, tc.max)(Attempt{
					Index: 2,
					Response: &http.Response{
						Header: tc.responseHeaders,
					},
				}))
			})
		}
	})
}

func newFakeClient(code int, body []byte, err error) Doer {
	return newFakeClientWithHeaders(map[string][]string{}, code, body, err)
}

func newFakeClientWithHeaders(respHeaders map[string][]string, code int, body []byte, err error) Doer {
	return DoerFunc(func(r *http.Request) (*http.Response, error) {
		rr := httptest.NewRecorder()
		for k, v := range respHeaders {
			rr.Header()[k] = v
		}
		_, _ = rr.Write(body)
		rr.Code = code
		return rr.Result(), err
	})
}

type bogusTransport struct{}

func (t bogusTransport) RoundTrip(*http.Request) (*http.Response, error) {
	panic("should not be called")
}

func TestRetryAfter(t *testing.T) {
	t.Run("Not set", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}))

		t.Cleanup(srv.Close)

		req, err := http.NewRequest("GET", srv.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		// spy on policy so we see what decisions it makes
		retries := 0
		policy := newRetryPolicy(5, time.Second) // smaller retries for faster failures
		wrapped := func(a Attempt) bool {
			if policy(a) {
				retries++
				return true
			}
			return false
		}

		cli, _ := NewFactory(
			NewMiddleware(
				ContextErrorMiddleware,
			),
			NewErrorResilientTransportOpt(
				wrapped,
				expJitterDelayOrRetryAfterDelay(50*time.Millisecond, 5*time.Second),
			),
		).Doer()

		res, err := cli.Do(req)
		if err != nil {
			t.Fatal(err)
		}

		if res.StatusCode != 429 {
			t.Fatalf("want status code 429, got: %d", res.StatusCode)
		}

		if want := 5; retries != want {
			t.Fatalf("expected %d retries, got %d", want, retries)
		}
	})
	t.Run("Format seconds", func(t *testing.T) {
		t.Run("Within configured limit", func(t *testing.T) {
			for _, responseCode := range []int{
				http.StatusTooManyRequests,
				http.StatusServiceUnavailable,
			} {
				t.Run(fmt.Sprintf("%d", responseCode), func(t *testing.T) {
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Add("retry-after", "1") // 1 second is smaller than the 2s we give the retry policy below.
						w.WriteHeader(responseCode)
					}))

					t.Cleanup(srv.Close)

					req, err := http.NewRequest("GET", srv.URL, nil)
					if err != nil {
						t.Fatal(err)
					}
					// spy on policy so we see what decisions it makes
					retries := 0
					policy := newRetryPolicy(5, 2*time.Second) // smaller retries for faster failures
					wrapped := func(a Attempt) bool {
						if policy(a) {
							retries++
							return true
						}
						return false
					}

					cli, _ := NewFactory(
						NewMiddleware(
							ContextErrorMiddleware,
						),
						NewErrorResilientTransportOpt(
							wrapped,
							expJitterDelayOrRetryAfterDelay(50*time.Millisecond, 5*time.Second),
						),
					).Doer()

					res, err := cli.Do(req)
					if err != nil {
						t.Fatal(err)
					}

					if res.StatusCode != responseCode {
						t.Fatalf("want status code %d, got: %d",
							responseCode, res.StatusCode)
					}

					if want := 5; retries != want {
						t.Fatalf("expected %d retries, got %d", want, retries)
					}
				})
			}
		})
		t.Run("Exceeds configured limit", func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Add("retry-after", "2") // 2 seconds is larger than the 1s we give the retry policy below.
				w.WriteHeader(http.StatusTooManyRequests)
			}))

			t.Cleanup(srv.Close)

			req, err := http.NewRequest("GET", srv.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			// spy on policy so we see what decisions it makes
			retries := 0
			policy := newRetryPolicy(5, time.Second) // smaller retries for faster failures
			wrapped := func(a Attempt) bool {
				if policy(a) {
					retries++
					return true
				}
				return false
			}

			cli, _ := NewFactory(
				NewMiddleware(
					ContextErrorMiddleware,
				),
				NewErrorResilientTransportOpt(
					wrapped,
					expJitterDelayOrRetryAfterDelay(50*time.Millisecond, 5*time.Second),
				),
			).Doer()

			res, err := cli.Do(req)
			if err != nil {
				t.Fatal(err)
			}

			if res.StatusCode != 429 {
				t.Fatalf("want status code 429, got: %d", res.StatusCode)
			}

			if want := 0; retries != want {
				t.Fatalf("expected %d retries, got %d", want, retries)
			}
		})
	})
	t.Run("Format Date", func(t *testing.T) {
		now := time.Now()
		t.Run("Within configured limit", func(t *testing.T) {
			for _, responseCode := range []int{
				http.StatusTooManyRequests,
				http.StatusServiceUnavailable,
			} {
				t.Run(fmt.Sprintf("%d", responseCode), func(t *testing.T) {
					srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						w.Header().Add("retry-after", now.Add(time.Second).Format(time.RFC1123)) // 1 second is smaller than the 2s we give the retry policy below.
						w.WriteHeader(responseCode)
					}))

					t.Cleanup(srv.Close)

					req, err := http.NewRequest("GET", srv.URL, nil)
					if err != nil {
						t.Fatal(err)
					}
					// spy on policy so we see what decisions it makes
					retries := 0
					policy := newRetryPolicy(5, 2*time.Second) // smaller retries for faster failures
					wrapped := func(a Attempt) bool {
						if policy(a) {
							retries++
							return true
						}
						return false
					}

					cli, _ := NewFactory(
						NewMiddleware(
							ContextErrorMiddleware,
						),
						NewErrorResilientTransportOpt(
							wrapped,
							expJitterDelayOrRetryAfterDelay(50*time.Millisecond, 5*time.Second),
						),
					).Doer()

					res, err := cli.Do(req)
					if err != nil {
						t.Fatal(err)
					}

					if res.StatusCode != responseCode {
						t.Fatalf("want status code %d, got: %d",
							responseCode, res.StatusCode)
					}

					if want := 5; retries != want {
						t.Fatalf("expected %d retries, got %d", want, retries)
					}
				})
			}
		})
		t.Run("Exceeds configured limit", func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Add("retry-after", now.Add(5*time.Second).Format(time.RFC1123)) // 5 seconds is larger than the 1s we give the retry policy below.
				w.WriteHeader(http.StatusTooManyRequests)
			}))

			t.Cleanup(srv.Close)

			req, err := http.NewRequest("GET", srv.URL, nil)
			if err != nil {
				t.Fatal(err)
			}
			// spy on policy so we see what decisions it makes
			retries := 0
			policy := newRetryPolicy(5, time.Second) // smaller retries for faster failures
			wrapped := func(a Attempt) bool {
				if policy(a) {
					retries++
					return true
				}
				return false
			}

			cli, _ := NewFactory(
				NewMiddleware(
					ContextErrorMiddleware,
				),
				NewErrorResilientTransportOpt(
					wrapped,
					expJitterDelayOrRetryAfterDelay(50*time.Millisecond, 5*time.Second),
				),
			).Doer()

			res, err := cli.Do(req)
			if err != nil {
				t.Fatal(err)
			}

			if res.StatusCode != 429 {
				t.Fatalf("want status code 429, got: %d", res.StatusCode)
			}

			if want := 0; retries != want {
				t.Fatalf("expected %d retries, got %d", want, retries)
			}
		})
	})
	t.Run("Invalid retry-after header", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("retry-after", "unparseable")
			w.WriteHeader(http.StatusTooManyRequests)
		}))

		t.Cleanup(srv.Close)

		req, err := http.NewRequest("GET", srv.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		// spy on policy so we see what decisions it makes
		retries := 0
		policy := newRetryPolicy(5, 2*time.Second) // smaller retries for faster failures
		wrapped := func(a Attempt) bool {
			if policy(a) {
				retries++
				return true
			}
			return false
		}

		cli, _ := NewFactory(
			NewMiddleware(
				ContextErrorMiddleware,
			),
			NewErrorResilientTransportOpt(
				wrapped,
				expJitterDelayOrRetryAfterDelay(50*time.Millisecond, 5*time.Second),
			),
		).Doer()

		res, err := cli.Do(req)
		if err != nil {
			t.Fatal(err)
		}

		if res.StatusCode != 429 {
			t.Fatalf("want status code 429, got: %d", res.StatusCode)
		}

		if want := 5; retries != want {
			t.Fatalf("expected %d retries, got %d", want, retries)
		}
	})
}
