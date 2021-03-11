package routevar

import (
	"reflect"
	"testing"
)

func TestDefRouteVars(t *testing.T) {
	tests := []struct {
		def       DefAtRev
		routeVars map[string]string
	}{
		{
			DefAtRev{RepoRev: RepoRev{Repo: "r", Rev: ""}, UnitType: "t", Unit: "u", Path: "p"},
			map[string]string{"Repo": "r", "Rev": "", "UnitType": "t", "Unit": "u", "Path": "p"},
		},
		{
			DefAtRev{RepoRev: RepoRev{Repo: "r", Rev: "v"}, UnitType: "t", Unit: "u", Path: "p"},
			map[string]string{"Repo": "r", "Rev": "@v", "UnitType": "t", "Unit": "u", "Path": "p"},
		},
		{
			DefAtRev{RepoRev: RepoRev{Repo: "r", Rev: "v"}, UnitType: "t", Unit: "u1/u2/u3", Path: "p1/p2/p3"},
			map[string]string{"Repo": "r", "Rev": "@v", "UnitType": "t", "Unit": "u1/u2/u3", "Path": "p1/p2/p3"},
		},
	}
	for _, test := range tests {
		v := DefRouteVars(test.def)
		if !reflect.DeepEqual(v, test.routeVars) {
			t.Errorf("%v: got %+v, want %+v", test.def, v, test.routeVars)
		}

		def := ToDefAtRev(test.routeVars)
		if def != test.def {
			t.Errorf("%v: got %+v, want %+v", test.routeVars, def, test.def)
		}
	}
}
