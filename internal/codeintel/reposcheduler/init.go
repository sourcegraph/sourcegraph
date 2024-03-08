package reposcheduler

import (
	"context"
	"time"
)

type RepositoryToIndex struct {
	ID int
}

type RepositorySchedulingService interface {
	GetRepositoriesForIndexScan(
		ctx context.Context,
		batchOptions RepositoryBatchOptions,
		now time.Time) (_ []RepositoryToIndex, err error)
}

type service struct {
	store RepositorySchedulingStore
}

var _ RepositorySchedulingService = &service{}

func NewService(store RepositorySchedulingStore) RepositorySchedulingService {
	return &service{
		store: store,
	}
}

func (s *service) GetRepositoriesForIndexScan(ctx context.Context, batchOptions RepositoryBatchOptions, now time.Time) ([]RepositoryToIndex, error) {
	ids, error := s.store.GetRepositoriesForIndexScan(ctx, batchOptions, now)

	if error != nil {
		return nil, error
	}

	repos := make([]RepositoryToIndex, len(ids))
	for i, repoId := range ids {
		repos[i] = RepositoryToIndex{ID: repoId}
	}

	return repos, nil

}
