package httpcli

import (
	"bytes"
	"context"
	"crypto/x509"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/testutil"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// NewErrorResilientTransportOpt returns an Opt that wraps an existing
// http.Transport of an http.Client with automatic retries.
func NewErrorResilientTransportOpt(retry retryFn, delay delayFn) Opt {
	return func(cli *http.Client) error {
		if cli.Transport == nil {
			cli.Transport = http.DefaultTransport
		}

		cli.Transport = WrapTransport(NewRetryingTransport(cli.Transport, retry, delay), cli.Transport)
		return nil
	}
}

// maxRetries returns the max retries to be attempted, which should be passed
// to NewRetryPolicy. If we're in tests, it returns 0, otherwise n.
func maxRetries(n int) int {
	if testutil.IsTest {
		return 0
	}
	return n
}

// newRetryPolicy returns a retry policy based on some Sourcegraph defaults.
func newRetryPolicy(max int, maxRetryAfterDuration time.Duration) retryFn {
	// Indicates in trace whether or not this request was retried at some point
	const retriedTraceAttributeKey = "httpcli.retried"

	return func(a Attempt) (retry bool) {
		tr := trace.FromContext(a.Request.Context())
		if a.Index == 0 {
			// For the initial attempt set it to false in case we never retry,
			// to make this easier to query in Cloud Trace. This attribute will
			// get overwritten later if a retry occurs.
			tr.SetAttributes(
				attribute.Bool(retriedTraceAttributeKey, false))
		}

		status := 0
		var retryAfterHeader string

		defer func() {
			// Avoid trace log spam if we haven't invoked the retry policy.
			shouldTraceLog := retry || a.Index > 0
			if tr.IsRecording() && shouldTraceLog {
				fields := []attribute.KeyValue{
					attribute.Bool("retry", retry),
					attribute.Int("attempt", a.Index),
					attribute.String("method", a.Request.Method),
					attribute.Stringer("url", a.Request.URL),
					attribute.Int("status", status),
					attribute.String("retry-after", retryAfterHeader),
				}
				if a.Error != nil {
					fields = append(fields, trace.Error(a.Error))
				}
				tr.AddEvent("request-retry-decision", fields...)
				// Record on span itself as well for ease of querying, updates
				// will overwrite previous values.
				tr.SetAttributes(
					attribute.Bool(retriedTraceAttributeKey, true),
					attribute.Int("httpcli.retriedAttempts", a.Index))
			}

			// Update request context with latest retry for logging middleware
			if shouldTraceLog {
				*a.Request = *a.Request.WithContext(
					context.WithValue(a.Request.Context(), requestRetryAttemptKey, a))
			}

			if retry {
				metricRetry.Inc()
			}
		}()

		if a.Response != nil {
			status = a.Response.StatusCode
		}

		if a.Index >= max { // Max retries
			return false
		}

		switch a.Error {
		case nil:
		case context.DeadlineExceeded, context.Canceled:
			return false
		default:
			// Don't retry more than 3 times for no such host errors.
			// This affords some resilience to dns unreliability while
			// preventing 20 attempts with a non existing name.
			var dnsErr *net.DNSError
			if a.Index >= 3 && errors.As(a.Error, &dnsErr) && dnsErr.IsNotFound {
				return false
			}

			if v, ok := a.Error.(*url.Error); ok {
				e := v.Error()
				// Don't retry if the error was due to too many redirects.
				if redirectsErrorRe.MatchString(e) {
					return false
				}

				// Don't retry if the error was due to an invalid protocol scheme.
				if schemeErrorRe.MatchString(e) {
					return false
				}

				// Don't retry if the error was due to TLS cert verification failure.
				if _, ok := v.Err.(x509.UnknownAuthorityError); ok {
					return false
				}

			}
			// The error is likely recoverable so retry.
			return true
		}

		// If we have some 5xx response or 429 response that could work after
		// a few retries, retry the request, as determined by retryWithRetryAfter
		if status == 0 ||
			(status >= 500 && status != http.StatusNotImplemented) ||
			status == http.StatusTooManyRequests {
			retry, retryAfterHeader = retryWithRetryAfter(a.Response, maxRetryAfterDuration)
			return retry
		}

		return false
	}
}

// retryWithRetryAfter always retries, unless we have a non-nil response that
// indicates a retry-after header as outlined here: https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Retry-After
func retryWithRetryAfter(response *http.Response, retryAfterMaxSleepDuration time.Duration) (bool, string) {
	// If a retry-after header exists, we only want to retry if it might resolve
	// the issue.
	retryAfterHeader, retryAfter := extractRetryAfter(response)
	if retryAfter != nil {
		// Retry if retry-after is within the maximum sleep duration, otherwise
		// there's no point retrying
		return *retryAfter <= retryAfterMaxSleepDuration, retryAfterHeader
	}

	// Otherwise, default to the behavior we always had: retry.
	return true, retryAfterHeader
}

// extractRetryAfter attempts to extract a retry-after time from retryAfterHeader,
// returning a nil duration if it cannot infer one.
func extractRetryAfter(response *http.Response) (retryAfterHeader string, retryAfter *time.Duration) {
	if response != nil {
		// See  https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Retry-After
		// for retry-after standards.
		retryAfterHeader = response.Header.Get("retry-after")
		if retryAfterHeader != "" {
			// There are two valid formats for retry-after headers: seconds
			// until retry in int, or a RFC1123 date string.
			// First, see if it is denoted in seconds.
			s, err := strconv.Atoi(retryAfterHeader)
			if err == nil {
				d := time.Duration(s) * time.Second
				return retryAfterHeader, &d
			}

			// If we weren't able to parse as seconds, try to parse as RFC1123.
			if err != nil {
				after, err := time.Parse(time.RFC1123, retryAfterHeader)
				if err != nil {
					// We don't know how to parse this header
					return retryAfterHeader, nil
				}
				in := time.Until(after)
				return retryAfterHeader, &in
			}
		}
	}
	return retryAfterHeader, nil
}

// expJitterDelayOrRetryAfterDelay returns a DelayFn that returns a delay
// between 0 and base * 2^attempt capped at max (an exponential backoff delay
// with jitter), unless a 'retry-after' value is provided in the response - then
// the 'retry-after' duration is used, up to max.
//
// See the full jitter algorithm in:
// http://www.awsarchitectureblog.com/2015/03/backoff.html
func expJitterDelayOrRetryAfterDelay(base, max time.Duration) delayFn {
	var mu sync.Mutex
	prng := rand.New(rand.NewSource(time.Now().UnixNano()))
	return func(attempt Attempt) time.Duration {
		var delay time.Duration
		if _, retryAfter := extractRetryAfter(attempt.Response); retryAfter != nil {
			// Delay by what upstream request tells us. If retry-after is
			// significantly higher than max, then it should be up to the retry
			// policy to choose not to retry the request.
			delay = *retryAfter
		} else {
			// Otherwise, generate a delay with some jitter.
			exp := math.Pow(2, float64(attempt.Index))
			top := float64(base) * exp
			n := int64(math.Min(float64(max), top))
			if n <= 0 {
				return base
			}

			mu.Lock()
			delay = time.Duration(prng.Int63n(n))
			mu.Unlock()
		}

		// Overflow handling
		switch {
		case delay < base:
			return base
		case delay > max:
			return max
		default:
			return delay
		}
	}
}

// Everything below this line is heavily inspired by https://github.com/PuerkitoBio/rehttp
// but modified and simplified for Sourcegraph needs such as:
// - Not supporting old Go versions for simplicity
// - Adding support for GetBody on the Request to reduce memory overhead
// The BSD 3-clause license (https://opensource.org/licenses/BSD-3-Clause).

// Copyright (c) 2015-2016, Martin Angers
// All rights reserved.

// Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:

// * Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.

// * Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

// * Neither the name of the author nor the names of its contributors may be used to endorse or promote products derived from this software without specific prior written permission.

// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

// Attempt holds the data describing a RoundTrip attempt.
type Attempt struct {
	// Index is the attempt index starting at 0.
	Index int

	// Request is the request for this attempt. If a non-nil Response
	// is present, this is the same as Response.Request, but since a
	// Response may not be available, it is guaranteed to be set on this
	// field.
	Request *http.Request

	// Response is the response for this attempt. It may be nil if an
	// error occurred an no response was received.
	Response *http.Response

	// Error is the error returned by the attempt, if any.
	Error error
}

// retryWithDelayFn is the signature for functions that implement retry strategies.
type retryWithDelayFn func(attempt Attempt) (bool, time.Duration)

// delayFn is the signature for functions that return the delay to apply
// before the next retry.
type delayFn func(attempt Attempt) time.Duration

// retryFn is the signature for functions that return whether a
// retry should be done for the request.
type retryFn func(attempt Attempt) bool

// toRetryFn combines retry and delay into a retryFn.
func toRetryFn(retry retryFn, delay delayFn) retryWithDelayFn {
	return func(attempt Attempt) (bool, time.Duration) {
		if ok := retry(attempt); !ok {
			return false, 0
		}
		return true, delay(attempt)
	}
}

// NewRetryingTransport creates a Transport with a retry strategy based on
// retry and delay to control the retry logic. It uses the provided
// RoundTripper to execute the requests. If rt is nil,
// http.DefaultTransport is used.
func NewRetryingTransport(rt http.RoundTripper, retry retryFn, delay delayFn) *retryTransport {
	if rt == nil {
		rt = http.DefaultTransport
	}
	return &retryTransport{
		RoundTripper: rt,
		predicate:    toRetryFn(retry, delay),
	}
}

// retryTransport wraps a RoundTripper such as *http.Transport and adds
// retry logic.
type retryTransport struct {
	http.RoundTripper

	predicate retryWithDelayFn
}

// RoundTrip implements http.RoundTripper for the Transport type.
// It calls its underlying http.RoundTripper to execute the request, and
// adds retry logic as per its configuration.
func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var attempt int

	// buffer the body if needed. If req.GetBody is set, we will use that to reset
	// the reader instead.
	var bufferedReqBody *bytes.Reader
	if req.GetBody == nil && req.Body != nil && req.Body != http.NoBody {
		var buf bytes.Buffer
		_, err := io.Copy(&buf, req.Body)
		req.Body.Close()
		if err != nil {
			// cannot even try the first attempt, body has been consumed
			return nil, err
		}

		bufferedReqBody = bytes.NewReader(buf.Bytes())
		req.Body = io.NopCloser(bufferedReqBody)
	}

	for {
		res, err := t.RoundTripper.RoundTrip(req)

		a := Attempt{
			Request:  req,
			Response: res,
			Index:    attempt,
			Error:    err,
		}
		retry, delay := t.predicate(a)
		if !retry {
			return res, err
		}

		// If the GetBody method is set, use that to reset the request body,
		// so we don't need to buffer it.
		if req.GetBody != nil {
			req.Body, err = req.GetBody()
			if err != nil {
				// failed to retry, return the results
				return res, err
			}
		} else if bufferedReqBody != nil {
			// Otherwise, if we buffered the body, reset the buffer reader to position 0.
			if _, serr := bufferedReqBody.Seek(0, 0); serr != nil {
				// failed to retry, return the results
				return res, err
			}
			req.Body = io.NopCloser(bufferedReqBody)
		}
		// close the disposed response's body, if any
		if res != nil {
			// Drain the body.
			_, _ = io.Copy(io.Discard, res.Body)
			res.Body.Close()
		}

		select {
		case <-time.After(delay):
			attempt++
		case <-req.Context().Done():
			// request canceled by caller, don't retry
			return nil, errors.New("net/http: request canceled")
		}
	}
}
