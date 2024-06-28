package modelconfig

import (
	"sync"

	"github.com/sourcegraph/sourcegraph/internal/modelconfig/types"
)

// Service is the system-wide component for obtaining the set of
// LLM models the current Sourcegraph instance is configured to use.
//
// You can obtain the global, package-level instance of this interface by
// calling the Get() method. It is safe for concurrent reads.
//
// Updating the system's model configuration is done by updating the Site
// Configuration. The implementation of the Service will listen
// for configuration changes and update the ModelConfiguration object in-
// memory as appropriate.
type Service interface {
	// Get returns the current model configuration for this Sourcegraph instance.
	// Callers should not modify the returned data, and treat it as if it were
	// immutable.
	Get() (*types.ModelConfiguration, error)
}

// Global instance of the model config service. We don't initialize via
// a sync.Once because we ensure Init cannot be called twice, and assume it
// will not be called concurrently.
var singletonConfigService *service

// Get returns the singleton ModelConfigService.
//
// This requires that the Init function has been called before hand, which
// is typically done on application startup.
func Get() Service {
	if singletonConfigService == nil {
		panic("ModelConfigService not initialized. Init not called.")
	}
	return singletonConfigService
}

// service implements the Service interface, and exposes a thread-safe `set` method
// for updating the current configuration.
type service struct {
	// currentConfig is the "source of truth" for this Sg instance's model configuration.
	currentConfig   *types.ModelConfiguration
	currentConfigMu sync.RWMutex
}

func (svc *service) Get() (*types.ModelConfiguration, error) {
	svc.currentConfigMu.RLock()
	defer svc.currentConfigMu.RUnlock()

	// Create a deep copy of the current configuration, so callers can operate on
	// older versions without worrying about data races or other types of errors.
	cfgCopy, err := deepCopy(svc.currentConfig)
	if err != nil {
		return nil, err
	}
	return cfgCopy, nil
}

// set updates the set.currentConfig to the supplied value. It is assumped that
// the Service will "own" the pointer, and the caller will no longer modify it.
func (svc *service) set(newConfig *types.ModelConfiguration) {
	// Block until the lock is available.
	svc.currentConfigMu.Lock()
	defer svc.currentConfigMu.Unlock()

	svc.currentConfig = newConfig
}
