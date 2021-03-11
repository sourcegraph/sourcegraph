package database

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"database/sql"
	"math/big"
	"net/http"
	"reflect"
	"testing"

	"github.com/gomodule/oauth1/oauth"
	"github.com/google/go-cmp/cmp"
	"github.com/keegancsmith/sqlf"

	"github.com/sourcegraph/sourcegraph/internal/database/dbtesting"
	"github.com/sourcegraph/sourcegraph/internal/errcode"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/auth"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketserver"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/gitlab"
	"github.com/sourcegraph/sourcegraph/internal/types"
)

func TestUserCredentials_CreateUpdate(t *testing.T) {
	db := dbtesting.GetDB(t)
	ctx, user := setUpUserCredentialTest(t, db)

	// Versions of Go before 1.14.x (where 3 < x < 11) cannot diff *big.Int
	// fields, which causes problems when diffing the keys embedded in OAuth
	// clients. We'll just define a little helper here to wrap cmp.Diff and get
	// us over the hump.
	diffAuthenticators := func(a, b auth.Authenticator) string {
		return cmp.Diff(a, b, cmp.Comparer(func(a, b *big.Int) bool {
			return a.Cmp(b) == 0
		}))
	}

	// Instead of two of every animal, we want one of every authenticator. Same,
	// same.
	for name, auth := range createUserCredentialAuths(t) {
		t.Run(name, func(t *testing.T) {
			scope := UserCredentialScope{
				Domain:              name,
				UserID:              user.ID,
				ExternalServiceType: extsvc.TypeGitHub,
				ExternalServiceID:   "https://github.com",
			}

			cred, err := UserCredentials(db).Create(ctx, scope, auth)
			if err != nil {
				t.Errorf("unexpected non-nil error: %v", err)
			} else if cred == nil {
				t.Error("unexpected nil credential")
			}

			if cred.ID == 0 {
				t.Error("unexpected zero ID")
			}
			if cred.Domain != scope.Domain {
				t.Errorf("unexpected domain: have=%q want=%q", cred.Domain, scope.Domain)
			}
			if cred.UserID != scope.UserID {
				t.Errorf("unexpected ID: have=%d want=%d", cred.UserID, scope.UserID)
			}
			if cred.ExternalServiceType != scope.ExternalServiceType {
				t.Errorf("unexpected external service type: have=%q want=%q", cred.ExternalServiceType, scope.ExternalServiceType)
			}
			if cred.ExternalServiceID != scope.ExternalServiceID {
				t.Errorf("unexpected external service id: have=%q want=%q", cred.ExternalServiceID, scope.ExternalServiceID)
			}
			if diff := diffAuthenticators(cred.Credential, auth); diff != "" {
				t.Errorf("unexpected credential:\n%s", diff)
			}
			if cred.CreatedAt.IsZero() {
				t.Errorf("unexpected zero creation time")
			}
			if cred.UpdatedAt.IsZero() {
				t.Errorf("unexpected zero update time")
			}

			// Ensure that trying to insert again fails.
			if cred, err := UserCredentials(db).Create(ctx, scope, auth); err == nil {
				t.Error("unexpected nil error")
			} else if cred != nil {
				t.Errorf("unexpected non-nil credential: %v", cred)
			}

			newExternalServiceType := extsvc.TypeGitLab

			cred.ExternalServiceType = newExternalServiceType

			if err := UserCredentials(db).Update(ctx, cred); err != nil {
				t.Errorf("unexpected non-nil error updating: %+v", err)
			}

			updatedCred, err := UserCredentials(db).GetByID(ctx, cred.ID)
			if err != nil {
				t.Errorf("unexpected non-nil error getting credential: %+v", err)
			}
			if diff := cmp.Diff(cred, updatedCred); diff != "" {
				t.Errorf("credential incorrectly updated: %s", diff)
			}
		})
	}
}

func TestUserCredentials_Delete(t *testing.T) {
	db := dbtesting.GetDB(t)
	ctx, user := setUpUserCredentialTest(t, db)

	t.Run("nonextant", func(t *testing.T) {
		err := UserCredentials(db).Delete(ctx, 1)
		if err == nil {
			t.Error("unexpected nil error")
		}

		e, ok := err.(UserCredentialNotFoundErr)
		if !ok {
			t.Errorf("error is not a userCredentialNotFoundError; got %T: %v", err, err)
		}
		if len(e.args) != 1 || e.args[0].(int64) != 1 {
			t.Errorf("unexpected args: have=%v want=[1]", e.args)
		}
	})

	t.Run("extant", func(t *testing.T) {
		scope := UserCredentialScope{
			Domain:              UserCredentialDomainBatches,
			UserID:              user.ID,
			ExternalServiceType: "github",
			ExternalServiceID:   "https://github.com",
		}
		token := &auth.OAuthBearerToken{Token: "abcdef"}

		cred, err := UserCredentials(db).Create(ctx, scope, token)
		if err != nil {
			t.Fatal(err)
		}

		if err := UserCredentials(db).Delete(ctx, cred.ID); err != nil {
			t.Errorf("unexpected non-nil error: %v", err)
		}

		_, err = UserCredentials(db).GetByID(ctx, cred.ID)
		if _, ok := err.(UserCredentialNotFoundErr); !ok {
			t.Errorf("unexpected error retrieving credential after deletion: %v", err)
		}
	})
}

func TestUserCredentials_GetByID(t *testing.T) {
	db := dbtesting.GetDB(t)
	ctx, user := setUpUserCredentialTest(t, db)

	t.Run("nonextant", func(t *testing.T) {
		cred, err := UserCredentials(db).GetByID(ctx, 1)
		if cred != nil {
			t.Errorf("unexpected non-nil credential: %v", cred)
		}
		if err == nil {
			t.Error("unexpected nil error")
		}

		e, ok := err.(UserCredentialNotFoundErr)
		if !ok {
			t.Errorf("error is not a userCredentialNotFoundError; got %T: %v", err, err)
		}
		if len(e.args) != 1 || e.args[0].(int64) != 1 {
			t.Errorf("unexpected args: have=%v want=[1]", e.args)
		}
	})

	t.Run("extant", func(t *testing.T) {
		scope := UserCredentialScope{
			Domain:              UserCredentialDomainBatches,
			UserID:              user.ID,
			ExternalServiceType: "github",
			ExternalServiceID:   "https://github.com",
		}
		token := &auth.OAuthBearerToken{Token: "abcdef"}

		want, err := UserCredentials(db).Create(ctx, scope, token)
		if err != nil {
			t.Fatal(err)
		}

		have, err := UserCredentials(db).GetByID(ctx, want.ID)
		if err != nil {
			t.Errorf("unexpected non-nil error: %v", err)
		}
		if diff := cmp.Diff(have, want); diff != "" {
			t.Errorf("unexpected credential:\n%s", diff)
		}
	})
}

func TestUserCredentials_GetByScope(t *testing.T) {
	db := dbtesting.GetDB(t)
	ctx, user := setUpUserCredentialTest(t, db)

	scope := UserCredentialScope{
		Domain:              UserCredentialDomainBatches,
		UserID:              user.ID,
		ExternalServiceType: "github",
		ExternalServiceID:   "https://github.com",
	}
	token := &auth.OAuthBearerToken{Token: "abcdef"}

	t.Run("nonextant", func(t *testing.T) {
		cred, err := UserCredentials(db).GetByScope(ctx, scope)
		if cred != nil {
			t.Errorf("unexpected non-nil credential: %v", cred)
		}
		if err == nil {
			t.Error("unexpected nil error")
		}

		e, ok := err.(UserCredentialNotFoundErr)
		if !ok {
			t.Errorf("error is not a userCredentialNotFoundError; got %T: %v", err, err)
		}
		if diff := cmp.Diff(e.args, []interface{}{scope}); diff != "" {
			t.Errorf("unexpected args:\n%s", diff)
		}
	})

	t.Run("extant", func(t *testing.T) {
		want, err := UserCredentials(db).Create(ctx, scope, token)
		if err != nil {
			t.Fatal(err)
		}

		have, err := UserCredentials(db).GetByScope(ctx, scope)
		if err != nil {
			t.Errorf("unexpected non-nil error: %v", err)
		}
		if diff := cmp.Diff(have, want); diff != "" {
			t.Errorf("unexpected credential:\n%s", diff)
		}
	})
}

func TestUserCredentials_List(t *testing.T) {
	db := dbtesting.GetDB(t)
	ctx, user := setUpUserCredentialTest(t, db)

	githubScope := UserCredentialScope{
		Domain:              UserCredentialDomainBatches,
		UserID:              user.ID,
		ExternalServiceType: "github",
		ExternalServiceID:   "https://github.com",
	}
	gitlabScope := UserCredentialScope{
		Domain:              UserCredentialDomainBatches,
		UserID:              user.ID,
		ExternalServiceType: "gitlab",
		ExternalServiceID:   "https://gitlab.com",
	}
	token := &auth.OAuthBearerToken{Token: "abcdef"}

	// Unlike the other tests in this file, we'll set up a couple of credentials
	// right now, and then list from there.
	githubCred, err := UserCredentials(db).Create(ctx, githubScope, token)
	if err != nil {
		t.Fatal(err)
	}

	gitlabCred, err := UserCredentials(db).Create(ctx, gitlabScope, token)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("not found", func(t *testing.T) {
		creds, next, err := UserCredentials(db).List(ctx, UserCredentialsListOpts{
			Scope: UserCredentialScope{
				Domain: "this is not a valid domain",
			},
		})
		if err != nil {
			t.Errorf("unexpected non-nil error: %v", err)
		}
		if next != 0 {
			t.Errorf("unexpected next: have=%d want=%d", next, 0)
		}
		if len(creds) != 0 {
			t.Errorf("unexpected non-zero number of credentials: %v", creds)
		}
	})

	for name, tc := range map[string]struct {
		scope UserCredentialScope
		want  *UserCredential
	}{
		"service ID only": {
			scope: UserCredentialScope{
				ExternalServiceID: "https://github.com",
			},
			want: githubCred,
		},
		"service type only": {
			scope: UserCredentialScope{
				ExternalServiceType: "gitlab",
			},
			want: gitlabCred,
		},
		"full scope": {
			scope: githubScope,
			want:  githubCred,
		},
	} {
		t.Run("single match on "+name, func(t *testing.T) {
			creds, next, err := UserCredentials(db).List(ctx, UserCredentialsListOpts{
				Scope: tc.scope,
			})
			if err != nil {
				t.Errorf("unexpected non-nil error: %v", err)
			}
			if next != 0 {
				t.Errorf("unexpected next: have=%d want=%d", next, 0)
			}
			if diff := cmp.Diff(creds, []*UserCredential{tc.want}); diff != "" {
				t.Errorf("unexpected credentials:\n%s", diff)
			}
		})
	}

	for name, opts := range map[string]UserCredentialsListOpts{
		"no options":   {},
		"domain only":  {Scope: UserCredentialScope{Domain: UserCredentialDomainBatches}},
		"user ID only": {Scope: UserCredentialScope{UserID: user.ID}},
		"domain and user ID": {
			Scope: UserCredentialScope{
				Domain: UserCredentialDomainBatches,
				UserID: user.ID,
			},
		},
		"authenticator type": {AuthenticatorType: []UserCredentialType{UserCredentialTypeOAuthBearerToken}},
	} {
		t.Run("multiple matches on "+name, func(t *testing.T) {
			creds, next, err := UserCredentials(db).List(ctx, opts)
			if err != nil {
				t.Errorf("unexpected non-nil error: %v", err)
			}
			if next != 0 {
				t.Errorf("unexpected next: have=%d want=%d", next, 0)
			}
			if diff := cmp.Diff(creds, []*UserCredential{githubCred, gitlabCred}); diff != "" {
				t.Errorf("unexpected credentials:\n%s", diff)
			}
		})

		t.Run("pagination for "+name, func(t *testing.T) {
			o := opts
			o.LimitOffset = &LimitOffset{Limit: 1}
			creds, next, err := UserCredentials(db).List(ctx, o)
			if err != nil {
				t.Errorf("unexpected non-nil error: %v", err)
			}
			if next != 1 {
				t.Errorf("unexpected next: have=%d want=%d", next, 1)
			}
			if diff := cmp.Diff(creds, []*UserCredential{githubCred}); diff != "" {
				t.Errorf("unexpected credentials:\n%s", diff)
			}

			o.LimitOffset = &LimitOffset{Limit: 1, Offset: next}
			creds, next, err = UserCredentials(db).List(ctx, o)
			if err != nil {
				t.Errorf("unexpected non-nil error: %v", err)
			}
			if next != 0 {
				t.Errorf("unexpected next: have=%d want=%d", next, 0)
			}
			if diff := cmp.Diff(creds, []*UserCredential{gitlabCred}); diff != "" {
				t.Errorf("unexpected credentials:\n%s", diff)
			}
		})
	}
}

func TestUserCredentials_Invalid(t *testing.T) {
	db := dbtesting.GetDB(t)
	ctx, user := setUpUserCredentialTest(t, db)

	t.Run("marshal", func(t *testing.T) {
		if _, err := UserCredentials(db).Create(ctx, UserCredentialScope{}, &invalidAuth{}); err == nil {
			t.Error("unexpected nil error")
		}
	})

	t.Run("unmarshal", func(t *testing.T) {
		// We'll set up some cases here that just shouldn't happen at all, and
		// make sure they bubble up with errors where we expect. Let's define a
		// helper to make that easier.

		insertRawCredential := func(t *testing.T, domain string, raw string) int64 {
			q := sqlf.Sprintf(
				userCredentialsCreateQueryFmtstr,
				domain,
				user.ID,
				"type",
				"id",
				raw,
				sqlf.Sprintf("id"),
			)

			var id int64
			row := db.QueryRowContext(ctx, q.Query(sqlf.PostgresBindVar), q.Args()...)
			if err := row.Scan(&id); err != nil {
				t.Fatal(err)
			}

			return id
		}

		for name, id := range map[string]int64{
			"invalid credential type": insertRawCredential(t, "invalid", `{"type":"InvalidType"}`),
			"lying credential type":   insertRawCredential(t, "lying", `{"type":"BasicAuth","username":42}`),
			"malformed JSON":          insertRawCredential(t, "malformed", "this is not valid JSON"),
		} {
			t.Run(name, func(t *testing.T) {
				if _, err := UserCredentials(db).GetByID(ctx, id); err == nil {
					t.Error("unexpected nil error")
				} else if _, ok := err.(UserCredentialNotFoundErr); ok {
					t.Error("unexpected not found error")
				}
			})
		}
	})
}

func TestUserCredentialNotFoundErr(t *testing.T) {
	err := UserCredentialNotFoundErr{}
	if have := errcode.IsNotFound(err); !have {
		t.Error("UserCredentialNotFoundErr does not say it represents a not found error")
	}
}

func createUserCredentialAuths(t *testing.T) map[string]auth.Authenticator {
	t.Helper()

	createOAuthClient := func(t *testing.T, token, secret string) *oauth.Client {
		t.Helper()

		// Generate a random key so we can test different clients are different.
		// Note that this is wildly insecure.
		key, err := rsa.GenerateKey(rand.Reader, 64)
		if err != nil {
			t.Fatal(err)
		}

		return &oauth.Client{
			Credentials: oauth.Credentials{
				Token:  token,
				Secret: secret,
			},
			PrivateKey: key,
		}
	}

	auths := make(map[string]auth.Authenticator)
	for _, a := range []auth.Authenticator{
		&auth.OAuthClient{Client: createOAuthClient(t, "abc", "def")},
		&auth.BasicAuth{Username: "foo", Password: "bar"},
		&auth.OAuthBearerToken{Token: "abcdef"},
		&bitbucketserver.SudoableOAuthClient{
			Client:   auth.OAuthClient{Client: createOAuthClient(t, "ghi", "jkl")},
			Username: "neo",
		},
		&gitlab.SudoableToken{Token: "mnop", Sudo: "qrs"},
	} {
		auths[reflect.TypeOf(a).String()] = a
	}

	return auths
}

func setUpUserCredentialTest(t *testing.T, db *sql.DB) (context.Context, *types.User) {
	if testing.Short() {
		t.Skip()
	}

	t.Helper()
	ctx := context.Background()

	// Create a user that allows us to link the credential somewhere.
	user, err := Users(db).Create(ctx, NewUser{
		Email:                 "a@example.com",
		Username:              "u2",
		Password:              "pw",
		EmailVerificationCode: "c",
	})
	if err != nil {
		t.Fatal(err)
	}

	return ctx, user
}

type invalidAuth struct{}

var _ auth.Authenticator = &invalidAuth{}

func (*invalidAuth) Authenticate(_ *http.Request) error { panic("should not be called") }
func (*invalidAuth) Hash() string                       { panic("should not be called") }
