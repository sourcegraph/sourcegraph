package dbconn

import (
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
)

var manager = &dbconnManager{
	registeredConfig: make(map[string]*pgx.ConnConfig),
}

// dbconnManager is a global singleton that manages data source registration
// for the stdlib.RegisterConnConfig function.
// DO NOT USE stdlib.RegisterConnConfig directly, use dbconnManager.registerConfig instead.
// This added layer is needed to make ConnectionUpdater work becuase
// it needs to be able to retrieve the ConnConfig for a given data source name.
// Such detail is not exposed by pgx, so we need to track it ourselves.
type dbconnManager struct {
	mu sync.Mutex
	// registeredConfig is a map of data source name to the corresponding pgx.ConnConfig.
	// e.g.,
	// registeredConnConfig0 -> ConnConfig{}
	// registeredConnConfig1 -> ConnConfig{}
	registeredConfig map[string]*pgx.ConnConfig
}

// registerConfig is a wrapper around stdlib.RegisterConnConfig.
// It registers config with pgx and also tracks it internally.
func (m *dbconnManager) registerConfig(cfg *pgx.ConnConfig) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	name := stdlib.RegisterConnConfig(cfg)
	m.registeredConfig[name] = cfg
	return name
}

// getConfig returns the pgx.ConnConfig for the given data source name.
func (m *dbconnManager) getConfig(name string) *pgx.ConnConfig {
	m.mu.Lock()
	defer m.mu.Unlock()
	if v, ok := m.registeredConfig[name]; ok {
		return v
	}
	return nil
}
