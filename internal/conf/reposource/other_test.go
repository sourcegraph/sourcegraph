package reposource

import (
	"reflect"
	"testing"

	"github.com/sourcegraph/sourcegraph/schema"
)

func TestOtherCloneURLToRepoName(t *testing.T) {
	tests := []struct {
		conn schema.OtherExternalServiceConnection
		urls []urlToRepoNameErr
	}{
		{
			conn: schema.OtherExternalServiceConnection{
				Url:                   "https://github.com",
				RepositoryPathPattern: "{base}/{repo}",
				Repos:                 []string{"gorilla/mux"},
			},
			urls: []urlToRepoNameErr{
				{"https://github.com/gorilla/mux", "github.com/gorilla/mux", nil},
				{"https://github.com/gorilla/mux.git", "github.com/gorilla/mux", nil},
				{"https://asdf.com/gorilla/mux.git", "", nil},
			},
		},
		{
			conn: schema.OtherExternalServiceConnection{
				Url:                   "https://github.com/?access_token=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				RepositoryPathPattern: "{base}/{repo}",
				Repos:                 []string{"gorilla/mux"},
			},
			urls: []urlToRepoNameErr{
				{"https://github.com/gorilla/mux", "github.com/gorilla/mux", nil},
				{"https://github.com/gorilla/mux.git", "github.com/gorilla/mux", nil},
			},
		},
		{
			conn: schema.OtherExternalServiceConnection{
				Url:                   "ssh://thaddeus@gerrit.com:12345",
				RepositoryPathPattern: "{base}/{repo}",
				Repos:                 []string{"repos/repo1"},
			},
			urls: []urlToRepoNameErr{{"ssh://thaddeus@gerrit.com:12345/repos/repo1", "gerrit.com-12345/repos/repo1", nil}},
		},
		{
			conn: schema.OtherExternalServiceConnection{
				Url:                   "ssh://thaddeus@gerrit.com:12345",
				RepositoryPathPattern: "prettyhost/{repo}",
				Repos:                 []string{"repos/repo1"},
			},
			urls: []urlToRepoNameErr{{"ssh://thaddeus@gerrit.com:12345/repos/repo1", "prettyhost/repos/repo1", nil}},
		},
		{
			conn: schema.OtherExternalServiceConnection{
				Url:                   "ssh://thaddeus@gerrit.com:12345/repos",
				RepositoryPathPattern: "{repo}",
				Repos:                 []string{"repo1"},
			},
			urls: []urlToRepoNameErr{
				{"ssh://thaddeus@gerrit.com:12345/repos/repo1", "repo1", nil},
				{"ssh://thaddeus@asdf.com/repos/repo1", "", nil},
				{"ssh://thaddeus@gerrit.com:12345/asdf/repo1", "", nil},
			},
		},
	}

	for _, test := range tests {
		for _, u := range test.urls {
			repoName, err := Other{&test.conn}.CloneURLToRepoName(u.cloneURL)
			if u.err != nil {
				if !reflect.DeepEqual(u.err, err) {
					t.Errorf("expected error [%v], but got [%v] for clone URL %q (connection: %+v)", u.err, err, u.cloneURL, test.conn)
				}
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			if u.repoName != string(repoName) {
				t.Errorf("expected %q but got %q for clone URL %q (connection: %+v)", u.repoName, repoName, u.cloneURL, test.conn)
			}
		}
	}
}
