package resolvers

import (
	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/internal/batches"
)

type changesetsStatsResolver struct {
	stats batches.ChangesetsStats
}

var _ graphqlbackend.ChangesetsStatsResolver = &changesetsStatsResolver{}

func (r *changesetsStatsResolver) Retrying() int32 {
	return r.stats.Retrying
}
func (r *changesetsStatsResolver) Failed() int32 {
	return r.stats.Failed
}
func (r *changesetsStatsResolver) Processing() int32 {
	return r.stats.Processing
}
func (r *changesetsStatsResolver) Unpublished() int32 {
	return r.stats.Unpublished
}
func (r *changesetsStatsResolver) Draft() int32 {
	return r.stats.Draft
}
func (r *changesetsStatsResolver) Open() int32 {
	return r.stats.Open
}
func (r *changesetsStatsResolver) Merged() int32 {
	return r.stats.Merged
}
func (r *changesetsStatsResolver) Closed() int32 {
	return r.stats.Closed
}
func (r *changesetsStatsResolver) Deleted() int32 {
	return r.stats.Deleted
}
func (r *changesetsStatsResolver) Total() int32 {
	return r.stats.Total
}
