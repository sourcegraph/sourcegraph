package defaults

import (
	"context"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufferSize = 1024 * 1024

func TestCloseGRPCConnectionCallback(t *testing.T) {
	listener := bufconn.Listen(bufferSize)
	defer listener.Close()

	// Start a fake GRPC server
	fakeServer := grpc.NewServer()
	defer fakeServer.Stop()

	go func() {
		if err := fakeServer.Serve(listener); err != nil {
			t.Errorf("gRPC server exited with error: %v", err)
			return
		}
	}()

	opts := []grpc.DialOption{
		grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
			return listener.Dial()
		}),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	}

	conn, err := grpc.DialContext(context.Background(), "doesn't matter", opts...)
	if err != nil {
		t.Fatalf("failed to dial gRPC server: %v", err)
	}

	defer conn.Close() // ensure the connection is closed when test ends

	ce := connAndError{conn: conn, dialErr: err}

	// Wait for the connection to be ready, or give up after timeout

	connectionInitialized := make(chan struct{})

	timeout := 5 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
				state := ce.conn.GetState()
				if state != connectivity.Idle && state != connectivity.Connecting {
					close(connectionInitialized)
					return
				}
			}
		}
	}()

	select {
	case <-ctx.Done():
		t.Fatalf("failed to connect to gRPC server within %s, state: %q", timeout.String(), ce.conn.GetState().String())
	case <-connectionInitialized:
	}

	// Double check that the connection is ready
	if state := ce.conn.GetState(); state != connectivity.Ready {
		t.Fatalf("expected gRPC connection to be in state %q, got state: %s", connectivity.Ready, state.String())
	}

	// Run test: run close connection callback
	closeGRPCConnection("", ce)

	// Try closing connection again, should return codes.Canceled error (i.e. connection already closed)
	err = ce.conn.Close()
	if status.Code(err) != codes.Canceled {
		t.Fatalf("expected %q code after closing connection twice, got err: %v", codes.Canceled.String(), err)
	}
}

func TestParseAddress(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected *url.URL
	}{
		{
			name: "valid URL",

			input: "https://example.com",
			expected: &url.URL{
				Scheme: "https",
				Host:   "example.com",
			},
		},
		{
			name: "host:port pair",

			input: "example.com:8080",
			expected: &url.URL{
				Host: "example.com:8080",
			},
		},
		{
			name:  "gitserver URL with port and scheme",
			input: "http://gitserver-0:3181",
			expected: &url.URL{
				Scheme: "http",
				Host:   "gitserver-0:3181",
			},
		},
		{
			name:  "IPv4 host:port",
			input: "127.0.0.1:3181",
			expected: &url.URL{
				Host: "127.0.0.1:3181",
			},
		},
		{
			name:  "IPv4 URL with port",
			input: "http://127.0.0.1:3181",
			expected: &url.URL{
				Scheme: "http",
				Host:   "127.0.0.1:3181",
			},
		},
		{
			name:  "IPv6 host:port",
			input: "[dead:beef::3]:80",
			expected: &url.URL{
				Host: "[dead:beef::3]:80",
			},
		},
		{
			name:  "IPv6 URL with port",
			input: "http://[dead:beef::3]:80",
			expected: &url.URL{
				Scheme: "http",
				Host:   "[dead:beef::3]:80",
			},
		},
		{
			name:     "empty string",
			input:    "",
			expected: &url.URL{},
		},
		{
			name:  "hostname without port",
			input: "example.com",
			expected: &url.URL{
				Host: "example.com",
			},
		},
		{
			name:  "non-standard scheme",
			input: "ftp://example.com",
			expected: &url.URL{
				Scheme: "ftp",
				Host:   "example.com",
			},
		},
		{
			name:  "URL with path, query, and fragment",
			input: "http://example.com/path?query#fragment",
			expected: &url.URL{
				Scheme:   "http",
				Host:     "example.com",
				Path:     "/path",
				RawQuery: "query",
				Fragment: "fragment",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := parseAddress(tc.input)
			if err != nil {
				t.Fatalf("unexpected error: %+v", err)
			}

			if diff := cmp.Diff(tc.expected.String(), u.String()); diff != "" {
				t.Fatalf("unexpected diff (-want +got):\n%s", diff)
			}

		})
	}
}
