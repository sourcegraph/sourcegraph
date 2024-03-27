package example

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/sourcegraph/log"

	"github.com/sourcegraph/sourcegraph/internal/version"
	"github.com/sourcegraph/sourcegraph/lib/background"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/managedservicesplatform/runtime"
)

type Config struct {
	StatelessMode bool
	Variable      string
}

func (c *Config) Load(env *runtime.Env) {
	c.StatelessMode = env.GetBool("STATELESS_MODE", "false", "if true, disable dependencies")
	c.Variable = env.Get("VARIABLE", "13", "variable value")
}

type Service struct{}

var _ runtime.Service[Config] = Service{}

func (s Service) Name() string    { return "example" }
func (s Service) Version() string { return version.Version() }
func (s Service) Initialize(
	ctx context.Context,
	logger log.Logger,
	contract runtime.Contract,
	config Config,
) (background.Routine, error) {
	logger.Info("starting service")

	if !config.StatelessMode {
		if err := initPostgreSQL(ctx, contract); err != nil {
			return nil, errors.Wrap(err, "initPostgreSQL")
		}
		logger.Info("postgresql database configured")

		if err := writeBigQueryEvent(ctx, contract, "service.initialized"); err != nil {
			return nil, errors.Wrap(err, "writeBigQueryEvent")
		}
		logger.Info("bigquery connection checked")

		if err := testRedisConnection(ctx, contract); err != nil {
			return nil, errors.Wrap(err, "newRedisConnection")
		}
		logger.Info("redis connection checked")
	}

	requestCounter, err := getRequestCounter()
	if err != nil {
		return nil, err
	}

	h := http.NewServeMux()
	h.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCounter.Add(r.Context(), 1)
		_, _ = w.Write([]byte(fmt.Sprintf("Variable: %s", config.Variable)))
	}))
	// Test endpoint for making CURL requests to arbitrary targets from this
	// service, for testing networking. Requires diagnostic auth.
	h.Handle("/proxy", contract.DiagnosticsAuthMiddleware(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := r.URL.Query().Get("host")
			if host == "" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("query parameter 'host' is required"))
				return
			}
			hostURL, err := url.Parse(host)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(err.Error()))
				return
			}

			path := r.URL.Query().Get("path")
			if path == "" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("query parameter 'path' is required"))
				return
			}

			// Copy the request body and build the request
			defer r.Body.Close()
			body, err := io.ReadAll(r.Body)
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(err.Error()))
				return
			}
			proxiedRequest, err := http.NewRequest(r.Method, "/"+path, bytes.NewReader(body))
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(err.Error()))
				return
			}

			// Copy relevant request headers after stripping their prefixes
			for k, vs := range r.Header {
				if strings.HasPrefix(k, "X-Proxy-") {
					for _, v := range vs {
						proxiedRequest.Header.Add(strings.TrimPrefix(k, "X-Proxy-"), v)
					}
				}
			}

			// Send to target
			proxy := httputil.NewSingleHostReverseProxy(hostURL)
			proxy.ServeHTTP(w, proxiedRequest)
		}),
	))
	contract.RegisterDiagnosticsHandlers(h, serviceState{
		statelessMode: config.StatelessMode,
		contract:      contract,
	})

	return background.CombinedRoutine{
		&httpRoutine{
			log: logger,
			Server: &http.Server{
				Addr:    fmt.Sprintf(":%d", contract.Port),
				Handler: h,
			},
		},
	}, nil
}

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
