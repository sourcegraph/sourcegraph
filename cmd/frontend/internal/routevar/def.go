package routevar

// DefAtRev refers to a def at a non-absolute commit ID (unlike
// DefSpec/DefKey, which require the CommitID field to have an
// absolute commit ID).
type DefAtRev struct {
	RepoRev
	Unit, UnitType, Path string
}

// Def captures def paths in URL routes.
const Def = "{UnitType}/{Unit:.+?}/-/{Path:.*?}"

func defURLPathToKeyPath(s string) string {
	if s == "_._" {
		return "."
	}
	return s
}

func DefRouteVars(s DefAtRev) map[string]string {
	m := RepoRevRouteVars(s.RepoRev)
	m["UnitType"] = s.UnitType
	m["Unit"] = s.Unit
	m["Path"] = s.Path
	return m
}

func ToDefAtRev(routeVars map[string]string) DefAtRev {
	return DefAtRev{
		RepoRev:  ToRepoRev(routeVars),
		UnitType: routeVars["UnitType"],
		Unit:     defURLPathToKeyPath(routeVars["Unit"]),
		Path:     defURLPathToKeyPath(pathUnescape(routeVars["Path"])),
	}
}
