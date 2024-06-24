package definitions

import (
	"github.com/sourcegraph/sourcegraph/monitoring/monitoring"
)

type Dashboards []*monitoring.Dashboard

// Default is the default set of monitoring dashboards to generate. Ensure that any
// dashboards created or removed are updated in the return value here as required.
func Default() Dashboards {
	return []*monitoring.Dashboard{
		Frontend(),
		GitServer(),
		Postgres(),
		PreciseCodeIntelWorker(),
		Redis(),
		Worker(),
		RepoUpdater(),
		Searcher(),
		Symbols(),
		SyntectServer(),
		Zoekt(),
		Prometheus(),
		Executor(),
		Containers(),
		CodeIntelAutoIndexing(),
		CodeIntelCodeNav(),
		CodeIntelPolicies(),
		CodeIntelRanking(),
		CodeIntelUploads(),
		Telemetry(),
		OtelCollector(),
	}
}

// Names returns the names of all dashboards.
func (ds Dashboards) Names() (names []string) {
	for _, d := range ds {
		names = append(names, d.Name)
	}
	return
}

// GetByName retrieves the dashboard of the given name, otherwise returns nil.
func (ds Dashboards) GetByName(name string) *monitoring.Dashboard {
	for _, d := range ds {
		if d.Name == name {
			return d
		}
	}
	return nil
}
