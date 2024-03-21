package api

import (
	"context"
	"fmt"
	"net/http"

	logger "github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/cmd/symbols/types"
	internalgrpc "github.com/sourcegraph/sourcegraph/internal/grpc"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	"github.com/sourcegraph/sourcegraph/internal/search"
	proto "github.com/sourcegraph/sourcegraph/internal/symbols/v1"
	internaltypes "github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

type grpcService struct {
	searchFunc   types.SearchFunc
	readFileFunc func(context.Context, internaltypes.RepoCommitPath) ([]byte, error)
	ctagsBinary  string
	proto.UnimplementedSymbolsServiceServer
	logger logger.Logger
}

func (s *grpcService) Search(ctx context.Context, r *proto.SearchRequest) (*proto.SearchResponse, error) {
	var response proto.SearchResponse

	params := r.ToInternal()
	res, err := s.searchFunc(ctx, params)
	if err != nil {
		s.logger.Error("symbol search failed",
			logger.String("arguments", fmt.Sprintf("%+v", params)),
			logger.Error(err),
		)

		var limitErr *limitHitError
		if errors.As(err, &limitErr) {
			response.FromInternal(&search.SymbolsResponse{Symbols: res, LimitHit: true})
			return &response, nil
		}

		response.FromInternal(&search.SymbolsResponse{Err: err.Error()})
		return &response, nil
	}

	response.FromInternal(&search.SymbolsResponse{Symbols: res})
	return &response, nil
}

func (s *grpcService) Healthz(ctx context.Context, _ *proto.HealthzRequest) (*proto.HealthzResponse, error) {
	// Note: Kubernetes only has beta support for GRPC Healthchecks since version >= 1.23. This means
	// that we probably need the old non-GRPC healthcheck endpoint for a while.
	//
	// See https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/#define-a-grpc-liveness-probe
	// for more information.
	return &proto.HealthzResponse{}, nil
}

func NewHandler(
	searchFunc types.SearchFunc,
	readFileFunc func(context.Context, internaltypes.RepoCommitPath) ([]byte, error),
	handleStatus func(http.ResponseWriter, *http.Request),
	ctagsBinary string,
) http.Handler {
	rootLogger := logger.Scoped("symbolsServer")

	// Initialize the gRPC server
	grpcServer := defaults.NewServer(rootLogger)
	proto.RegisterSymbolsServiceServer(grpcServer, &grpcService{
		searchFunc:   searchFunc,
		readFileFunc: readFileFunc,
		ctagsBinary:  ctagsBinary,
		logger:       rootLogger.Scoped("grpc"),
	})

	jsonLogger := rootLogger.Scoped("jsonrpc")

	// Initialize the legacy JSON API server
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", handleHealthCheck(jsonLogger))

	if handleStatus != nil {
		mux.HandleFunc("/status", handleStatus)
	}

	return internalgrpc.MultiplexHandlers(grpcServer, mux)
}

func handleHealthCheck(l logger.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		if _, err := w.Write([]byte("OK")); err != nil {
			l.Error("failed to write healthcheck response", logger.Error(err))
		}
	}
}
