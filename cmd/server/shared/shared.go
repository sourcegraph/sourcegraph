// Package shared provides the entrypoint to Sourcegraph's single docker
// image. It has functionality to setup the shared environment variables, as
// well as create the Procfile for goreman to run.
package shared

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"

	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/goreman"
)

// FrontendInternalHost is the value of SRC_FRONTEND_INTERNAL.
const FrontendInternalHost = "127.0.0.1:3090"

// DefaultEnv is environment variables that will be set if not already set.
//
// If it is modified by an external package, it must be modified immediately on startup,
// before `shared.Main` is called.
var DefaultEnv = map[string]string{
	// Sourcegraph services running in this container
	"SRC_GIT_SERVERS":       "127.0.0.1:3178",
	"SEARCHER_URL":          "http://127.0.0.1:3181",
	"REPO_UPDATER_URL":      "http://127.0.0.1:3182",
	"QUERY_RUNNER_URL":      "http://127.0.0.1:3183",
	"SRC_SYNTECT_SERVER":    "http://127.0.0.1:9238",
	"SYMBOLS_URL":           "http://127.0.0.1:3184",
	"SRC_HTTP_ADDR":         ":8080",
	"SRC_HTTPS_ADDR":        ":8443",
	"SRC_FRONTEND_INTERNAL": FrontendInternalHost,
	"GITHUB_BASE_URL":       "http://127.0.0.1:3180", // points to github-proxy

	"GRAFANA_SERVER_URL": "http://127.0.0.1:3370",
	"JAEGER_SERVER_URL":  "http://127.0.0.1:16686",

	// Limit our cache size to 100GB, same as prod. We should probably update
	// searcher/symbols to ensure this value isn't larger than the volume for
	// CACHE_DIR.
	"SEARCHER_CACHE_SIZE_MB": "50000",
	"SYMBOLS_CACHE_SIZE_MB":  "50000",

	// Used to differentiate between deployments on dev, Docker, and Kubernetes.
	"DEPLOY_TYPE": "docker-container",

	// enables the debug proxy (/-/debug)
	"SRC_PROF_HTTP": "",

	"LOGO": "t",

	// TODO other bits
	// * DEBUG LOG_REQUESTS https://github.com/sourcegraph/sourcegraph/issues/8458
}

// Set verbosity based on simple interpretation of env var to avoid external dependencies (such as
// on github.com/sourcegraph/sourcegraph/internal/env).
var verbose = os.Getenv("SRC_LOG_LEVEL") == "dbug" || os.Getenv("SRC_LOG_LEVEL") == "info"

// Main is the main server command function which is shared between Sourcegraph
// server's open-source and enterprise variant.
func Main() {
	flag.Parse()
	log.SetFlags(0)

	// Ensure CONFIG_DIR and DATA_DIR

	// Load $CONFIG_DIR/env before we set any defaults
	{
		configDir := SetDefaultEnv("CONFIG_DIR", "/etc/sourcegraph")
		err := os.MkdirAll(configDir, 0755)
		if err != nil {
			log.Fatalf("failed to ensure CONFIG_DIR exists: %s", err)
		}

		err = godotenv.Load(filepath.Join(configDir, "env"))
		if err != nil && !os.IsNotExist(err) {
			log.Fatalf("failed to load %s: %s", filepath.Join(configDir, "env"), err)
		}
	}

	// Next persistence
	{
		SetDefaultEnv("SRC_REPOS_DIR", filepath.Join(DataDir, "repos"))
		SetDefaultEnv("CACHE_DIR", filepath.Join(DataDir, "cache"))
	}

	// Special case some convenience environment variables
	if redis, ok := os.LookupEnv("REDIS"); ok {
		SetDefaultEnv("REDIS_ENDPOINT", redis)
	}

	data, err := json.MarshalIndent(SrcProfServices, "", "  ")
	if err != nil {
		log.Println("Failed to marshal default SRC_PROF_SERVICES")
	} else {
		SetDefaultEnv("SRC_PROF_SERVICES", string(data))
	}

	for k, v := range DefaultEnv {
		SetDefaultEnv(k, v)
	}

	// Now we put things in the right place on the FS
	if err := copySSH(); err != nil {
		// TODO There are likely several cases where we don't need SSH
		// working, we shouldn't prevent setup in those cases. The main one
		// that comes to mind is an ORIGIN_MAP which creates https clone URLs.
		log.Println("Failed to setup SSH authorization:", err)
		log.Fatal("SSH authorization required for cloning from your codehost. Please see README.")
	}
	if err := copyConfigs(); err != nil {
		log.Fatal("Failed to copy configs:", err)
	}

	// TODO validate known_hosts contains all code hosts in config.

	nginx, err := nginxProcFile()
	if err != nil {
		log.Fatal("Failed to setup nginx:", err)
	}

	postgresExporterLine := fmt.Sprintf(`postgres_exporter: env DATA_SOURCE_NAME="%s" postgres_exporter --log.level=%s`, dbutil.PostgresDSN("", "postgres", os.Getenv), convertLogLevel(os.Getenv("SRC_LOG_LEVEL")))

	procfile := []string{
		nginx,
		`frontend: env CONFIGURATION_MODE=server frontend`,
		`gitserver: gitserver`,
		`query-runner: query-runner`,
		`symbols: symbols`,
		`searcher: searcher`,
		`github-proxy: github-proxy`,
		`repo-updater: repo-updater`,
		`syntect_server: sh -c 'env QUIET=true ROCKET_ENV=production ROCKET_PORT=9238 ROCKET_LIMITS='"'"'{json=10485760}'"'"' ROCKET_SECRET_KEY='"'"'SeerutKeyIsI7releuantAndknvsuZPluaseIgnorYA='"'"' ROCKET_KEEP_ALIVE=0 ROCKET_ADDRESS='"'"'"127.0.0.1"'"'"' syntect_server | grep -v "Rocket has launched" | grep -v "Warning: environment is"' | grep -v 'Configured for production'`,
		postgresExporterLine,
	}
	procfile = append(procfile, ProcfileAdditions...)

	monitoringLines, err := maybeMonitoring()
	if err != nil {
		log.Fatal(err)
	}
	if len(monitoringLines) != 0 {
		procfile = append(procfile, monitoringLines...)
	}

	if minioLines := maybeMinio(); len(minioLines) != 0 {
		procfile = append(procfile, minioLines...)
	}

	redisStoreLine, err := maybeRedisStoreProcFile()
	if err != nil {
		log.Fatal(err)
	}
	if redisStoreLine != "" {
		procfile = append(procfile, redisStoreLine)
	}
	redisCacheLine, err := maybeRedisCacheProcFile()
	if err != nil {
		log.Fatal(err)
	}
	if redisCacheLine != "" {
		procfile = append(procfile, redisCacheLine)
	}

	postgresLine, err := maybePostgresProcFile()
	if err != nil {
		log.Fatal(err)
	}
	if postgresLine != "" {
		procfile = append(procfile, postgresLine)
	}

	procfile = append(procfile, maybeZoektProcFile()...)

	// Shutdown if any process dies
	procDiedAction := goreman.Shutdown
	if ignore, _ := strconv.ParseBool(os.Getenv("IGNORE_PROCESS_DEATH")); ignore {
		// IGNORE_PROCESS_DEATH is an escape hatch so that sourcegraph/server
		// keeps running in the case of a subprocess dying on startup. An
		// example use case is connecting to postgres even though frontend is
		// dying due to a bad migration.
		procDiedAction = goreman.Ignore
	}

	// If in restore mode, only run PostgreSQL
	if restore, _ := strconv.ParseBool(os.Getenv("PGRESTORE")); restore {
		procfile = []string{}
		procfile = append(procfile, postgresLine)
	}

	err = goreman.Start([]byte(strings.Join(procfile, "\n")), goreman.Options{
		RPCAddr:        "127.0.0.1:5005",
		ProcDiedAction: procDiedAction,
	})
	if err != nil {
		log.Fatal(err)
	}
}
