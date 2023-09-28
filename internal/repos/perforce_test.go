package repos

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/extsvc/perforce"
	"github.com/sourcegraph/sourcegraph/internal/testutil"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/internal/types/typestest"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/schema"
)

func setupMockP4Executable() (string, error) {
	// an inline shell script is pretty ugly,
	// but it has the advantage of keeping all the pieces in one place
	mockP4Script := `#!/usr/bin/env bash

### mock for p4 connection tests

### debug output in case more arguments are added so it's easy to see what is expected
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
echo "$@" >>${DIR}/p4.log

### handle "login -s" by outputting something that conforms to the expected output and exiting with success
[[ "${1}" = "login" ]] && [[ "${2}" = "-s" ]] && {
	echo "User ${P4USER:-admin} ticket expires in 130279 hours 20 minutes."
	exit 0
}

### handle "p4 depots" by returns some hard-coded depots. If the tests change, these need to change also
[[ "${3}" == "depots" ]] && {
	sg_depot='{"desc":"Created by admin.\n","map":"Sourcegraph/...","name":"Sourcegraph","time":"1628879609","type":"local"}'
	eng_depot='{"desc":"Created by admin.\n","map":"Engineering/...","name":"Engineering","time":"1628542108","type":"local"}'
	case "${5}" in
	Engineering) echo "${eng_depot}" ;;
	Sourcegraph) echo "${sg_depot}" ;;
	*)
		echo echo "${eng_depot}"
		echo "${sg_depot}"
		;;
	esac
	exit 0
}
	`
	tempdir, err := os.MkdirTemp("", "")
	if err != nil {
		return "", errors.Wrap(err, "setupMockP4Executable")
	}
	if err := os.WriteFile(filepath.Join(tempdir, "p4"), []byte(mockP4Script), 0755); err != nil {
		return "", errors.Wrap(err, "setupMockP4Executable")
	}
	os.Setenv("PATH", fmt.Sprintf("%s%c%s", tempdir, os.PathListSeparator, os.Getenv("PATH")))
	// return the temp directory so that it can be cleaned up later
	// becasue `defer` doesn't cross function call boundaries
	return tempdir, nil
}

func TestPerforceSource_ListRepos(t *testing.T) {
	// set up a mock p4 executable before running the tests
	tempdir, err := setupMockP4Executable()
	if err != nil {
		t.Errorf("error while setting up: %s", err)
	}
	defer os.RemoveAll(tempdir)
	assertAllReposListed := func(want []string) typestest.ReposAssertion {
		return func(t testing.TB, rs types.Repos) {
			t.Helper()

			have := rs.Names()
			sort.Strings(have)
			sort.Strings(want)

			if diff := cmp.Diff(want, have); diff != "" {
				t.Errorf("Mismatch (-want +got):\n%s", diff)
			}
		}
	}

	testCases := []struct {
		name   string
		assert typestest.ReposAssertion
		conf   *schema.PerforceConnection
		err    string
	}{
		{
			name: "list",
			assert: assertAllReposListed([]string{
				"Sourcegraph",
				"Engineering/Cloud",
			}),
			conf: &schema.PerforceConnection{
				P4Port:   "ssl:111.222.333.444:1666",
				P4User:   "admin",
				P4Passwd: "pa$$word",
				Depots: []string{
					"//Sourcegraph",
					"//Engineering/Cloud",
				},
			},
			err: "<nil>",
		},
	}

	for _, tc := range testCases {
		tc := tc
		tc.name = "PERFORCE-LIST-REPOS/" + tc.name
		t.Run(tc.name, func(t *testing.T) {
			svc := &types.ExternalService{
				Kind:   extsvc.KindPerforce,
				Config: extsvc.NewUnencryptedConfig(MarshalJSON(t, tc.conf)),
			}

			perforceSrc, err := newPerforceSource(svc, tc.conf)
			if err != nil {
				t.Fatal(err)
			}

			repos, err := ListAll(context.Background(), perforceSrc)

			if have, want := fmt.Sprint(err), tc.err; have != want {
				t.Errorf("error:\nhave: %q\nwant: %q", have, want)
			}

			if tc.assert != nil {
				tc.assert(t, repos)
			}
		})
	}
}

func TestPerforceSource_makeRepo(t *testing.T) {
	depots := []perforce.Depot{
		{
			Depot: "//Sourcegraph",
			Type:  perforce.Local,
		},
		{
			Depot: "//Engineering/Cloud",
			Type:  perforce.Stream,
		},
	}

	svc := types.ExternalService{
		ID:     1,
		Kind:   extsvc.KindPerforce,
		Config: extsvc.NewEmptyConfig(),
	}

	tests := []struct {
		name   string
		schema *schema.PerforceConnection
	}{
		{
			name: "simple",
			schema: &schema.PerforceConnection{
				P4Port:   "ssl:111.222.333.444:1666",
				P4User:   "admin",
				P4Passwd: "pa$$word",
			},
		}, {
			name: "path-pattern",
			schema: &schema.PerforceConnection{
				P4Port:                "ssl:111.222.333.444:1666",
				P4User:                "admin",
				P4Passwd:              "pa$$word",
				RepositoryPathPattern: "perforce/{depot}",
			},
		},
	}
	for _, test := range tests {
		test.name = "PerforceSource_makeRepo_" + test.name
		t.Run(test.name, func(t *testing.T) {
			s, err := newPerforceSource(&svc, test.schema)
			if err != nil {
				t.Fatal(err)
			}

			var got []*types.Repo
			for _, depot := range depots {
				got = append(got, s.makeRepo(depot))
			}

			testutil.AssertGolden(t, "testdata/golden/"+test.name, Update(test.name), got)
		})
	}
}
