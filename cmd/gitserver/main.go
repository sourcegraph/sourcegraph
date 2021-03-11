// gitserver is the gitserver server.
package main // import "github.com/sourcegraph/sourcegraph/cmd/gitserver"

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/inconshreveable/log15"
	jsoniter "github.com/json-iterator/go"
	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/server"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/database/dbconn"
	"github.com/sourcegraph/sourcegraph/internal/database/dbutil"
	"github.com/sourcegraph/sourcegraph/internal/debugserver"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
	"github.com/sourcegraph/sourcegraph/internal/jsonc"
	"github.com/sourcegraph/sourcegraph/internal/logging"
	"github.com/sourcegraph/sourcegraph/internal/profiler"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/trace/ot"
	"github.com/sourcegraph/sourcegraph/internal/tracer"
	"github.com/sourcegraph/sourcegraph/schema"
)

var (
	reposDir                     = env.Get("SRC_REPOS_DIR", "/data/repos", "Root dir containing repos.")
	wantPctFree                  = env.MustGetInt("SRC_REPOS_DESIRED_PERCENT_FREE", 10, "Target percentage of free space on disk.")
	janitorInterval              = env.MustGetDuration("SRC_REPOS_JANITOR_INTERVAL", 1*time.Minute, "Interval between cleanup runs")
	syncRepoStateInterval        = env.MustGetDuration("SRC_REPOS_SYNC_STATE_INTERVAL", 10*time.Minute, "Interval between state syncs")
	syncRepoStateBatchSize       = env.MustGetInt("SRC_REPOS_SYNC_STATE_BATCH_SIZE", 500, "Number of upserts to perform per batch")
	syncRepoStateUpsertPerSecond = env.MustGetInt("SRC_REPOS_SYNC_STATE_UPSERT_PER_SEC", 500, "The number of upserted rows allowed per second across all gitserver instances")
	envHostname                  = env.Get("HOSTNAME", "", "Hostname override")
)

func main() {
	env.Lock()
	env.HandleHelpFlag()

	if err := profiler.Init(); err != nil {
		log.Fatalf("failed to start profiler: %v", err)
	}

	logging.Init()
	tracer.Init()
	trace.Init(true)

	if reposDir == "" {
		log.Fatal("git-server: SRC_REPOS_DIR is required")
	}
	if err := os.MkdirAll(reposDir, os.ModePerm); err != nil {
		log.Fatalf("failed to create SRC_REPOS_DIR: %s", err)
	}

	wantPctFree2, err := getPercent(wantPctFree)
	if err != nil {
		log.Fatalf("SRC_REPOS_DESIRED_PERCENT_FREE is out of range: %v", err)
	}

	db, err := getDB()
	if err != nil {
		log.Fatalf("failed to initialize database stores: %v", err)
	}
	repoStore := database.Repos(db)
	externalServiceStore := database.ExternalServices(db)

	gitserver := server.Server{
		ReposDir:           reposDir,
		DesiredPercentFree: wantPctFree2,
		GetRemoteURLFunc: func(ctx context.Context, repo api.RepoName) (string, error) {
			r, err := repoStore.GetByName(ctx, repo)
			if err != nil {
				return "", err
			}
			for _, info := range r.Sources {
				return info.CloneURL, nil
			}
			return "", fmt.Errorf("no sources for %q", repo)
		},
		GetVCSSyncer: func(ctx context.Context, repo api.RepoName) (server.VCSSyncer, error) {
			r, err := repoStore.GetByName(ctx, repo)
			if err != nil {
				return nil, errors.Wrap(err, "get repository")
			}

			switch r.ExternalRepo.ServiceType {
			case extsvc.TypePerforce:
				// Extract options from external service config
				var c schema.PerforceConnection
				for _, info := range r.Sources {
					es, err := externalServiceStore.GetByID(ctx, info.ExternalServiceID())
					if err != nil {
						return nil, errors.Wrap(err, "get external service")
					}

					normalized, err := jsonc.Parse(es.Config)
					if err != nil {
						return nil, errors.Wrap(err, "normalize JSON")
					}

					if err = jsoniter.Unmarshal(normalized, &c); err != nil {
						return nil, errors.Wrap(err, "unmarshal JSON")
					}
					break
				}

				return &server.PerforceDepotSyncer{
					MaxChanges: int(c.MaxChanges),
				}, nil
			}
			return &server.GitRepoSyncer{}, nil
		},
		Hostname: hostnameBestEffort(),
		DB:       db,
	}
	gitserver.RegisterMetrics()

	if tmpDir, err := gitserver.SetupAndClearTmp(); err != nil {
		log.Fatalf("failed to setup temporary directory: %s", err)
	} else {
		// Additionally set TMP_DIR so other temporary files we may accidentally
		// create are on the faster RepoDir mount.
		os.Setenv("TMP_DIR", tmpDir)
	}

	// Create Handler now since it also initializes state
	handler := ot.Middleware(gitserver.Handler())

	go debugserver.Start()
	go gitserver.Janitor(janitorInterval)
	go gitserver.SyncRepoState(syncRepoStateInterval, syncRepoStateBatchSize, syncRepoStateUpsertPerSecond)

	port := "3178"
	host := ""
	if env.InsecureDev {
		host = "127.0.0.1"
	}
	addr := net.JoinHostPort(host, port)
	srv := &http.Server{
		Addr:    addr,
		Handler: handler,
	}
	log15.Info("git-server: listening", "addr", srv.Addr)

	go func() {
		err := srv.ListenAndServe()
		if err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Listen for shutdown signals. When we receive one attempt to clean up,
	// but do an insta-shutdown if we receive more than one signal.
	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGINT, syscall.SIGHUP)
	<-c
	go func() {
		<-c
		os.Exit(0)
	}()

	// Stop accepting requests. In the future we should use graceful shutdown.
	srv.Close()

	// The most important thing this does is kill all our clones. If we just
	// shutdown they will be orphaned and continue running.
	gitserver.Stop()
}

func hostnameBestEffort() string {
	if envHostname != "" {
		return envHostname
	}
	h, _ := os.Hostname()
	return h
}

func getPercent(p int) (int, error) {
	if p < 0 {
		return 0, fmt.Errorf("negative value given for percentage: %d", p)
	}
	if p > 100 {
		return 0, fmt.Errorf("excessively high value given for percentage: %d", p)
	}
	return p, nil
}

// getStores initializes a connection to the database and returns RepoStore and
// ExternalServiceStore.
func getDB() (dbutil.DB, error) {
	//
	// START FLAILING

	// Gitserver is an internal actor. We rely on the frontend to do authz
	// checks for user requests.
	authz.SetProviders(true, []authz.Provider{})

	// END FLAILING
	//

	dsn := conf.Get().ServiceConnections.PostgresDSN
	conf.Watch(func() {
		newDSN := conf.Get().ServiceConnections.PostgresDSN
		if dsn != newDSN {
			// The DSN was changed (e.g. by someone modifying the env vars on
			// the frontend). We need to respect the new DSN. Easiest way to do
			// that is to restart our service (kubernetes/docker/goreman will
			// handle starting us back up).
			log.Fatalf("Detected repository DSN change, restarting to take effect: %q", newDSN)
		}
	})

	return dbconn.New(dsn, "gitserver")
}
