package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/version"
	"github.com/sourcegraph/sourcegraph/lib/background"
	"github.com/sourcegraph/sourcegraph/lib/managedservicesplatform/runtime"
)

func main() {
	runtime.Start[Config](Service{})
}

type Service struct{}

// Initialize implements runtime.Service.
func (s Service) Initialize(ctx context.Context, logger log.Logger, contract runtime.Contract, config Config) (background.Routine, error) {
	server := NewServer(fmt.Sprintf(":%d", contract.Port), logger, config)

	return background.CombinedRoutine{
		&httpRoutine{
			log:    logger,
			Server: server.http,
		},
	}, nil
}

func (s Service) Name() string    { return "build-tracker" }
func (s Service) Version() string { return version.Version() }

type httpRoutine struct {
	log log.Logger
	*http.Server
}

func (s *httpRoutine) Start() {
	if err := s.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		s.log.Error("error stopping server", log.Error(err))
	}
}

func (s *httpRoutine) Stop() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.Server.Shutdown(ctx); err != nil {
		s.log.Error("error shutting down server", log.Error(err))
	} else {
		s.log.Info("server stopped")
	}
}
