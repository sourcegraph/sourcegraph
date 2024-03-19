package ui

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gorilla/mux"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/backend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/globals"
	uirouter "github.com/sourcegraph/sourcegraph/cmd/frontend/internal/app/ui/router"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbmocks"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/fileutil"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/repoupdater"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/schema"
	"github.com/sourcegraph/sourcegraph/ui/assets"
)

func TestRedirects(t *testing.T) {
	assets.UseDevAssetsProvider()
	assets.MockLoadWebBuildManifest = func() (*assets.WebBuildManifest, error) {
		return &assets.WebBuildManifest{}, nil
	}
	defer func() { assets.MockLoadWebBuildManifest = nil }()

	check := func(t *testing.T, path string, wantStatusCode int, wantRedirectLocation, userAgent string) {
		t.Helper()

		gss := dbmocks.NewMockGlobalStateStore()
		gss.GetFunc.SetDefaultReturn(database.GlobalState{SiteID: "a"}, nil)

		users := dbmocks.NewMockUserStore()
		users.GetByCurrentAuthUserFunc.SetDefaultReturn(&types.User{ID: 1, SiteAdmin: true}, nil)
		extSvcs := dbmocks.NewMockExternalServiceStore()
		extSvcs.CountFunc.SetDefaultReturn(0, nil)
		repoStatistics := dbmocks.NewMockRepoStatisticsStore()
		repoStatistics.GetRepoStatisticsFunc.SetDefaultReturn(database.RepoStatistics{Total: 1}, nil)

		db := dbmocks.NewMockDB()
		db.GlobalStateFunc.SetDefaultReturn(gss)
		db.UsersFunc.SetDefaultReturn(users)
		db.ExternalServicesFunc.SetDefaultReturn(extSvcs)
		db.RepoStatisticsFunc.SetDefaultReturn(repoStatistics)

		InitRouter(db)
		rw := httptest.NewRecorder()
		req, err := http.NewRequest("GET", path, nil)
		if err != nil {
			t.Fatal(err)
		}

		req.Header.Set("User-Agent", userAgent)
		uirouter.Router.ServeHTTP(rw, req)
		if rw.Code != wantStatusCode {
			t.Errorf("got HTTP response code %d, want %d", rw.Code, wantStatusCode)
		}
		if got := rw.Header().Get("Location"); got != wantRedirectLocation {
			t.Errorf("got redirect location %q, want %q", got, wantRedirectLocation)
		}
	}

	t.Run("on Sourcegraph.com", func(t *testing.T) {
		dotcom.MockSourcegraphDotComMode(t, true)

		t.Run("root", func(t *testing.T) {
			check(t, "/", http.StatusTemporaryRedirect, "/search", "Mozilla/5.0")
		})
	})

	t.Run("on Sourcegraph.com from Cookiebot", func(t *testing.T) {
		dotcom.MockSourcegraphDotComMode(t, true)

		t.Run("root", func(t *testing.T) {
			check(t, "/", http.StatusTemporaryRedirect, "/search", "Mozilla/5.0 Cookiebot")
		})
	})

	t.Run("non-Sourcegraph.com", func(t *testing.T) {
		dotcom.MockSourcegraphDotComMode(t, false)

		t.Run("root", func(t *testing.T) {
			check(t, "/", http.StatusTemporaryRedirect, "/search", "Mozilla/5.0")
		})
	})
}

func TestRepoShortName(t *testing.T) {
	tests := []struct {
		input api.RepoName
		want  string
	}{
		{input: "repo", want: "repo"},
		{input: "github.com/foo/bar", want: "foo/bar"},
		{input: "mycompany.com/foo", want: "foo"},
	}
	for _, tst := range tests {
		t.Run(string(tst.input), func(t *testing.T) {
			got := repoShortName(tst.input)
			if got != tst.want {
				t.Fatalf("input %q got %q want %q", tst.input, got, tst.want)
			}
		})
	}
}

func TestNewCommon_repo_error(t *testing.T) {
	assets.UseDevAssetsProvider()
	assets.MockLoadWebBuildManifest = func() (*assets.WebBuildManifest, error) {
		return &assets.WebBuildManifest{}, nil
	}
	defer func() { assets.MockLoadWebBuildManifest = nil }()

	cases := []struct {
		name string
		rev  string
		err  error

		want string
		code int
	}{{
		name: "cloning",
		err:  &gitdomain.RepoNotExistError{CloneInProgress: true},
		code: 200,
	}, {
		name: "repo-404",
		err:  &gitdomain.RepoNotExistError{Repo: "repo-404"},
		want: "repository does not exist: repo-404",
		code: 404,
	}, {
		name: "rev-404",
		rev:  "@marco",
		err:  &gitdomain.RevisionNotFoundError{Repo: "rev-404", Spec: "marco"},
		want: "revision not found: rev-404@marco",
		code: 404,
	}, {
		name: "repoupdater-not-found",
		err:  &repoupdater.ErrNotFound{Repo: "repo-404", IsNotFound: true},
		want: fmt.Sprintf("repository not found (name=%s notfound=%v)", "repo-404", true),
		code: 404,
	}, {
		name: "repoupdater-unauthorized",
		err:  &repoupdater.ErrUnauthorized{Repo: "repo-unauth", NoAuthz: true},
		want: fmt.Sprintf("not authorized (name=%s noauthz=%v)", "repo-unauth", true),
		code: 401,
	}, {
		name: "github.com/sourcegraphtest/Always500Test",
		want: "error caused by Always500Test repo name",
		code: 500,
	}}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			backend.Mocks.Repos.MockGetByName(t, api.RepoName(tt.name), 1)
			backend.Mocks.Repos.MockGet(t, 1)
			backend.Mocks.Repos.ResolveRev = func(context.Context, api.RepoName, string) (api.CommitID, error) {
				if tt.err != nil {
					return "", tt.err
				}
				return "deadbeef", nil
			}

			req, err := http.NewRequest("GET", "/", nil)
			if err != nil {
				t.Fatal(err)
			}
			req = mux.SetURLVars(req, map[string]string{
				"Repo": tt.name,
				"Rev":  tt.rev,
			})

			code := 200
			got := ""
			serveError := func(w http.ResponseWriter, r *http.Request, db database.DB, err error, statusCode int) {
				got = err.Error()
				code = statusCode
			}

			gss := dbmocks.NewMockGlobalStateStore()
			gss.GetFunc.SetDefaultReturn(database.GlobalState{SiteID: "a"}, nil)

			config := &schema.OtherExternalServiceConnection{
				Url:   "https://url.com",
				Repos: []string{"serve-git-local"},
			}

			bs, err := json.Marshal(config)
			if err != nil {
				t.Fatal(err)
			}

			extSvcOther := types.ExternalService{
				Kind:   extsvc.KindOther,
				ID:     1,
				Config: extsvc.NewUnencryptedConfig(string(bs)),
			}

			extSvcs := dbmocks.NewMockExternalServiceStore()
			extSvcs.ListFunc.SetDefaultReturn([]*types.ExternalService{&extSvcOther}, nil)

			repoStatistics := dbmocks.NewMockRepoStatisticsStore()
			repoStatistics.GetRepoStatisticsFunc.SetDefaultReturn(database.RepoStatistics{Total: 1}, nil)

			users := dbmocks.NewMockUserStore()
			users.GetByCurrentAuthUserFunc.SetDefaultReturn(nil, nil)

			db := dbmocks.NewMockDB()
			db.GlobalStateFunc.SetDefaultReturn(gss)
			db.ExternalServicesFunc.SetDefaultReturn(extSvcs)
			db.RepoStatisticsFunc.SetDefaultReturn(repoStatistics)
			db.UsersFunc.SetDefaultReturn(users)

			_, err = newCommon(httptest.NewRecorder(), req, db, "test", index, serveError)
			if err != nil {
				if got != "" || code != 200 {
					t.Fatal("serveError called and error returned from newCommon")
				}
				code = 500
				got = err.Error()
			}

			if tt.want != got {
				t.Errorf("unexpected error.\ngot:  %s\nwant: %s", got, tt.want)
			}
			if tt.code != code {
				t.Errorf("unexpected status code: got=%d want=%d", code, tt.code)
			}
		})
	}
}

func TestRedirectTreeOrBlob(t *testing.T) {
	tests := []struct {
		name          string
		route         string
		path          string
		common        *Common
		mockStat      fs.FileInfo
		expHandled    bool
		expStatusCode int
		expLocation   string
	}{
		{
			name:          "empty commit ID, no redirect",
			common:        &Common{},
			expStatusCode: http.StatusOK,
		},
		{
			name:  "empty path, no redirect",
			route: routeRepo,
			path:  "",
			common: &Common{
				Repo: &types.Repo{
					Name: "github.com/user/repo",
				},
				CommitID: "eca7e807356b887ee24b7a7497973bbfc5688dac",
			},
			expStatusCode: http.StatusOK,
		},
		{
			name:  "root path, no redirect",
			route: routeRepo,
			path:  "/",
			common: &Common{
				Repo: &types.Repo{
					Name: "github.com/user/repo",
				},
				CommitID: "eca7e807356b887ee24b7a7497973bbfc5688dac",
			},
			expStatusCode: http.StatusOK,
		},
		{
			name:  "view tree, no redirect",
			route: routeTree,
			path:  "/some/dir",
			common: &Common{
				Repo: &types.Repo{
					Name: "github.com/user/repo",
				},
				CommitID: "eca7e807356b887ee24b7a7497973bbfc5688dac",
			},
			mockStat:      &fileutil.FileInfo{Mode_: os.ModeDir},
			expStatusCode: http.StatusOK,
		},
		{
			name:  "view blob, no redirect",
			route: routeBlob,
			path:  "/some/file.go",
			common: &Common{
				Repo: &types.Repo{
					Name: "github.com/user/repo",
				},
				CommitID: "eca7e807356b887ee24b7a7497973bbfc5688dac",
			},
			mockStat:      &fileutil.FileInfo{}, // Not a directory
			expStatusCode: http.StatusOK,
		},

		// "/github.com/user/repo/-/tree/some/file.go" -> "/github.com/user/repo/-/blob/some/file.go"
		{
			name:  "redirct tree to blob",
			route: routeTree,
			path:  "/some/file.go",
			common: &Common{
				Repo: &types.Repo{
					Name: "github.com/user/repo",
				},
				CommitID: "eca7e807356b887ee24b7a7497973bbfc5688dac",
			},
			mockStat:      &fileutil.FileInfo{}, // Not a directory
			expHandled:    true,
			expStatusCode: http.StatusTemporaryRedirect,
			expLocation:   "/github.com/user/repo/-/blob/some/file.go",
		},
		// "/github.com/user/repo/-/blob/some/dir" -> "/github.com/user/repo/-/tree/some/dir"
		{
			name:  "redirct blob to tree",
			route: routeBlob,
			path:  "/some/dir",
			common: &Common{
				Repo: &types.Repo{
					Name: "github.com/user/repo",
				},
				CommitID: "eca7e807356b887ee24b7a7497973bbfc5688dac",
			},
			mockStat:      &fileutil.FileInfo{Mode_: os.ModeDir},
			expHandled:    true,
			expStatusCode: http.StatusTemporaryRedirect,
			expLocation:   "/github.com/user/repo/-/tree/some/dir",
		},
		// "/github.com/user/repo@master/-/tree/some/file.go" -> "/github.com/user/repo@master/-/blob/some/file.go"
		{
			name:  "redirct tree to blob on a revision",
			route: routeTree,
			path:  "/some/file.go",
			common: &Common{
				Repo: &types.Repo{
					Name: "github.com/user/repo",
				},
				Rev:      "@master",
				CommitID: "eca7e807356b887ee24b7a7497973bbfc5688dac",
			},
			mockStat:      &fileutil.FileInfo{}, // Not a directory
			expHandled:    true,
			expStatusCode: http.StatusTemporaryRedirect,
			expLocation:   "/github.com/user/repo@master/-/blob/some/file.go",
		},
		// "/github.com/user/repo@master/-/blob/some/dir" -> "/github.com/user/repo@master/-/tree/some/dir"
		{
			name:  "redirct blob to tree on a revision",
			route: routeBlob,
			path:  "/some/dir",
			common: &Common{
				Repo: &types.Repo{
					Name: "github.com/user/repo",
				},
				Rev:      "@master",
				CommitID: "eca7e807356b887ee24b7a7497973bbfc5688dac",
			},
			mockStat:      &fileutil.FileInfo{Mode_: os.ModeDir},
			expHandled:    true,
			expStatusCode: http.StatusTemporaryRedirect,
			expLocation:   "/github.com/user/repo@master/-/tree/some/dir",
		},

		// "/github.com/user/repo/-/tree" -> "/github.com/user/repo"
		{
			name:  "redirct tree to root",
			route: routeTree,
			path:  "",
			common: &Common{
				Repo: &types.Repo{
					Name: "github.com/user/repo",
				},
				CommitID: "eca7e807356b887ee24b7a7497973bbfc5688dac",
			},
			expHandled:    true,
			expStatusCode: http.StatusTemporaryRedirect,
			expLocation:   "/github.com/user/repo",
		},
		// "/github.com/user/repo/-/blob" -> "/github.com/user/repo"
		{
			name:  "redirct blob to root",
			route: routeBlob,
			path:  "",
			common: &Common{
				Repo: &types.Repo{
					Name: "github.com/user/repo",
				},
				CommitID: "eca7e807356b887ee24b7a7497973bbfc5688dac",
			},
			expHandled:    true,
			expStatusCode: http.StatusTemporaryRedirect,
			expLocation:   "/github.com/user/repo",
		},
		// "/github.com/user/repo@master/-/tree" -> "/github.com/user/repo"
		{
			name:  "redirct tree to root on a revision",
			route: routeTree,
			path:  "",
			common: &Common{
				Repo: &types.Repo{
					Name: "github.com/user/repo",
				},
				Rev:      "@master",
				CommitID: "eca7e807356b887ee24b7a7497973bbfc5688dac",
			},
			expHandled:    true,
			expStatusCode: http.StatusTemporaryRedirect,
			expLocation:   "/github.com/user/repo@master",
		},
		// "/github.com/user/repo@master/-/blob" -> "/github.com/user/repo"
		{
			name:  "redirct blob to root on a revision",
			route: routeBlob,
			path:  "",
			common: &Common{
				Repo: &types.Repo{
					Name: "github.com/user/repo",
				},
				Rev:      "@master",
				CommitID: "eca7e807356b887ee24b7a7497973bbfc5688dac",
			},
			expHandled:    true,
			expStatusCode: http.StatusTemporaryRedirect,
			expLocation:   "/github.com/user/repo@master",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gsClient := gitserver.NewMockClient()
			gsClient.StatFunc.SetDefaultReturn(test.mockStat, nil)

			w := httptest.NewRecorder()
			r, err := http.NewRequest("GET", test.path, nil)
			if err != nil {
				t.Fatal(err)
			}

			handled, err := redirectTreeOrBlob(test.route, test.path, test.common, w, r, dbmocks.NewMockDB(), gsClient)
			if err != nil {
				t.Fatal(err)
			}

			if handled != test.expHandled {
				t.Fatalf("handled: want %v but got %v", test.expHandled, handled)
			} else if w.Code != test.expStatusCode {
				t.Fatalf("code: want %d but got %d", test.expStatusCode, w.Code)
			}

			if got := w.Header().Get("Location"); got != test.expLocation {
				t.Fatalf("redirect location: want %q but got %q", test.expLocation, got)
			}
		})
	}
}

func init() {
	globals.ConfigurationServerFrontendOnly = &conf.Server{}
	gss := dbmocks.NewMockGlobalStateStore()
	gss.GetFunc.SetDefaultReturn(database.GlobalState{SiteID: "a"}, nil)

	db := dbmocks.NewMockDB()
	db.GlobalStateFunc.SetDefaultReturn(gss)
}
