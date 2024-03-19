package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/google/go-github/v55/github"
	_ "github.com/mattn/go-sqlite3"
	"github.com/sourcegraph/conc/pool"
	"golang.org/x/oauth2"

	"github.com/sourcegraph/sourcegraph/dev/scaletesting/internal/store"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

type config struct {
	githubToken    string
	githubOrg      string
	githubURL      string
	githubUser     string
	githubPassword string

	count    int
	prefix   string
	resume   string
	retry    int
	insecure bool
}

type repo struct {
	*store.Repo
	blank *blankRepo
}

func main() {
	var cfg config

	flag.StringVar(&cfg.githubToken, "github.token", "", "(required) GitHub personal access token for the destination GHE instance")
	flag.StringVar(&cfg.githubURL, "github.url", "", "(required) GitHub base URL for the destination GHE instance")
	flag.StringVar(&cfg.githubOrg, "github.org", "", "(required) GitHub organization for the destination GHE instance to add the repos")
	flag.StringVar(&cfg.githubUser, "github.login", "", "(required) GitHub organization for the destination GHE instance to add the repos")
	flag.StringVar(&cfg.githubPassword, "github.password", "", "(required) GitHub organization for the destination GHE instance to add the repos")
	flag.IntVar(&cfg.count, "count", 100, "Amount of blank repos to create")
	flag.IntVar(&cfg.retry, "retry", 5, "Retries count")
	flag.StringVar(&cfg.prefix, "prefix", "repo", "Prefix to use when naming the repo, ex '[prefix]000042'")
	flag.StringVar(&cfg.resume, "resume", "state.db", "Temporary state to use to resume progress if interrupted")
	flag.BoolVar(&cfg.insecure, "insecure", false, "Accept invalid TLS certificates")

	flag.Parse()

	out := output.NewOutput(os.Stdout, output.OutputOpts{})

	ctx := context.Background()
	tc := oauth2.NewClient(ctx, oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: cfg.githubToken},
	))

	if cfg.insecure {
		tc.Transport.(*oauth2.Transport).Base = http.DefaultTransport
		tc.Transport.(*oauth2.Transport).Base.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	gh, err := github.NewClient(tc).WithEnterpriseURLs(cfg.githubURL, cfg.githubURL)
	if err != nil {
		writeFailure(out, "Failed to sign-in to GHE")
		log.Fatal(err)
	}

	if cfg.githubOrg == "" {
		writeFailure(out, "-github.org must be provided")
		flag.Usage()
		os.Exit(-1)
	}
	if cfg.githubURL == "" {
		writeFailure(out, "-github.URL must be provided")
		flag.Usage()
		os.Exit(-1)
	}
	if cfg.githubToken == "" {
		writeFailure(out, "-github.token must be provided")
		flag.Usage()
		os.Exit(-1)
	}
	if cfg.githubUser == "" {
		writeFailure(out, "-github.login must be provided")
		flag.Usage()
		os.Exit(-1)
	}
	if cfg.githubPassword == "" {
		writeFailure(out, "-github.password must be provided")
		flag.Usage()
		os.Exit(-1)
	}

	blank, err := newBlankRepo(cfg.githubUser, cfg.githubPassword)
	if err != nil {
		writeFailure(out, "Failed to create folder for repository")
		log.Fatal(err)
	}
	defer blank.teardown()
	err = blank.init(ctx)
	if err != nil {
		writeFailure(out, "Failed to initialize blank repository")
		log.Fatal(err)
	}

	state, err := store.New(cfg.resume)
	if err != nil {
		log.Fatal(err)
	}
	var storeRepos []*store.Repo
	if storeRepos, err = state.Load(); err != nil {
		log.Fatal(err)
	}

	if len(storeRepos) == 0 {
		storeRepos, err = generate(state, cfg)
		if err != nil {
			log.Fatal(err)
		}
		writeSuccess(out, "generated jobs in %s", cfg.resume)
	} else {
		writeSuccess(out, "resuming jobs from %s", cfg.resume)
	}

	// assign blank repo clones to avoid clogging the remotes
	blanks := []*blankRepo{}
	clonesCount := cfg.count / 100
	if clonesCount < 1 {
		clonesCount = 1
	}
	for i := range clonesCount {
		clone, err := blank.clone(ctx, i)
		if err != nil {
			log.Fatal(err)
		}
		defer clone.teardown()
		blanks = append(blanks, clone)
	}

	// Wrap repos from the store with ones having a blank repo attached.
	repos := make([]*repo, len(storeRepos))
	for i, r := range storeRepos {
		repos[i] = &repo{Repo: r}
	}

	// Distribute the blank repos.
	for i := range cfg.count {
		repos[i].blank = blanks[i%clonesCount]
	}

	if _, _, err := gh.Organizations.Get(ctx, cfg.githubOrg); err != nil {
		writeFailure(out, "organization does not exists")
		log.Fatal(err)
	}

	bars := []output.ProgressBar{
		{Label: "CreatingRepos", Max: float64(cfg.count)},
		{Label: "Adding remotes", Max: float64(cfg.count)},
		{Label: "Pushing branches", Max: float64(cfg.count)},
	}
	progress := out.Progress(bars, nil)

	p := pool.New().WithMaxGoroutines(20)
	var done int64
	for _, repo := range repos {
		repo := repo
		if repo.Created {
			atomic.AddInt64(&done, 1)
			progress.SetValue(0, float64(done))
			continue
		}
		p.Go(func() {
			newRepo, _, err := gh.Repositories.Create(ctx, cfg.githubOrg, &github.Repository{Name: github.String(repo.Name)})
			if err != nil {
				writeFailure(out, "Failed to create repository %s", repo.Name)
				repo.Failed = err.Error()
				if err := state.SaveRepo(repo.Repo); err != nil {
					log.Fatal(err)
				}
				return
			}
			repo.GitURL = newRepo.GetGitURL()
			repo.Created = true
			repo.Failed = ""
			if err = state.SaveRepo(repo.Repo); err != nil {
				log.Fatal(err)
			}

			atomic.AddInt64(&done, 1)
			progress.SetValue(0, float64(done))
		})
	}
	p.Wait()

	done = 0
	// Adding a remote will lock git configuration, so we shard
	// them by blank repo duplicates.
	p = pool.New().WithMaxGoroutines(20)
	for _, repo := range repos {
		repo := repo
		p.Go(func() {
			err = repo.blank.addRemote(ctx, repo.Name, repo.GitURL)
			if err != nil {
				writeFailure(out, "Failed to add remote to repository %s", repo.Name)
				log.Fatal(err)
			}
			atomic.AddInt64(&done, 1)
			progress.SetValue(1, float64(done))
		})
	}
	p.Wait()

	done = 0
	p = pool.New().WithMaxGoroutines(30)
	for _, repo := range repos {
		repo := repo
		if !repo.Created {
			atomic.AddInt64(&done, 1)
			progress.SetValue(2, float64(done))
			continue
		}
		if repo.Pushed {
			atomic.AddInt64(&done, 1)
			progress.SetValue(2, float64(done))
			continue
		}
		p.Go(func() {
			err := repo.blank.pushRemote(ctx, repo.Name, cfg.retry)
			if err != nil {
				writeFailure(out, "Failed to push to repository %s", repo.Name)
				repo.Failed = err.Error()
				if err := state.SaveRepo(repo.Repo); err != nil {
					log.Fatal(err)
				}
				return
			}
			repo.Pushed = true
			repo.Failed = ""
			if err := state.SaveRepo(repo.Repo); err != nil {
				log.Fatal(err)
			}
			atomic.AddInt64(&done, 1)
			progress.SetValue(2, float64(done))
		})
	}
	p.Wait()

	progress.Destroy()
	all, err := state.CountAllRepos()
	if err != nil {
		log.Fatal(err)
	}
	completed, err := state.CountCompletedRepos()
	if err != nil {
		log.Fatal(err)
	}

	writeSuccess(out, "Successfully added %d repositories on $GHE/%s (%d failures)", completed, cfg.githubOrg, all-completed)
}

func writeSuccess(out *output.Output, format string, a ...any) {
	out.WriteLine(output.Linef("✅", output.StyleSuccess, format, a...))
}

func writeFailure(out *output.Output, format string, a ...any) {
	out.WriteLine(output.Linef("❌", output.StyleFailure, format, a...))
}

func generateNames(prefix string, count int) []string {
	names := make([]string, count)
	for i := range count {
		names[i] = fmt.Sprintf("%s%09d", prefix, i)
	}
	return names
}

func generate(s *store.Store, cfg config) ([]*store.Repo, error) {
	names := generateNames(cfg.prefix, cfg.count)
	repos := make([]*store.Repo, 0, len(names))
	for _, name := range names {
		repos = append(repos, &store.Repo{Name: name})
	}

	if err := s.Insert(repos); err != nil {
		return nil, err
	}
	return s.Load()
}
