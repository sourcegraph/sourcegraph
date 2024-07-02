package clientconfig

import (
	"context"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/cody"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/clientconfig"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/database"
)

// Service is the system-wide component for obtaining the client config
// for a given user.
//
// You can obtain the global, package-level instance of this interface by
// calling the Get() method. It is safe for concurrent reads.
type Service interface {
	// Get returns the current cody client configuration for the authenticated
	// user. Callers should not modify the returned data, and treat it as if it
	// were immutable.
	GetForActor(ctx context.Context, actor *actor.Actor) (*clientconfig.ClientConfig, error)
}

// Global instance of the client config service. We don't initialize via
// a sync.Once because we ensure Init cannot be called twice, and assume it
// will not be called concurrently.
var singletonConfigService *service

// Get returns the singleton ClientConfigService.
//
// This requires that the Init function has been called before hand, which
// is typically done on application startup.
func Get() Service {
	if singletonConfigService == nil {
		panic("ClientConfigService not initialized. Init not called.")
	}
	return singletonConfigService
}

// service implements the Service interface
type service struct {
	logger log.Logger
	db     database.DB
}

func (svc *service) GetForActor(ctx context.Context, actor *actor.Actor) (*clientconfig.ClientConfig, error) {
	c := clientconfig.ClientConfigV1{
		// TODO(chrsmith): TODO(slimsag): Set this to `true` when and only when clients should use
		// the new LLM models httpapi endpoint being added in e.g. https://github.com/sourcegraph/sourcegraph/pull/63507
		ModelsAPIEnabled: false,
	}

	// 🚨 SECURITY: This code lets site admins restrict who has access to Cody at all via RBAC.
	// https://sourcegraph.com/docs/cody/clients/enable-cody-enterprise#enable-cody-only-for-some-users
	c.CodyEnabled, _ = cody.IsCodyEnabled(ctx, svc.db)

	// 🚨 SECURITY: This code enforces that users do not have access to Cody features which
	// site admins do not want them to have access to.
	//
	// Legacy admin-control configuration which should be moved to RBAC, not globally in site
	// config. e.g. we should do it like https://github.com/sourcegraph/sourcegraph/pull/58831
	features := conf.GetConfigFeatures(conf.Get().SiteConfig())
	if features != nil { // nil -> Cody not enabled
		c.ChatEnabled = features.Chat
		c.AutoCompleteEnabled = features.AutoComplete
		c.CommandsEnabled = features.Commands
		c.AttributionEnabled = features.Attribution
	}

	// Legacy feature-enablement configuration which should be moved to featureflag or RBAC,
	// not exist in site config.
	completionConfig := conf.GetCompletionsConfig(conf.Get().SiteConfig())
	if completionConfig != nil { // nil -> Cody not enabled
		c.SmartContextWindowEnabled = completionConfig.SmartContextWindow != "disabled"
	}

	return &clientconfig.ClientConfig{V1: &c}, nil
}
