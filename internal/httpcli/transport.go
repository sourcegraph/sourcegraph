package httpcli

import "net/http"

// WrappedTransport can be implemented to allow a wrapped transport to expose its
// underlying transport for modification.
type WrappedTransport interface {
	// RoundTripper is the transport implementation that should be exposed.
	http.RoundTripper

	// Unwrap should provide a pointer to the underlying transport that has been wrapped.
	// The returned value should never be nil.
	Unwrap() *http.RoundTripper
}

// unwrapAll performs a recursive unwrap on transport until we reach a transport that
// cannot be unwrapped. The pointer to the pointer can be used to replace the underlying
// transport, most commonly by attempting to cast it as an *http.Transport.
//
// WrappedTransport.Unwrap should never return nil, so unwrapAll will never return nil.
func unwrapAll(transport WrappedTransport) *http.RoundTripper {
	wrapped := transport.Unwrap()
	if unwrappable, ok := (*wrapped).(WrappedTransport); ok {
		return unwrapAll(unwrappable)
	}
	return wrapped
}

func WrapTransport(transport, base http.RoundTripper) WrappedTransport {
	return &wrappedTransport{
		RoundTripper: transport,
		Wrapped:      base,
	}
}

// wrappedTransport is an http.RoundTripper that allows the underlying RoundTripper to be
// exposed for modification.
type wrappedTransport struct {
	http.RoundTripper
	Wrapped http.RoundTripper
}

var _ WrappedTransport = &wrappedTransport{}

func (wt *wrappedTransport) Unwrap() *http.RoundTripper { return &wt.Wrapped }
