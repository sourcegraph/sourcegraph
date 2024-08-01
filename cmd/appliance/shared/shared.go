package shared

import (
	"context"
	"net"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/sourcegraph/log"
	sglogr "github.com/sourcegraph/log/logr"

	"github.com/sourcegraph/sourcegraph/internal/appliance"
	"github.com/sourcegraph/sourcegraph/internal/appliance/healthchecker"
	"github.com/sourcegraph/sourcegraph/internal/appliance/reconciler"
	"github.com/sourcegraph/sourcegraph/internal/appliance/selfupdate"
	pb "github.com/sourcegraph/sourcegraph/internal/appliance/v1"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/releaseregistry"
	"github.com/sourcegraph/sourcegraph/internal/service"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

var onlyOneSignalHandler = make(chan struct{})

func Start(ctx context.Context, observationCtx *observation.Context, ready service.ReadyFunc, config *Config) error {
	logger := observationCtx.Logger
	logr := sglogr.New(logger)

	ctrl.SetLogger(logr)

	k8sClient, err := client.New(config.k8sConfig, client.Options{})
	if err != nil {
		logger.Error("unable to create kubernetes client", log.Error(err))
		return err
	}

	relregClient := releaseregistry.NewClient(config.relregEndpoint)

	noResourceRestrictions := false
	noResourceRestrictions, err = strconv.ParseBool(config.noResourceRestrictions)
	if err != nil {
		logger.Error("parsing APPLIANCE_NO_RESOURCE_RESTRICTIONS as bool", log.Error(err))
		return err
	}

	app, err := appliance.NewAppliance(k8sClient, relregClient, config.applianceVersion, config.namespace, noResourceRestrictions, logger)
	if err != nil {
		logger.Error("failed to create appliance", log.Error(err))
		return err
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Logger: logr,
		Metrics: metricsserver.Options{
			BindAddress:   config.metrics.addr,
			SecureServing: config.metrics.secure,
		},
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				config.namespace: {},
			},
		},
	})
	if err != nil {
		logger.Error("unable to start manager", log.Error(err))
		return err
	}

	beginHealthCheckLoop := make(chan struct{})

	if err = (&reconciler.Reconciler{
		Client:               mgr.GetClient(),
		Scheme:               mgr.GetScheme(),
		Recorder:             mgr.GetEventRecorderFor("sourcegraph-appliance"),
		BeginHealthCheckLoop: beginHealthCheckLoop,
	}).SetupWithManager(mgr); err != nil {
		logger.Error("unable to create the appliance controller", log.Error(err))
		return err
	}

	listener, err := net.Listen("tcp", config.grpc.addr)
	if err != nil {
		logger.Error("unable to create tcp listener", log.Error(err))
		return err
	}

	srv := &http.Server{
		Addr:         config.http.addr,
		Handler:      app.Routes(),
		IdleTimeout:  time.Minute,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	grpcServer := makeGRPCServer(logger, app)

	selfUpdater := &selfupdate.SelfUpdate{
		Interval:        time.Hour,
		Logger:          logger.Scoped("SelfUpdate"),
		K8sClient:       k8sClient,
		RelregClient:    relregClient,
		DeploymentNames: config.selfDeploymentName,
		Namespace:       config.namespace,
	}

	probe := &healthchecker.PodProbe{K8sClient: k8sClient}
	healthChecker := &healthchecker.HealthChecker{
		Probe:     probe,
		K8sClient: k8sClient,
		Logger:    logger.Scoped("HealthChecker"),

		ServiceName: types.NamespacedName{Name: "sourcegraph-frontend", Namespace: config.namespace},
		Interval:    time.Minute,
		Graceperiod: time.Minute,
	}

	g, ctx := errgroup.WithContext(ctx)
	ctx = shutdownOnSignal(ctx)

	// fire-and-forget a pprof server
	go http.ListenAndServe(config.pprofAddr, nil)

	g.Go(func() error {
		logger.Info("gRPC server listening", log.String("address", listener.Addr().String()))
		if err := grpcServer.Serve(listener); err != nil {
			logger.Error("problem running gRPC server", log.Error(err))
			return err
		}
		return nil
	})
	g.Go(func() error {
		logger.Info("http server listening", log.String("address", config.http.addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("problem running http server", log.Error(err))
			return err
		}
		return nil
	})
	g.Go(func() error {
		logger.Info("starting manager")
		if err := mgr.Start(ctx); err != nil {
			logger.Error("problem running manager", log.Error(err))
			return err
		}
		return nil
	})
	g.Go(func() error {
		if err := healthChecker.ManageIngressFacingService(ctx, beginHealthCheckLoop, "app=sourcegraph-frontend", config.namespace); err != nil {
			logger.Error("problem running HealthChecker", log.Error(err))
			return err
		}
		return nil
	})
	if config.selfDeploymentName != "" {
		g.Go(func() error {
			return selfUpdater.Loop(ctx)
		})
	}
	g.Go(func() error {
		<-ctx.Done()
		grpcServer.GracefulStop()
		logger.Info("shutting down gRPC server gracefully")
		_ = srv.Shutdown(ctx)
		logger.Info("shutting down http server gracefully")
		return ctx.Err()
	})

	// Mark health server as ready
	ready()

	return g.Wait()
}

func makeGRPCServer(logger log.Logger, server pb.ApplianceServiceServer) *grpc.Server {
	grpcServer := defaults.NewServer(logger)
	pb.RegisterApplianceServiceServer(grpcServer, server)

	return grpcServer
}

// shutdownOnSignal registers for SIGTERM and SIGINT. A context is returned
// which is canceled on one of these signals. If a second signal is caught, the program
// is terminated with exit code 1.
func shutdownOnSignal(ctx context.Context) context.Context {
	close(onlyOneSignalHandler)

	ctx, cancel := context.WithCancel(ctx)

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancel() // first signal. Cancel context.
		<-c
		os.Exit(1) // second signal. Exit now.
	}()

	return ctx
}
