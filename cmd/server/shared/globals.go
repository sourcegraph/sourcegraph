package shared

// This file contains global variables that can be modified in a limited fashion by an external
// package (e.g., the enterprise package).

// SrcProfServices defines the default value for SRC_PROF_SERVICES.
//
// If it is modified by an external package, it must be modified immediately on startup, before
// `shared.Main` is called.
//
// The same data is currently reflected in the following (and should be kept in-sync):
//   - the SRC_PROF_SERVICES envvar when using sg
//   - the file dev/src-prof-services.json when using by using `sg start`
var SrcProfServices = []map[string]string{
	{"Name": "frontend", "Host": "127.0.0.1:6063"},
	{"Name": "gitserver", "Host": "127.0.0.1:6068"},
	{"Name": "searcher", "Host": "127.0.0.1:6069"},
	{"Name": "symbols", "Host": "127.0.0.1:6071"},
	{"Name": "repo-updater", "Host": "127.0.0.1:6074"},
	{"Name": "worker", "Host": "127.0.0.1:6089"},
	{"Name": "precise-code-intel-worker", "Host": "127.0.0.1:6088"},
	// no executors in server image
	{"Name": "zoekt-indexserver", "Host": "127.0.0.1:6072"},
	{"Name": "zoekt-webserver", "Host": "127.0.0.1:3070", "DefaultPath": "/debug/requests/"},
}

// ProcfileAdditions is a list of Procfile lines that should be added to the emitted Procfile that
// defines the services configuration.
//
// If it is modified by an external package, it must be modified immediately on startup, before
// `shared.Main` is called.
var ProcfileAdditions []string

// DataDir is the root directory for storing persistent data. It should NOT be modified by any
// external package.
var DataDir = SetDefaultEnv("DATA_DIR", "/var/opt/sourcegraph")

var AllowSingleDockerCodeInsights bool
