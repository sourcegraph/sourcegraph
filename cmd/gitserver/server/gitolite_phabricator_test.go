package server

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/schema"
)

func TestServer_handleGet(t *testing.T) {
	conn := []*schema.GitoliteConnection{{
		Prefix: "mygitolite.host/",
		Host:   "git@mygitolite.host",
		Phabricator: &schema.Phabricator{
			CallsignCommand: `echo ${REPO} | tr a-z A-Z`,
			Url:             "https://phab.mycompany.com",
		},
	}}
	api.MockExternalServiceConfigs = func(kind string, result interface{}) error {
		buf, err := json.Marshal(conn)
		if err != nil {
			return err
		}
		return json.Unmarshal(buf, result)
	}
	t.Cleanup(func() { api.MockExternalServiceConfigs = nil })

	s := &Server{ReposDir: "/testroot"}
	h := s.Handler()

	cases := []struct {
		repo        string
		expMetadata string
	}{{
		repo:        "somerepo",
		expMetadata: `{"callsign":"SOMEREPO"}`,
	}, {
		repo:        "anotherrepo",
		expMetadata: `{"callsign":"ANOTHERREPO"}`,
	}}

	for _, testcase := range cases {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/getGitolitePhabricatorMetadata?gitolite=git@mygitolite.host&repo="+testcase.repo, nil)
		h.ServeHTTP(rr, req)

		result := strings.TrimSpace(rr.Body.String())
		if result != testcase.expMetadata {
			t.Errorf("for repo %q, expected metadata %q, but got %q", testcase.repo, testcase.expMetadata, result)
		}
	}
}

func TestServer_handleGet_invalid(t *testing.T) {
	conn := []*schema.GitoliteConnection{{
		Prefix: "mygitolite.host/",
		Host:   "git@mygitolite.host",
		Phabricator: &schema.Phabricator{
			CallsignCommand: `echo "Something went wrong this is not a valid callsign"`,
		},
	}}
	api.MockExternalServiceConfigs = func(kind string, result interface{}) error {
		buf, err := json.Marshal(conn)
		if err != nil {
			return err
		}
		return json.Unmarshal(buf, result)
	}
	t.Cleanup(func() { api.MockExternalServiceConfigs = nil })

	s := &Server{ReposDir: "/testroot"}
	h := s.Handler()

	cases := []struct {
		repo        string
		expMetadata string
	}{{
		repo:        "somerepo",
		expMetadata: `{"callsign":""}`,
	}, {
		repo:        "anotherrepo",
		expMetadata: `{"callsign":""}`,
	}}

	for _, testcase := range cases {
		rr := httptest.NewRecorder()
		req := httptest.NewRequest("GET", "/getGitolitePhabricatorMetadata?gitolite=git@mygitolite.host&repo="+testcase.repo, nil)
		h.ServeHTTP(rr, req)

		result := strings.TrimSpace(rr.Body.String())
		if result != testcase.expMetadata {
			t.Errorf("for repo %q, expected metadata %q, but got %q", testcase.repo, testcase.expMetadata, result)
		}
	}
}
