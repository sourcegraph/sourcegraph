package shared

import (
	"context"

	"github.com/sourcegraph/sourcegraph/internal/debugserver"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/service"
)

var Service service.Service = svc{}

type svc struct{}

func (svc) Name() string { return "appliance" }

func (svc) Configure() (env.Config, []debugserver.Endpoint) {
	var config Config
	config.Load()
	return &config, nil
}

func (svc) Start(ctx context.Context, observationCtx *observation.Context, ready service.ReadyFunc, config env.Config) error {
	return Start(ctx, observationCtx, ready, config.(*Config))
}
