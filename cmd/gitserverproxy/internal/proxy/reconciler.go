package proxy

import (
	"context"
	"math/rand"
	"sync"

	v1 "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// reconciler should be wrapped in a periodic goroutine. It could also be triggered
// from a debugserver call or a proper gRPC call, if we want to expose it internally
// for debugging.
// Reconciler takes the list of desired repos on disk plus the list of repos on
// all the shards and diffs them.
// Repos that are not on disk yet will be enqueued
// for cloning.
// Repos that are on disk but not part of the desired set will be deleted after
// some TTL has passed.
type reconciler struct {
	locator Locator
	store   RepoLookupStore
	cs      ClientSource
	mu      sync.Mutex
}

func (r *reconciler) ReconcileSingleflight(ctx context.Context) error {
	if !r.mu.TryLock() {
		return errors.New("reconciler is already in progress")
	}
	defer r.mu.Unlock()
	return r.Reconcile(ctx)
}

func (r *reconciler) Reconcile(ctx context.Context) error {
	// TODO: Consider using this for deletions:
	// limiter := ratelimit.NewInstrumentedLimiter("PurgeRepoWorker", rate.NewLimiter(10, 1))
	// TODO: Consider respecting this configuration.
	// purgeConfig := conf.SiteConfig().RepoPurgeWorker
	// if purgeConfig == nil {
	// 	purgeConfig = &schema.RepoPurgeWorker{
	// 		// Defaults - align with documentation
	// 		IntervalMinutes:   15,
	// 		DeletedTTLMinutes: 60,
	// 	}
	// }
	// if purgeConfig.IntervalMinutes <= 0 {
	// 	logger.Debug("purge worker disabled via site config", log.Int("repoPurgeWorker.interval", purgeConfig.IntervalMinutes))
	// 	return nil
	// }

	// TODO: Reconciler should do an immediate run when these settings change:
	// var previousAddrs string
	// var previousPinned string
	// fullSync := true

	// return goroutine.NewPeriodicGoroutine(
	// 	actor.WithInternalActor(ctx),
	// 	goroutine.HandlerFunc(func(ctx context.Context) error {
	// 		gitServerAddrs := gitserver.NewGitserverAddresses(conf.Get())
	// 		addrs := gitServerAddrs.Addresses
	// 		// We turn addrs into a string here for easy comparison and storage of previous
	// 		// addresses since we'd need to take a copy of the slice anyway.
	// 		currentAddrs := strings.Join(addrs, ",")
	// 		// If the addresses changed, we need to do a full sync.
	// 		fullSync = fullSync || currentAddrs != previousAddrs
	// 		previousAddrs = currentAddrs

	// 		// We turn PinnedServers into a string here for easy comparison and storage
	// 		// of previous pins.
	// 		pinnedServerPairs := make([]string, 0, len(gitServerAddrs.PinnedServers))
	// 		for k, v := range gitServerAddrs.PinnedServers {
	// 			pinnedServerPairs = append(pinnedServerPairs, fmt.Sprintf("%s=%s", k, v))
	// 		}
	// 		sort.Strings(pinnedServerPairs)
	// 		currentPinned := strings.Join(pinnedServerPairs, ",")
	// 		// If the pinned repos changed, we need to do a full sync.
	// 		fullSync = fullSync || currentPinned != previousPinned
	// 		previousPinned = currentPinned

	wanted, err := r.wantedRepos(ctx)
	if err != nil {
		return err
	}

	have, err := r.haveRepos(ctx)
	if err != nil {
		return err
	}

	// Compare wanted vs have repos and act accordingly
	wantedMap := make(map[string]ListRepo)
	for _, r := range wanted {
		wantedMap[r.UID] = r
	}
	// TODO: haveMap should be a map[string][]*v1.ListRepo and ListRepo should
	// denote the shard a repo is on.
	// This will let us make sure that repos are only ever on one shard for now.
	// We should then elect the leader shard and remove all other clones.
	haveMap := make(map[string]*v1.ListRepo)
	for _, r := range have {
		haveMap[r.GetRepo().GetUid()] = r
	}
	cloned := make([]string, len(wanted)/2)

	var errs error

	// First, check that all repos in wanted are actually on some shard.
	for uid, want := range wantedMap {
		_, ok := haveMap[uid]
		if !ok {
			// Repo is wanted but not present, need to add
			err := r.addRepo(ctx, want)
			// Collect errors so we can reconcile as many repos as possible.
			if err != nil {
				errs = errors.Append(errs, err)
			}
		} else {
			cloned = append(cloned, uid)
		}
	}

	// Next, check that there are no repos we no longer want to track.
	for uid, have := range haveMap {
		_, ok := wantedMap[uid]
		if !ok {
			// Repo is present but no longer wanted, need to remove
			// TODO: Here we could do some smart things, like TTL support etc.
			err := r.removeRepo(ctx, have)
			// Collect errors so we can reconcile as many repos as possible.
			if err != nil {
				errs = errors.Append(errs, err)
			}
		} else {
			if have.GetMaybeCorrupt() {
				r.removeRepo(ctx, have)
				// err = db.GitserverRepos().LogCorruption(ctx, repoName, fmt.Sprintf("sourcegraph detected corrupt repo: %s", reason), shardID)
				// if err != nil {
				// 	logger.Warn("failed to log repo corruption", log.String("repo", string(repoName)), log.Error(err))
				// }

				// logger.Info("removing corrupt repo", log.String("repo", string(dir)), log.String("reason", reason))
				// if err := fs.RemoveRepo(repoName); err != nil {
				// 	return true, err
				// }
				// reposRemoved.WithLabelValues(reason).Inc()

				// // Set as not_cloned in the database.
				// if err := db.GitserverRepos().SetCloneStatus(ctx, repoName, types.CloneStatusNotCloned, shardID); err != nil {
				// 	return true, errors.Wrap(err, "failed to update clone status")
				// }

				// return true, nil
			} else {
				cloned = append(cloned, uid)
			}
		}
	}

	// TODO: All repos in the intersection of have and want are verified to be
	// cloned at this stage, we can update our database with clone statuses here.
	// TODO: Maybe cloned needs to be deduplicated here?

	return errs
}

func (r *reconciler) wantedRepos(ctx context.Context) ([]ListRepo, error) {
	wanted := []ListRepo{}
	var nextPage string
	for {
		page, cur, err := r.store.ListRepos(ctx, nextPage)
		if err != nil {
			return nil, err
		}
		nextPage = cur
		wanted = append(wanted, page...)
		if nextPage == "" {
			// We shuffle the slice to randomize the order repos are processed in, to
			// increase chances of distributing load across shards.
			rand.Shuffle(len(wanted), func(i, j int) { wanted[i], wanted[j] = wanted[j], wanted[i] })
			return wanted, nil
		}
	}
}

func (r *reconciler) haveRepos(ctx context.Context) ([]*v1.ListRepo, error) {
	clis, err := r.cs.AllClients(ctx)
	if err != nil {
		return nil, err
	}

	have := []*v1.ListRepo{}
	for _, cli := range clis {
		var pageToken string
		for {
			resp, err := cli.ListRepos(ctx, &v1.ListReposRequest{PageToken: pageToken})
			if err != nil {
				return nil, err
			}
			for _, r := range resp.GetRepos() {
				have = append(have, r)
			}
			pageToken = resp.GetNextPageToken()
			if pageToken == "" {
				break
			}
		}
	}
	return have, nil
}

func (r *reconciler) addRepo(ctx context.Context, lr ListRepo) error {
	cc, repo, err := r.locator.Locate(ctx, &v1.GitserverRepository{Uid: lr.UID})
	if err != nil {
		return err
	}

	// TODO: We should not call repo update in here right away, but rather enqueue
	// it in some service that will handle the actual clones and fetches with proper
	// priorities and will honor concurrency limits etc.

	// TODO: The thing that will end up calling this method should do:
	// It may already be cloned
	// if !cloned {
	// 	if err := s.db.GitserverRepos().SetCloneStatus(ctx, repo, types.CloneStatusCloning, s.hostname); err != nil {
	// 		s.logger.Error("Setting clone status in DB", log.Error(err))
	// 	}
	// }
	// defer func() {
	// 	cloned, err := s.fs.RepoCloned(repo)
	// 	if err != nil {
	// 		s.logger.Error("failed to check if repo is cloned", log.Error(err))
	// 	} else if err := s.db.GitserverRepos().SetCloneStatus(
	// 		// Use a background context to ensure we still update the DB even if we time out
	// 		context.Background(),
	// 		repo,
	// 		cloneStatus(cloned, false),
	// 		s.hostname,
	// 	); err != nil {
	// 		s.logger.Error("Setting clone status in DB", log.Error(err))
	// 	}
	// }()
	_, err = cc.RepoUpdate(ctx, &v1.RepoUpdateRequest{
		Repo: repo,
	})
	// TODO: Here we should update our gitserver_repos table to reflect the clone
	// status, last_changed, last_fetched, etc.

	// It should also record the last error seen, until it's persisted as part
	// of the job anyways.
	// func (s *Server) setLastErrorNonFatal(ctx context.Context, name api.RepoName, err error) {
	// 	var errString string
	// 	if err != nil {
	// 		errString = err.Error()
	// 	}

	// 	if err := s.db.GitserverRepos().SetLastError(ctx, name, errString, s.hostname); err != nil {
	// 		s.logger.Warn("Setting last error in DB", log.Error(err))
	// 	}
	// }

	return err
}

func (r *reconciler) removeRepo(ctx context.Context, lr *v1.ListRepo) error {
	// TODO: Locate won't work for deleted repos, need to make that work somehow.
	cc, repo, err := r.locator.Locate(ctx, &v1.GitserverRepository{Uid: lr.GetRepo().GetUid()})
	if err != nil {
		return err
	}

	_, err = cc.RepoDelete(ctx, &v1.RepoDeleteRequest{
		Repo: repo,
	})

	// TODO: Who should do this? Locator? The store?
	// Regardless, it should be recorded here.
	// err = db.GitserverRepos().SetCloneStatus(ctx, repo, types.CloneStatusNotCloned, shardID)
	// if err != nil {
	// 	return errors.Wrap(err, "setting clone status after delete")
	// }

	return err
}
