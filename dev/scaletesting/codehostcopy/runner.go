package main

import (
	"context"
	"net/url"
	"os"

	"github.com/sourcegraph/run"
	"github.com/sourcegraph/sourcegraph/lib/group"
)

type Runner struct {
	source      CodeHostSource
	destination CodeHostDestination
}

func NewRunner(source CodeHostSource, dest CodeHostDestination) *Runner {
	return &Runner{
		source:      source,
		destination: dest,
	}
}

func (r *Runner) Run(ctx context.Context) error {
	srcRepos, err := r.source.ListRepos(ctx)
	if err != nil {
		return err
	}

	err = inTempFolder(func() error {
		g := group.NewWithResults[error]().WithMaxConcurrency(10)
		for _, repo := range srcRepos[21:30] {
			repo := repo
			g.Go(func() error {
				gitURL, err := r.destination.CreateRepo(ctx, repo.name)
				if err != nil {
					return err
				}

				err = uploadRepo(ctx, repo, gitURL)
				if err != nil {
					return err
				}
				return nil
			})
		}
		errs := g.Wait()
		for _, err := range errs {
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return nil
}

func uploadRepo(ctx context.Context, repo *Repo, gitURL *url.URL) error {
	err := run.Bash(ctx, "git clone", repo.url).Run().Stream(os.Stdout)
	if err != nil {
		return err
	}
	err = run.Bash(ctx, "git remote add destination", gitURL.String()).Dir(repo.name).Run().Stream(os.Stdout)
	if err != nil {
		return err
	}
	return run.Bash(ctx, "git push destination").Dir(repo.name).Run().Stream(os.Stdout)
}

func inTempFolder(f func() error) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	defer func() {
		_ = os.Chdir(cwd)
	}()

	path, err := os.MkdirTemp(os.TempDir(), "repo")
	if err != nil {
		return err
	}
	err = os.Chdir(path)
	if err != nil {
		return err
	}

	return f()
}
