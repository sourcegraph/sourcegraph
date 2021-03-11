package httpheader

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/licensing"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/schema"
)

// SEE ALSO FOR MANUAL TESTING: See the Middleware docstring for information about the testproxy
// helper program, which helps with manual testing of the HTTP auth proxy behavior.
func TestMiddleware(t *testing.T) {
	defer licensing.TestingSkipFeatureChecks()()

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor := actor.FromContext(r.Context())
		if actor.IsAuthenticated() {
			fmt.Fprintf(w, "user %v", actor.UID)
		} else {
			fmt.Fprint(w, "no user")
		}
	}))

	const headerName = "x-sso-user-header"
	conf.Mock(&conf.Unified{SiteConfiguration: schema.SiteConfiguration{AuthProviders: []schema.AuthProviders{{HttpHeader: &schema.HTTPHeaderAuthProvider{UsernameHeader: headerName}}}}})
	defer conf.Mock(nil)

	t.Run("not sent", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		handler.ServeHTTP(rr, req)
		if got, want := rr.Body.String(), "no user"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("not sent, actor present", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		req = req.WithContext(actor.WithActor(context.Background(), &actor.Actor{UID: 123}))
		handler.ServeHTTP(rr, req)
		if got, want := rr.Body.String(), "user 123"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("sent, user", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set(headerName, "alice")
		var calledMock bool
		auth.MockGetAndSaveUser = func(ctx context.Context, op auth.GetAndSaveUserOp) (userID int32, safeErrMsg string, err error) {
			calledMock = true
			if op.ExternalAccount.ServiceType == "http-header" && op.ExternalAccount.ServiceID == "" && op.ExternalAccount.ClientID == "" && op.ExternalAccount.AccountID == "alice" {
				return 1, "", nil
			}
			return 0, "safeErr", fmt.Errorf("account %v not found in mock", op.ExternalAccount)
		}
		defer func() { auth.MockGetAndSaveUser = nil }()
		handler.ServeHTTP(rr, req)
		if got, want := rr.Body.String(), "user 1"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if !calledMock {
			t.Error("!calledMock")
		}
	})

	t.Run("sent, actor already set", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set(headerName, "alice")
		req = req.WithContext(actor.WithActor(context.Background(), &actor.Actor{UID: 123}))
		handler.ServeHTTP(rr, req)
		if got, want := rr.Body.String(), "user 123"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("sent, with un-normalized username", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set(headerName, "alice_zhao")
		const wantNormalizedUsername = "alice-zhao"
		var calledMock bool
		auth.MockGetAndSaveUser = func(ctx context.Context, op auth.GetAndSaveUserOp) (userID int32, safeErrMsg string, err error) {
			calledMock = true
			if op.UserProps.Username != wantNormalizedUsername {
				t.Errorf("got %q, want %q", op.UserProps.Username, wantNormalizedUsername)
			}
			if op.ExternalAccount.ServiceType == "http-header" && op.ExternalAccount.ServiceID == "" && op.ExternalAccount.ClientID == "" && op.ExternalAccount.AccountID == "alice_zhao" {
				return 1, "", nil
			}
			return 0, "safeErr", fmt.Errorf("account %v not found in mock", op.ExternalAccount)
		}
		defer func() { auth.MockGetAndSaveUser = nil }()
		handler.ServeHTTP(rr, req)
		if got, want := rr.Body.String(), "user 1"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if !calledMock {
			t.Error("!calledMock")
		}
	})
}

func TestMiddleware_stripPrefix(t *testing.T) {
	defer licensing.TestingSkipFeatureChecks()()

	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor := actor.FromContext(r.Context())
		if actor.IsAuthenticated() {
			fmt.Fprintf(w, "user %v", actor.UID)
		} else {
			fmt.Fprint(w, "no user")
		}
	}))

	const headerName = "x-sso-user-header"
	conf.Mock(&conf.Unified{SiteConfiguration: schema.SiteConfiguration{
		AuthProviders: []schema.AuthProviders{
			{
				HttpHeader: &schema.HTTPHeaderAuthProvider{
					UsernameHeader:            headerName,
					StripUsernameHeaderPrefix: "accounts.google.com:",
				},
			},
		},
	}})
	defer conf.Mock(nil)

	t.Run("sent, user", func(t *testing.T) {
		rr := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/", nil)
		req.Header.Set(headerName, "accounts.google.com:alice")
		var calledMock bool
		auth.MockGetAndSaveUser = func(ctx context.Context, op auth.GetAndSaveUserOp) (userID int32, safeErrMsg string, err error) {
			calledMock = true
			if op.ExternalAccount.ServiceType == "http-header" && op.ExternalAccount.ServiceID == "" && op.ExternalAccount.ClientID == "" && op.ExternalAccount.AccountID == "alice" {
				return 1, "", nil
			}
			return 0, "safeErr", fmt.Errorf("account %v not found in mock", op.ExternalAccount)
		}
		defer func() { auth.MockGetAndSaveUser = nil }()
		handler.ServeHTTP(rr, req)
		if got, want := rr.Body.String(), "user 1"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if !calledMock {
			t.Error("!calledMock")
		}
	})
}
