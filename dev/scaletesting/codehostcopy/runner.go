package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"

	"github.com/sourcegraph/conc/pool"
	"github.com/sourcegraph/log"
	"github.com/sourcegraph/run"

	"github.com/sourcegraph/sourcegraph/dev/scaletesting/internal/store"
	"github.com/sourcegraph/sourcegraph/lib/output"
)

const Unlimited = 0

type Runner struct {
	source      CodeHostSource
	destination CodeHostDestination
	store       *store.Store
	logger      log.Logger
}

// GitOpt is an option which changes the git command that gets invoked
type GitOpt func(cmd *run.Command) *run.Command

func logRepo(r *store.Repo, fields ...log.Field) []log.Field {
	return append([]log.Field{
		log.Object("repo",
			log.String("name", r.Name),
			log.String("from", r.GitURL),
			log.String("to", r.ToGitURL),
		),
	}, fields...)
}

func NewRunner(logger log.Logger, s *store.Store, source CodeHostSource, dest CodeHostDestination) *Runner {
	return &Runner{
		logger:      logger,
		source:      source,
		destination: dest,
		store:       s,
	}
}

func (r *Runner) addSSHKey(ctx context.Context) (func(), error) {
	// Add SSH Key to source and dest
	srcKey, err := r.source.AddSSHKey(ctx)
	if err != nil {
		return nil, err
	}

	destKey, err := r.destination.AddSSHKey(ctx)
	if err != nil {
		// Have to remove the source since it was added earlier
		r.source.DropSSHKey(ctx, srcKey)
		return nil, err
	}

	// create a func that cleans the ssh keys up when called
	return func() {
		r.source.DropSSHKey(ctx, srcKey)
		r.destination.DropSSHKey(ctx, destKey)
	}, nil
}

func (r *Runner) List(ctx context.Context, limit int) error {
	out := output.NewOutput(os.Stdout, output.OutputOpts{})

	// Load existing repositories.
	srcRepos, err := r.store.Load()
	if err != nil {
		r.logger.Error("failed to open state database", log.Error(err))
		return err
	}
	loadedFromDB := true

	// If we're starting fresh, really fetch them.
	if len(srcRepos) == 0 {
		loadedFromDB = false
		r.logger.Info("No existing state found, creating ...")
		out.WriteLine(output.Line(output.EmojiHourglass, output.StyleBold, "Listing repos"))

		var repos []*store.Repo
		repoIter := r.source.Iterator()
		for !repoIter.Done() && repoIter.Err() == nil {
			repos = append(repos, repoIter.Next(ctx)...)
			if limit != Unlimited && len(repos) >= limit {
				break
			}
		}

		if repoIter.Err() != nil {
			r.logger.Error("failed to list repositories from source", log.Error(err))
			return err
		}
		srcRepos = repos
		if err := r.store.Insert(repos); err != nil {
			r.logger.Error("failed to insert repositories from source", log.Error(err))
			return err
		}
	}
	block := out.Block(output.Line(output.EmojiInfo, output.StyleBold, fmt.Sprintf("List of repos (db: %v limit: %d total: %d)", loadedFromDB, limit, len(srcRepos))))
	if limit != 0 && limit < len(srcRepos) {
		srcRepos = srcRepos[:limit]
	}
	for _, r := range srcRepos {
		block.Writef("Name: %s Created: %v Pushed: %v GitURL: %s ToGitURL: %s Failed: %s", r.Name, r.Created, r.Pushed, r.GitURL, r.ToGitURL, r.Failed)
	}
	block.Close()
	return nil
}

func (r *Runner) Copy(ctx context.Context, concurrency int) error {
	out := output.NewOutput(os.Stdout, output.OutputOpts{})

	out.WriteLine(output.Line(output.EmojiInfo, output.StyleGrey, "Adding codehost ssh key"))
	cleanup, err := r.addSSHKey(ctx)
	if err != nil {
		return err
	}

	pruneKeys := func() {
		out.WriteLine(output.Line(output.EmojiInfo, output.StyleGrey, "Removing codehost ssh key"))
		cleanup()
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		pruneKeys()
		os.Exit(1)
	}()
	defer pruneKeys()

	// Load existing repositories.
	srcRepos, err := r.store.Load()
	if err != nil {
		r.logger.Error("failed to open state database", log.Error(err))
		return err
	}

	t, remainder, err := r.source.InitializeFromState(ctx, srcRepos)
	if err != nil {
		r.logger.Fatal(err.Error())
	}

	r.logger.Info(fmt.Sprintf("%d repositories processed, %d repositories left", len(srcRepos), remainder))

	bars := []output.ProgressBar{
		{Label: "Copying repos", Max: float64(t)},
	}
	progress := out.Progress(bars, nil)
	defer progress.Destroy()
	var done int64

	p := pool.NewWithResults[error]().WithMaxGoroutines(concurrency)

	repoIter := r.source.Iterator()
	for !repoIter.Done() && repoIter.Err() == nil {
		repos := repoIter.Next(ctx)
		if err = r.store.Insert(repos); err != nil {
			r.logger.Error("failed to insert repositories from source", log.Error(err))
		}

		for _, rr := range repos {
			repo := rr
			p.Go(func() error {
				// Create the repo on destination.
				if !repo.Created {
					toGitURL, err := r.destination.CreateRepo(ctx, repo.Name)
					if err != nil {
						repo.Failed = err.Error()
						r.logger.Error("failed to create repo", logRepo(repo, log.Error(err))...)
					} else {
						repo.ToGitURL = toGitURL.String()
						repo.Created = true
						// If we resumed and this repo previously failed, we need to clear the failed status as it succeeded now
						repo.Failed = ""
					}
					if err = r.store.SaveRepo(repo); err != nil {
						r.logger.Error("failed to save repo", logRepo(repo, log.Error(err))...)
						return err
					}
				}

				// Push the repo on destination.
				if !repo.Pushed && repo.Created {
					err := pushRepo(ctx, repo, r.source.GitOpts(), r.destination.GitOpts())
					if err != nil {
						repo.Failed = err.Error()
						r.logger.Error("failed to push repo", logRepo(repo, log.Error(err))...)
						println()
					} else {
						repo.Pushed = true
					}
					if err = r.store.SaveRepo(repo); err != nil {
						r.logger.Error("failed to save repo", logRepo(repo, log.Error(err))...)
						return err
					}
				}
				atomic.AddInt64(&done, 1)
				progress.SetValue(0, float64(done))
				progress.SetLabel(0, fmt.Sprintf("Copying repos (%d/%d)", done, t))
				return nil
			})
		}
	}

	if repoIter.Err() != nil {
		return repoIter.Err()
	}

	errs := p.Wait()
	for _, e := range errs {
		if e != nil {
			return e
		}
	}
	return nil
}

func pushRepo(ctx context.Context, repo *store.Repo, srcOpts []GitOpt, destOpts []GitOpt) error {
	// Handle the testing case
	if strings.HasPrefix(repo.ToGitURL, "https://dummy.local") {
		return nil
	}

	tmpDir, err := os.MkdirTemp(os.TempDir(), fmt.Sprintf("repo__%s", repo.Name))
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
	}()

	// we add the repo name so that we ensure we cd to the right repo directory
	// if we don't do this, there is no guarantee that the repo name and the git url are the same
	cmd := run.Bash(ctx, "git clone --bare", repo.GitURL, repo.Name).Dir(tmpDir)
	for _, opt := range srcOpts {
		cmd = opt(cmd)
	}
	err = cmd.Run().Wait()
	if err != nil {
		return err
	}
	repoDir := filepath.Join(tmpDir, repo.Name)
	cmd = run.Bash(ctx, "git remote set-url origin", repo.ToGitURL).Dir(repoDir)
	for _, opt := range destOpts {
		cmd = opt(cmd)
	}
	err = cmd.Run().Wait()
	if err != nil {
		return err
	}
	return gitPushWithRetry(ctx, repoDir, 3, destOpts...)
}

func gitPushWithRetry(ctx context.Context, dir string, retry int, destOpts ...GitOpt) error {
	var err error
	for range retry {
		// --force, with mirror we want the remote to look exactly as we have it
		cmd := run.Bash(ctx, "git push --mirror --force origin").Dir(dir)
		for _, opt := range destOpts {
			cmd = opt(cmd)
		}
		err = cmd.Run().Wait()
		if err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "timed out") || strings.Contains(errStr, "502") {
				continue
			}
			return err
		}
		break
	}
	return nil
}
