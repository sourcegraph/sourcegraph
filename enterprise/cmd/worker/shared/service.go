package shared

import (
	"context"

	shared "github.com/sourcegraph/sourcegraph/cmd/worker/shared"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/oobmigration/migrations"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/service"
)

type svc struct{}

func (svc) Name() string { return "worker" }

func (svc) Configure() env.Config {
	return shared.LoadConfig(additionalJobs, migrations.RegisterEnterpriseMigrators)
}

func (svc) Start(ctx context.Context, observationCtx *observation.Context, config env.Config) error {
	go setAuthzProviders(ctx, observationCtx)
	return shared.Start(ctx, observationCtx, config.(*shared.Config), getEnterpriseInit(observationCtx.Logger))
}

var Service service.Service = svc{}
