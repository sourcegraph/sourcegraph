// Package globals contains global variables that should be set by the frontend's main function on initialization.
package globals

import (
	"net/url"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/inconshreveable/log15" //nolint:logging // TODO move all logging to sourcegraph/log //nolint:go

	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/schema"
)

var defaultExternalURL = &url.URL{
	Scheme: "http",
	Host:   "example.com",
}

var externalURL = func() atomic.Value {
	var v atomic.Value
	v.Store(defaultExternalURL)
	return v
}()

var watchExternalURLOnce sync.Once

// WatchExternalURL watches for changes in the `externalURL` site configuration
// so that changes are reflected in what is returned by the ExternalURL function.
func WatchExternalURL() {
	watchExternalURLOnce.Do(func() {
		conf.Watch(func() {
			after := defaultExternalURL
			if val := conf.Get().ExternalURL; val != "" {
				var err error
				if after, err = url.Parse(val); err != nil {
					log15.Error("globals.ExternalURL", "value", val, "error", err)
					return
				}
			}

			if before := ExternalURL(); !reflect.DeepEqual(before, after) {
				SetExternalURL(after)
				if before.Host != "example.com" {
					log15.Info(
						"globals.ExternalURL",
						"updated", true,
						"before", before,
						"after", after,
					)
				}
			}
		})
	})
}

// ExternalURL returns the fully-resolved, externally accessible frontend URL.
// Callers must not mutate the returned pointer.
func ExternalURL() *url.URL {
	return externalURL.Load().(*url.URL)
}

// SetExternalURL sets the fully-resolved, externally accessible frontend URL.
func SetExternalURL(u *url.URL) {
	externalURL.Store(u)
}

var defaultBranding = &schema.Branding{
	BrandName: "Sourcegraph",
}

// branding mirrors the value of `branding` in the site configuration.
// This variable is used to monitor configuration change via conf.Watch and must be operated atomically.
var branding = func() atomic.Value {
	var v atomic.Value
	v.Store(defaultBranding)
	return v
}()

var brandingWatchers uint32

// WatchBranding watches for changes in the `branding` site configuration
// so that changes are reflected in what is returned by the Branding function.
// This should only be called once and will panic otherwise.
func WatchBranding() {
	if atomic.AddUint32(&brandingWatchers, 1) != 1 {
		panic("WatchBranding called more than once")
	}

	conf.Watch(func() {
		after := conf.Get().Branding
		if after == nil {
			after = defaultBranding
		} else if after.BrandName == "" {
			bcopy := *after
			bcopy.BrandName = defaultBranding.BrandName
			after = &bcopy
		}

		if before := Branding(); !reflect.DeepEqual(before, after) {
			SetBranding(after)
			log15.Debug(
				"globals.Branding",
				"updated", true,
				"before", before,
				"after", after,
			)
		}
	})
}

// Branding returns the last valid value of branding in the site configuration.
// Callers must not mutate the returned pointer.
func Branding() *schema.Branding {
	return branding.Load().(*schema.Branding)
}

// SetBranding sets a valid value for the branding.
func SetBranding(u *schema.Branding) {
	branding.Store(u)
}

// ConfigurationServerFrontendOnly provides the contents of the site configuration
// to other services and manages modifications to it.
//
// Any another service that attempts to use this variable will panic.
var ConfigurationServerFrontendOnly *conf.Server
