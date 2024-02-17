package bitbucketcloudoauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	bitbucketlogin "github.com/dghubble/gologin/v2/bitbucket"
	"github.com/google/go-cmp/cmp"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/auth"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/bitbucketcloud"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

type emailResponse struct {
	Values []bitbucketcloud.UserEmail `json:"values"`
}

var returnUsername string
var returnAccountID string
var returnEmails emailResponse

func createTestServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/user") {
			json.NewEncoder(w).Encode(struct {
				Username string `json:"username"`
				UUID     string `json:"uuid"`
			}{
				Username: returnUsername,
				UUID:     returnAccountID,
			})
			return
		}
		if strings.HasSuffix(r.URL.Path, "/user/emails") {
			json.NewEncoder(w).Encode(returnEmails)
			return
		}

	}))
}

func TestSessionIssuerHelper_GetOrCreateUser(t *testing.T) {
	ratelimit.SetupForTest(t)

	server := createTestServer()
	defer server.Close()
	bbURL, _ := url.Parse(server.URL)
	clientID := "client-id"

	// Top-level mock data
	//
	// authSaveableUsers that will be accepted by auth.GetAndSaveUser
	authSaveableUsers := map[string]int32{
		"alice": 1,
	}

	type input struct {
		description     string
		bbUser          *bitbucketlogin.User
		bbUserEmails    []bitbucketcloud.UserEmail
		bbUserEmailsErr error
		allowSignup     bool
	}
	cases := []struct {
		inputs        []input
		expActor      *actor.Actor
		expErr        bool
		expAuthUserOp *auth.GetAndSaveUserOp
	}{
		{
			inputs: []input{{
				description: "bbUser, verified email -> session created",
				bbUser:      &bitbucketlogin.User{Username: "alice"},
				bbUserEmails: []bitbucketcloud.UserEmail{
					{
						Email:       "alice@example.com",
						IsConfirmed: true,
						IsPrimary:   true,
					},
				},
			}},
			expActor: &actor.Actor{UID: 1},
			expAuthUserOp: &auth.GetAndSaveUserOp{
				UserProps:       u("alice", "alice@example.com", true),
				ExternalAccount: acct(extsvc.TypeBitbucketCloud, server.URL+"/", clientID, "1234"),
			},
		},
		{
			inputs: []input{{
				description: "bbUser, primary email not verified but another is -> no session created",
				bbUser:      &bitbucketlogin.User{Username: "alice"},
				bbUserEmails: []bitbucketcloud.UserEmail{
					{
						Email:       "alice@example1.com",
						IsPrimary:   true,
						IsConfirmed: false,
					},
					{
						Email:       "alice@example2.com",
						IsPrimary:   false,
						IsConfirmed: true,
					},
				},
			}},
			expActor: &actor.Actor{UID: 1},
			expAuthUserOp: &auth.GetAndSaveUserOp{
				UserProps:       u("alice", "alice@example2.com", true),
				ExternalAccount: acct(extsvc.TypeBitbucketCloud, server.URL+"/", clientID, "1234"),
			},
		},
		{
			inputs: []input{{
				description:  "bbUser, no emails -> no session created",
				bbUser:       &bitbucketlogin.User{Username: "alice"},
				bbUserEmails: []bitbucketcloud.UserEmail{},
			}, {
				description:     "bbUser, email fetching err -> no session created",
				bbUser:          &bitbucketlogin.User{Username: "alice"},
				bbUserEmailsErr: errors.New("x"),
			}, {
				description: "bbUser, plenty of emails but none verified -> no session created",
				bbUser:      &bitbucketlogin.User{Username: "alice"},
				bbUserEmails: []bitbucketcloud.UserEmail{
					{
						Email:       "alice@example1.com",
						IsPrimary:   true,
						IsConfirmed: false,
					},
					{
						Email:       "alice@example2.com",
						IsPrimary:   true,
						IsConfirmed: false,
					},
					{
						Email:       "alice@example3.com",
						IsPrimary:   true,
						IsConfirmed: false,
					},
				},
			}, {
				description: "no bbUser -> no session created",
			}, {
				description: "bbUser, verified email, unsaveable -> no session created",
				bbUser:      &bitbucketlogin.User{Username: "bob"},
			}},
			expErr: true,
		},
	}
	for _, c := range cases {
		for _, ci := range c.inputs {
			c, ci := c, ci
			t.Run(ci.description, func(t *testing.T) {
				if ci.bbUser != nil {
					returnUsername = ci.bbUser.Username
					returnAccountID = "1234"
				}
				returnEmails.Values = ci.bbUserEmails

				var gotAuthUserOp *auth.GetAndSaveUserOp
				auth.MockGetAndSaveUser = func(ctx context.Context, op auth.GetAndSaveUserOp) (newUserCreated bool, userID int32, safeErrMsg string, err error) {
					if gotAuthUserOp != nil {
						t.Fatal("GetAndSaveUser called more than once")
					}
					op.ExternalAccountData = extsvc.AccountData{} // ignore AccountData value
					gotAuthUserOp = &op

					if uid, ok := authSaveableUsers[op.UserProps.Username]; ok {
						return false, uid, "", nil
					}
					return false, 0, "safeErr", errors.New("auth.GetAndSaveUser error")
				}
				defer func() {
					auth.MockGetAndSaveUser = nil
				}()

				ctx := bitbucketlogin.WithUser(context.Background(), ci.bbUser)
				conf := &schema.BitbucketCloudConnection{
					Url:    server.URL,
					ApiURL: server.URL,
				}
				bbClient, err := bitbucketcloud.NewClient(server.URL, conf, nil)
				if err != nil {
					t.Fatal(err)
				}
				s := &sessionIssuerHelper{
					baseURL:     extsvc.NormalizeBaseURL(bbURL),
					clientKey:   clientID,
					allowSignup: ci.allowSignup,
					client:      bbClient,
				}

				tok := &oauth2.Token{AccessToken: "dummy-value-that-isnt-relevant-to-unit-correctness"}
				_, actr, _, err := s.GetOrCreateUser(ctx, tok, nil)
				if c.expErr && err == nil {
					t.Errorf("expected err %v, but was nil", c.expErr)
				} else if !c.expErr && err != nil {
					t.Errorf("expected no error, but was %v", err)
				}

				if got, exp := actr, c.expActor; !reflect.DeepEqual(got, exp) {
					t.Errorf("expected actor %v, got %v", exp, got)
				}

				if got, exp := gotAuthUserOp, c.expAuthUserOp; !reflect.DeepEqual(got, exp) {
					t.Error(cmp.Diff(got, exp))
				}
			})
		}
	}
}

func TestSessionIssuerHelper_SignupMatchesSecondaryAccount(t *testing.T) {
	ratelimit.SetupForTest(t)

	server := createTestServer()
	defer server.Close()

	returnEmails = emailResponse{Values: []bitbucketcloud.UserEmail{
		{
			Email:       "primary@example.com",
			IsPrimary:   true,
			IsConfirmed: true,
		},
		{
			Email:       "secondary@example.com",
			IsPrimary:   false,
			IsConfirmed: true,
		},
	}}

	// We just want to make sure that we end up getting to the secondary email
	auth.MockGetAndSaveUser = func(ctx context.Context, op auth.GetAndSaveUserOp) (newUserCreated bool, userID int32, safeErrMsg string, err error) {
		if op.CreateIfNotExist {
			// We should not get here as we should hit the second email address
			// before trying again with creation enabled.
			t.Fatal("Should not get here")
		}
		// Mock the second email address matching
		if op.UserProps.Email == "secondary@example.com" {
			return false, 1, "", nil
		}
		return false, 0, "no match", errors.New("no match")
	}
	defer func() {
		auth.MockGetAndSaveUser = nil
	}()

	bbURL, _ := url.Parse(server.URL)
	clientID := "client-id"
	returnUsername = "alice"
	returnAccountID = "1234"

	bbUser := &bitbucketlogin.User{
		Username: returnUsername,
	}

	ctx := bitbucketlogin.WithUser(context.Background(), bbUser)
	conf := &schema.BitbucketCloudConnection{
		Url:    server.URL,
		ApiURL: server.URL,
	}
	bbClient, err := bitbucketcloud.NewClient(server.URL, conf, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := &sessionIssuerHelper{
		baseURL:     extsvc.NormalizeBaseURL(bbURL),
		clientKey:   clientID,
		allowSignup: true,
		client:      bbClient,
	}
	tok := &oauth2.Token{AccessToken: "dummy-value-that-isnt-relevant-to-unit-correctness"}
	_, _, _, err = s.GetOrCreateUser(ctx, tok, nil)
	if err != nil {
		t.Fatal(err)
	}
}

func u(username, email string, emailIsVerified bool) database.NewUser {
	return database.NewUser{
		Username:        username,
		Email:           email,
		EmailIsVerified: emailIsVerified,
	}
}

func acct(serviceType, serviceID, clientID, accountID string) extsvc.AccountSpec {
	return extsvc.AccountSpec{
		ServiceType: serviceType,
		ServiceID:   serviceID,
		ClientID:    clientID,
		AccountID:   accountID,
	}
}
