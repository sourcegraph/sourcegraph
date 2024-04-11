package proxy

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/sourcegraph/conc/pool"

	v1 "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// optimizationScheduler should be wrapped in a periodic goroutine. It could also be triggered
// from a debugserver call or a proper gRPC call, if we want to expose it internally
// for debugging.
type optimizationScheduler struct {
	locator Locator
	store   RepoLookupStore
	cs      ClientSource
	mu      sync.Mutex
}

func (r *optimizationScheduler) ReconcileSingleflight(ctx context.Context) error {
	if !r.mu.TryLock() {
		return errors.New("optimizationScheduler is already in progress")
	}
	defer r.mu.Unlock()
	return r.Reconcile(ctx)
}

const optimizationConcurrency = 5

func (r *optimizationScheduler) Reconcile(ctx context.Context) error {
	repos, err := r.allRepos(ctx)
	if err != nil {
		return err
	}

	var errs error
	var errsMu sync.Mutex

	// TODO: Maybe we should group this by shard and make sure each shard always
	// has N active jobs at most?
	p := pool.New().WithContext(ctx)
	reposCh := make(chan ListRepo)

	for range optimizationConcurrency {
		p.Go(func(ctx context.Context) error {
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case repo, ok := <-reposCh:
					if !ok {
						return nil
					}
					// We only want to optimize repos every N minutes.
					// The list of repos we receive here is ordered by
					// LastOptimizationAt ASC, so when this repo isn't due yet,
					// all remaining onces won't be due either. In that case, we
					// can return early here.
					// TODO: Made the interval configurable.
					if time.Since(repo.LastOptimizationAt) < time.Minute {
						// TODO: Need to make sure the producer routine ends properly.
						return nil
					}
					err := r.optimizeRepo(ctx, repo)
					// Collect errors so we can reconcile as many repos as possible.
					if err != nil {
						errsMu.Lock()
						errs = errors.Append(errs, err)
						errsMu.Unlock()
					}
				}
			}
		})
	}

	// Enqueuer goroutine.
	p.Go(func(ctx context.Context) error {
		defer close(reposCh)

		for _, repo := range repos {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case reposCh <- repo:
			}
		}
		return nil
	})

	if err := p.Wait(); err != nil {
		return err
	}

	return errs
}

// TODO: Evaluate how costly this will be for an instance like dotcom.
// We will likely want to compile this list somewhat frequently.
func (r *optimizationScheduler) allRepos(ctx context.Context) ([]ListRepo, error) {
	wanted := []ListRepo{}
	var nextPage string
	for {
		page, cur, err := r.store.ListRepos(ctx, nextPage)
		if err != nil {
			return nil, err
		}
		nextPage = cur
		for _, r := range page {
			if !r.DeleteAfter.IsZero() {
				// No need to schedule repos for optimization that are deleted
				// soon anyways.
				continue
			}
			wanted = append(wanted, r)
		}
		if nextPage == "" {
			// We sort the slice by LastOptimizationAt ASC:
			sort.Slice(wanted, func(i, j int) bool {
				// TODO: Make sure this is correct and works for zerotimes as well.
				return wanted[i].LastOptimizationAt.Before(wanted[j].LastOptimizationAt)
			})
			return wanted, nil
		}
	}
}

func (r *optimizationScheduler) optimizeRepo(ctx context.Context, lr ListRepo) error {
	cc, repo, err := r.locator.Locate(ctx, &v1.GitserverRepository{Uid: lr.UID})
	if err != nil {
		return err
	}

	// TODO: Call optimizerepo here once it exists.
	_, err = cc.RepoUpdate(ctx, &v1.RepoUpdateRequest{
		Repo: repo,
	})

	// TODO need to update the store with LastOptimizationAt here.
	if err := r.store.SetLastOptimization(ctx, lr.UID, time.Now()); err != nil {
		return errors.Wrap(err, "failed to set last optimization time")
	}

	// TODO Call this here to reflect potential size reductions post-optimization.
	// // SetRepoSize will attempt to update ONLY the repo size of a GitServerRepo.
	// // If the size value hasn't changed, the row will not be updated.
	// SetRepoSize(ctx context.Context, name api.RepoName, size int64, shardID string) error

	return err
}
