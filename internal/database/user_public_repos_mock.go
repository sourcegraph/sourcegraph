package database

import "context"

type MockUserPublicRepos struct {
	ListByUser func(ctx context.Context, userID int32) ([]UserPublicRepo, error)
}
