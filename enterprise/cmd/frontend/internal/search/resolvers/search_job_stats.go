package resolvers

import (
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
)

var _ graphqlbackend.SearchJobStatsResolver = &searchJobStatsResolver{}

type searchJobStatsResolver struct {
}

func (e *searchJobStatsResolver) Total() int32 {
	//TODO implement me
	panic("implement me")
}

func (e *searchJobStatsResolver) Completed() int32 {
	//TODO implement me
	panic("implement me")
}

func (e *searchJobStatsResolver) Failed() int32 {
	//TODO implement me
	panic("implement me")
}

func (e *searchJobStatsResolver) InProgress() int32 {
	//TODO implement me
	panic("implement me")
}
