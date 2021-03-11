package routevar

import (
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"testing"

	"github.com/gorilla/mux"
)

func TestRepoPattern(t *testing.T) {
	pat, err := regexp.Compile("^" + RepoPattern + "$")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		input     string
		wantMatch bool
	}{
		{"foo", true},
		{"foo/bar", true},
		{"foo.com/bar", true},
		{"foo.com/-bar", true},
		{"foo.com/-bar-", true},
		{"foo.com/bar-", true},
		{"foo.com/.bar", true},
		{"foo.com/bar.baz", true},
		{"fo_o.com/bar", true},
		{".foo", true},
		{"./foo", true},

		{"", false},
		{"/foo", false},
		{"foo/", false},
		{"/foo/", false},
		{"foo.com/-", false},
		{"foo.com/-/bar", false},
		{"-/bar", false},
		{"/-/bar", false},
		{"bar@a", false},
		{"bar@a/b", false},
	}
	for _, test := range tests {
		match := pat.MatchString(test.input)
		if match != test.wantMatch {
			t.Errorf("%q: got match == %v, want %v", test.input, match, test.wantMatch)
		}

		repo, err := ParseRepo(test.input)
		if gotErr, wantErr := err != nil, !test.wantMatch; gotErr != wantErr {
			t.Errorf("%q: got err == %v, want error? == %v", test.input, err, wantErr)
		}
		if err == nil {
			if string(repo) != test.input {
				t.Errorf("%q: got repo == %q, want %q", test.input, repo, test.input)
			}
		}
	}
}

func TestRevPattern(t *testing.T) {
	pat, err := regexp.Compile("^" + RevPattern + "$")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		input     string
		wantMatch bool
	}{
		{"v", true},
		{"v/v", true},
		{"my/branch/name", true},
		{"bar~10", true},
		{"bar^10", true},

		{"-", false},
		{"v/-", false},
		{"v/-/v", false},
		{"-/v", false},
	}
	for _, test := range tests {
		match := pat.MatchString(test.input)
		if match != test.wantMatch {
			t.Errorf("%q: got match == %v, want %v", test.input, match, test.wantMatch)
		}
	}
}

func TestRepo(t *testing.T) {
	r := mux.NewRouter()
	r.Path("/" + Repo)

	tests := []struct {
		path        string
		wantNoMatch bool
		wantVars    map[string]string
	}{
		{path: "/foo", wantVars: map[string]string{"Repo": "foo"}},
		{path: "/foo.com/bar", wantVars: map[string]string{"Repo": "foo.com/bar"}},

		{path: "/foo.com/bar/-/abc/def", wantNoMatch: true},
		{path: "/foo.com/bar@a", wantNoMatch: true},
		{path: "/foo.com/bar@a/b", wantNoMatch: true},
		{path: "/foo.com/bar/@a", wantNoMatch: true},
		{path: "/-/foo.com/bar", wantNoMatch: true},
		{path: "/", wantNoMatch: true},
		{path: "/-/", wantNoMatch: true},
	}
	for _, test := range tests {
		var m mux.RouteMatch
		ok := r.Match(&http.Request{Method: "GET", URL: &url.URL{Path: test.path}}, &m)
		if ok == test.wantNoMatch {
			t.Errorf("%q: got match == %v, want %v", test.path, ok, !test.wantNoMatch)
		}
		if ok {
			if !reflect.DeepEqual(m.Vars, test.wantVars) {
				t.Errorf("%q: got vars == %v, want %v", test.path, m.Vars, test.wantVars)
			}

			url, err := m.Route.URLPath(pairs(m.Vars)...)
			if err != nil {
				t.Errorf("%q: URLPath: %s", test.path, err)
				continue
			}
			if url.Path != test.path {
				t.Errorf("%q: got path == %q, want %q", test.path, url.Path, test.path)
			}
		}
	}
}

func TestRev(t *testing.T) {
	r := mux.NewRouter()
	r.Path("/" + Rev)

	tests := []struct {
		path        string
		wantNoMatch bool
		wantVars    map[string]string
	}{
		{path: "/v", wantVars: map[string]string{"Rev": "v"}},
		{path: "/v/v/v", wantVars: map[string]string{"Rev": "v/v/v"}},

		{path: "", wantNoMatch: true},
		{path: "/", wantNoMatch: true},
	}
	for _, test := range tests {
		var m mux.RouteMatch
		ok := r.Match(&http.Request{Method: "GET", URL: &url.URL{Path: test.path}}, &m)
		if ok == test.wantNoMatch {
			t.Errorf("%q: got match == %v, want %v", test.path, ok, !test.wantNoMatch)
		}
		if ok {
			if !reflect.DeepEqual(m.Vars, test.wantVars) {
				t.Errorf("%q: got vars == %v, want %v", test.path, m.Vars, test.wantVars)
			}

			url, err := m.Route.URLPath(pairs(m.Vars)...)
			if err != nil {
				t.Errorf("%q: URLPath: %s", test.path, err)
				continue
			}
			if url.Path != test.path {
				t.Errorf("%q: got path == %q, want %q", test.path, url.Path, test.path)
			}
		}
	}
}

func TestRepoRevSpec(t *testing.T) {
	tests := []struct {
		spec      RepoRev
		routeVars map[string]string
	}{
		{RepoRev{Repo: "a.com/x", Rev: "r"}, map[string]string{"Repo": "a.com/x", "Rev": "@r"}},
		{RepoRev{Repo: "x", Rev: "r"}, map[string]string{"Repo": "x", "Rev": "@r"}},
	}

	for _, test := range tests {
		routeVars := RepoRevRouteVars(test.spec)
		if !reflect.DeepEqual(routeVars, test.routeVars) {
			t.Errorf("got route vars %+v, want %+v", routeVars, test.routeVars)
		}
		spec := ToRepoRev(routeVars)
		if spec != test.spec {
			t.Errorf("got spec %+v from route vars %+v, want %+v", spec, routeVars, test.spec)
		}
	}
}
