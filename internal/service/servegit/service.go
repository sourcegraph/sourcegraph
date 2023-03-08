package servegit

import (
	"context"
	"os"

	"github.com/sourcegraph/sourcegraph/internal/debugserver"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/service"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type Config struct {
	ServeConfig

	// LocalRoot is the code to sync based on where app is run from. This is
	// different to the repos a user explicitly adds via the setup wizard.
	// This should not be used as the root value in the service.
	CWDRoot string
}

func (c *Config) Load() {
	// We bypass BaseConfig since it doesn't handle variables being empty.
	if src, ok := os.LookupEnv("SRC"); ok {
		c.CWDRoot = src
	} else if pwd, err := os.Getwd(); err == nil {
		c.CWDRoot = pwd
	}

	c.ServeConfig.Load()
}

type svc struct{}

func (s svc) Name() string {
	return "servegit"
}

func (s svc) Configure() (env.Config, []debugserver.Endpoint) {
	c := &Config{}
	c.Load()
	return c, nil
}

func (s svc) Start(ctx context.Context, observationCtx *observation.Context, ready service.ReadyFunc, configI env.Config) (err error) {
	config := configI.(*Config)

	// Start servegit which walks Root to find repositories and exposes
	// them over HTTP for Sourcegraph's syncer to discover and clone.
	srv := &Serve{
		ServeConfig: config.ServeConfig,
		Logger:      observationCtx.Logger,
	}
	if err := srv.Start(); err != nil {
		return errors.Wrap(err, "failed to start servegit server which discovers local repositories")
	}

	if config.CWDRoot == "" {
		observationCtx.Logger.Warn("skipping local code since the environment variable SRC is not set")
		return nil
	}

	// Now that servegit is running, we can add the external service which
	// connects to it.
	//
	// Note: src.Addr is updated to reflect the actual listening address.
	if err := ensureExtSVC(observationCtx, "http://"+srv.Addr, config.CWDRoot); err != nil {
		return errors.Wrap(err, "failed to create external service which imports local repositories")
	}

	return nil
}

var Service service.Service = svc{}
