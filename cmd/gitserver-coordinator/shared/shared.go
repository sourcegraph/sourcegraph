package shared

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"strings"

	"github.com/gorilla/mux"
	"github.com/sourcegraph/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	v1reflectiongrpc "google.golang.org/grpc/reflection/grpc_reflection_v1"
	v1alphareflectiongrpc "google.golang.org/grpc/reflection/grpc_reflection_v1alpha"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/runtime/protoimpl"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver-coordinator/shared/proxy"
	"github.com/sourcegraph/sourcegraph/cmd/gitserver-coordinator/shared/router"
	"github.com/sourcegraph/sourcegraph/internal/actor"
	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/authz/subrepoperms"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/conf/conftypes"
	"github.com/sourcegraph/sourcegraph/internal/database"
	connections "github.com/sourcegraph/sourcegraph/internal/database/connections/live"
	"github.com/sourcegraph/sourcegraph/internal/encryption/keyring"
	"github.com/sourcegraph/sourcegraph/internal/env"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	gitserverproto "github.com/sourcegraph/sourcegraph/internal/gitserver/v1"
	"github.com/sourcegraph/sourcegraph/internal/goroutine"
	"github.com/sourcegraph/sourcegraph/internal/goroutine/recorder"
	"github.com/sourcegraph/sourcegraph/internal/grpc/defaults"
	"github.com/sourcegraph/sourcegraph/internal/grpc/grpcserver"
	"github.com/sourcegraph/sourcegraph/internal/httpserver"
	"github.com/sourcegraph/sourcegraph/internal/instrumentation"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/requestclient"
	"github.com/sourcegraph/sourcegraph/internal/requestinteraction"
	"github.com/sourcegraph/sourcegraph/internal/service"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

func Main(ctx context.Context, observationCtx *observation.Context, ready service.ReadyFunc, config *Config) error {
	logger := observationCtx.Logger

	// Load and validate configuration.
	if err := config.Validate(); err != nil {
		return errors.Wrap(err, "failed to validate configuration")
	}

	// Create a database connection.
	sqlDB, err := getDB(observationCtx)
	if err != nil {
		return errors.Wrap(err, "initializing database stores")
	}
	db := database.NewDB(observationCtx.Logger, sqlDB)

	// Initialize the keyring.
	err = keyring.Init(ctx)
	if err != nil {
		return errors.Wrap(err, "initializing keyring")
	}

	authz.DefaultSubRepoPermsChecker = subrepoperms.NewSubRepoPermsClient(db.SubRepoPerms())
	router := router.NewLegacyRouter(db)

	routines := []goroutine.BackgroundRoutine{
		grpcserver.NewFromAddr(logger, config.ListenAddress, makeGRPCServer(logger, router)),
		makeHTTPServer(logger, config.HTTPCloneAddress),
	}

	// Register recorder in all routines that support it.
	recorderCache := recorder.GetCache()
	rec := recorder.New(observationCtx.Logger, env.MyName, recorderCache)
	for _, r := range routines {
		if recordable, ok := r.(recorder.Recordable); ok {
			// Set the hostname to the shardID so we record the routines per
			// gitserver instance.
			recordable.SetJobName("gitserver-coordinator")
			recordable.RegisterRecorder(rec)
			rec.Register(recordable)
		}
	}
	rec.RegistrationDone()

	logger.Info("gitserver-coordinator: listening", log.String("addr", config.ListenAddress))

	// We're ready!
	ready()

	// Launch all routines!
	goroutine.MonitorBackgroundRoutines(ctx, routines...)

	return nil
}

func makeHTTPServer(logger log.Logger, httpCloneAddress string) goroutine.BackgroundRoutine {
	handler := NewHTTPHandler(logger)
	handler = actor.HTTPMiddleware(logger, handler)
	handler = requestclient.InternalHTTPMiddleware(handler)
	handler = requestinteraction.HTTPMiddleware(handler)
	handler = trace.HTTPMiddleware(logger, handler, conf.DefaultClient())
	handler = instrumentation.HTTPMiddleware("", handler)
	return httpserver.NewFromAddr(httpCloneAddress, &http.Server{
		Handler: handler,
	})
}

// NewHTTPHandler returns a HTTP handler that serves a git upload pack server,
// plus a few other endpoints.
// TODO: Make zoekt talk to coordinator and not frontend: https://sourcegraph.com/-/editor?remote_url=github.com%2Fsourcegraph%2Fzoekt&branch=main&file=cmd%2Fzoekt-sourcegraph-indexserver%2Fsg.go&editor=VSCode&version=2.2.16&start_row=399&start_col=0&end_row=401&end_col=1
func NewHTTPHandler(logger log.Logger) http.Handler {
	base := mux.NewRouter()
	base.StrictSlash(true)

	gitRouter := base.PathPrefix("/git").Subrouter()

	var getRepoName = func(r *http.Request) string {
		return mux.Vars(r)["RepoName"]
	}

	gitserverRedirect := func(gitPath string) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			repo := getRepoName(r)

			addrs := gitserver.NewGitserverAddresses(conf.Get())
			addrForRepo := addrs.AddrForRepo(r.Context(), api.RepoName(repo))

			redirectURL := &url.URL{
				Scheme:   "http",
				Host:     addrForRepo,
				Path:     path.Join("/git", repo, gitPath),
				RawQuery: r.URL.RawQuery,
			}

			http.Redirect(w, r, redirectURL.String(), http.StatusSeeOther)
		})
	}

	gitRouter.Path("/{RepoName:.*}/info/refs").Handler(gitserverRedirect("/info/refs"))
	gitRouter.Path("/{RepoName:.*}/git-upload-pack").Handler(gitserverRedirect("/git-upload-pack"))

	return base
}

// makeGRPCServer creates a new *grpc.Server for the gitserver endpoints and registers
// it with methods on the given server.
func makeGRPCServer(logger log.Logger, router router.RepositoryRouter) *grpc.Server {
	dir, err := gitserverDirector(logger, router)
	if err != nil {
		logger.Fatal("failed to create gitserver director", log.Error(err))
	}
	grpcServer := defaults.NewServerNoReflect(
		logger,
		grpc.UnknownServiceHandler(proxy.TransparentHandler(dir)),
	)

	// FLAILING: Trying to make reflection work for the Git API:
	fakeServer := defaults.NewServerNoReflect(logger)
	gitserverproto.RegisterGitserverServiceServer(fakeServer, gitserverproto.UnimplementedGitserverServiceServer{})

	refOpt := reflection.ServerOptions{Services: serviceInfoProviderFunc(func() map[string]grpc.ServiceInfo {
		base := grpcServer.GetServiceInfo()
		git := fakeServer.GetServiceInfo()
		base[gitserverproto.GitserverService_ServiceDesc.ServiceName] = git[gitserverproto.GitserverService_ServiceDesc.ServiceName]
		return base
	})}

	svr := reflection.NewServerV1(refOpt)
	v1alphareflectiongrpc.RegisterServerReflectionServer(grpcServer, reflection.NewServer(refOpt))
	v1reflectiongrpc.RegisterServerReflectionServer(grpcServer, svr)
	// END

	// proto.RegisterGitserverServiceServer(grpcServer, server.NewGRPCServer(s))

	return grpcServer
}

type serviceInfoProviderFunc func() map[string]grpc.ServiceInfo

func (f serviceInfoProviderFunc) GetServiceInfo() map[string]grpc.ServiceInfo {
	return f()
}

func gitserverDirector(logger log.Logger, router router.RepositoryRouter) (proxy.StreamDirector, error) {
	dst, err := defaults.Dial("127.0.0.1:3501", logger)
	if err != nil {
		return nil, errors.Wrap(err, "dialing gitserver")
	}

	pr, err := protoregistry.GlobalFiles.FindFileByPath(gitserverproto.GitserverService_ServiceDesc.Metadata.(string))
	if err != nil {
		return nil, errors.Wrap(err, "find file by path")
	}

	serviceName := gitserverproto.GitserverService_ServiceDesc.ServiceName[strings.LastIndex(gitserverproto.GitserverService_ServiceDesc.ServiceName, ".")+1:]
	gitSvc := pr.Services().ByName(protoreflect.Name(serviceName))
	if gitSvc == nil {
		return nil, errors.Newf("failed to find service by name: %q", serviceName)
	}

	methodCache := make(map[string]*cachedMethod)
	for i := 0; i < gitSvc.Methods().Len(); i++ {
		m := gitSvc.Methods().Get(i)

		inputName := m.Input().FullName()
		// Now we get the type of the request message.
		// That is `rpc GetCommit(GetCommitRequest) returns (GetCommitResponse)`
		//                        ^^^^^^^^^^^^^^^^
		requestType, err := protoregistry.GlobalTypes.FindMessageByName(inputName)
		if err != nil {
			if errors.Is(err, protoregistry.NotFound) {
				return nil, errors.Wrapf(err, "no registered type found for %q", inputName)
			}
			return nil, errors.Wrapf(err, "failed to get type for %q: %v", inputName)
		}

		methodCache[string(m.Name())] = &cachedMethod{
			requestType: requestType,
		}

		// Find the field on the request message type that has the `target_repository`
		// field set.
		found := false
		foundOld := false
		for i := 0; i < m.Input().Fields().Len(); i++ {
			field := m.Input().Fields().Get(i)
			ext := getExtension(field.Options(), gitserverproto.E_TargetRepository)
			if ext == nil {
				ext = getExtension(field.Options(), gitserverproto.E_OldStringRepository)
				if ext != nil {
					isRepo, ok := ext.(bool)
					if !ok {
						return nil, errors.Newf("extension on method %q field %q not a bool", m.FullName(), field.FullName())
					}
					if isRepo {
						if foundOld {
							return nil, errors.Newf("extension old_string_repository should not be set on more than one field for %q", requestType.Descriptor().FullName())
						}
						if field.Kind() != protoreflect.StringKind {
							return nil, errors.Newf("field for old_string_repository is not a string on %q", m.FullName())
						}
						methodCache[string(m.Name())].fallbackField = field
						foundOld = true
					} else {
						return nil, errors.Newf("extension old_string_repository should not be false on method %q field %q", m.FullName(), field.FullName())
					}
				}
				continue
			}

			isRepo, ok := ext.(bool)
			if !ok {
				return nil, errors.Newf("extension on method %q field %q not a bool", m.FullName(), field.FullName())
			}
			if isRepo {
				if found {
					return nil, errors.Newf("extension target_repository should not be set on more than one field for %q", requestType.Descriptor().FullName())
				}
				if field.Kind() != protoreflect.MessageKind {
					return nil, errors.Newf("field for target_repository is not a message on %q", m.FullName())
				}
				methodCache[string(m.Name())].targetField = field
				methodCache[string(m.Name())].shouldRoute = true
				found = true
			} else {
				return nil, errors.Newf("extension target_repository should not be false on method %q field %q", m.FullName(), field.FullName())
			}
		}
		if !found {
			logger.Warn("could not find target_repository field on method", log.String("method", string(m.FullName())))
		}
	}

	repoTypeName := "gitserver.v1.GitserverRepository"
	repoType, err := protoregistry.GlobalTypes.FindMessageByName(protoreflect.FullName(repoTypeName))
	if err != nil {
		if errors.Is(err, protoregistry.NotFound) {
			return nil, errors.Wrapf(err, "no registered type found for %q", repoTypeName)
		}
		return nil, errors.Wrapf(err, "failed to get type for %q: %v", repoTypeName)
	}

	return func(ctx context.Context, fullMethodName string, peeker proxy.StreamPeeker) (_ context.Context, _ *grpc.ClientConn, _ proto.Message, err error) {
		tr, ctx := trace.New(ctx, "gitserverDirector")
		defer tr.EndWithErr(&err)

		fmt.Printf("received request for %q\n", fullMethodName)

		service, method, err := parseMethodName(fullMethodName)
		if err != nil {
			return nil, nil, nil, err
		}

		// Make sure only requests to the GitService are proxied.
		if protoreflect.FullName(service) != gitSvc.FullName() {
			return nil, nil, nil, status.New(codes.Unimplemented, fmt.Sprintf("unknown service %v", service)).Err()
		}

		// Make sure we know how to handle the method.
		meth, ok := methodCache[method]
		if !ok {
			return nil, nil, nil, status.New(codes.Unimplemented, fmt.Sprintf("unknown method %v for service %v", method, service)).Err()
		}

		var msgToForward proto.Message

		if meth.shouldRoute {
			// Grab the first message on the stream.
			firstMsg, err := peeker.Peek()
			if err != nil {
				return nil, nil, nil, status.Errorf(codes.Internal, "peeking: %v", err)
			}

			// Make a new instance of the reflected type of the request message and
			// unmarshal the first message on the stream into it so we can read and
			// modify it.
			req := meth.requestType.New().Interface()
			if err := proto.Unmarshal(firstMsg.ProtoReflect().GetUnknown(), req); err != nil {
				return nil, nil, nil, errors.Wrapf(err, "unmarshaling request %T", req)
			}

			msgToForward = proto.Clone(req)

			// TODO: Make use of addr here.
			upstreamRepo, _, err := func() (*gitserverproto.GitserverRepository, string, error) {
				// If the target_repository field is not set, we try to extract the
				// information from the old_string_repository field which is a string,
				// and contains the same info that would go in the uid field.
				if !req.ProtoReflect().Has(meth.targetField) {
					// Request doesn't have a target_repository set, so let's see if a fallback
					// exists instead.
					if meth.fallbackField != nil && req.ProtoReflect().Has(meth.fallbackField) {
						repoName := req.ProtoReflect().Get(meth.fallbackField).String()

						return router.RouteLegacy(ctx, api.RepoName(repoName))
					}
					return nil, "", status.Errorf(codes.InvalidArgument, "request has no target_repository field")
				}

				field := req.ProtoReflect().Get(meth.targetField)

				r, ok := field.Message().Interface().(*gitserverproto.GitserverRepository)
				if !ok {
					return nil, "", errors.New("field for target_repository is not a GitserverRepository")
				}

				return router.Route(ctx, r)
			}()
			if err != nil {
				return nil, nil, nil, status.Error(codes.Internal, errors.Wrap(err, "routing repo").Error())
			}

			upsertedRepo := repoType.New()
			upsertedRepo.Set(meth.targetField.Message().Fields().ByName("uid"), protoreflect.ValueOf(upstreamRepo.Uid))
			upsertedRepo.Set(meth.targetField.Message().Fields().ByName("name"), protoreflect.ValueOf(upstreamRepo.Name))
			upsertedRepo.Set(meth.targetField.Message().Fields().ByName("path"), protoreflect.ValueOf(upstreamRepo.Path))

			msgToForward.ProtoReflect().Set(meth.targetField, protoreflect.ValueOf(upsertedRepo))
			fmt.Printf("got request for repo %q\n", upstreamRepo.Name)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		// Copy the inbound metadata explicitly.
		outCtx := metadata.NewOutgoingContext(ctx, md.Copy())
		if ok {
			// TODO: Pick dst based on routing decision.
			return outCtx, dst, msgToForward, err
		}
		return nil, nil, nil, status.Errorf(codes.Unimplemented, "Unknown method")
	}, nil
}

type cachedMethod struct {
	requestType   protoreflect.MessageType
	shouldRoute   bool
	targetField   protoreflect.FieldDescriptor
	fallbackField protoreflect.FieldDescriptor
}

func parseMethodName(fullMethodName string) (service, method string, err error) {
	if fullMethodName != "" && fullMethodName[0] == '/' {
		fullMethodName = fullMethodName[1:]
	}
	pos := strings.LastIndex(fullMethodName, "/")
	if pos == -1 {
		errDesc := fmt.Sprintf("malformed method name: %q", fullMethodName)
		return "", "", status.New(codes.Unimplemented, errDesc).Err()
	}

	service = fullMethodName[:pos]
	method = fullMethodName[pos+1:]

	return service, method, nil
}

func getExtension(options proto.Message, extension *protoimpl.ExtensionInfo) interface{} {
	if !proto.HasExtension(options, extension) {
		return nil
	}

	return proto.GetExtension(options, extension)
}

// getDB initializes a connection to the database and returns a dbutil.DB
func getDB(observationCtx *observation.Context) (*sql.DB, error) {
	// Gitserver is an internal actor. We rely on the frontend to do authz checks for
	// user requests.
	//
	// This call to SetProviders is here so that calls to GetProviders don't block.
	authz.SetProviders(true, []authz.Provider{})

	dsn := conf.GetServiceConnectionValueAndRestartOnChange(func(serviceConnections conftypes.ServiceConnections) string {
		return serviceConnections.PostgresDSN
	})
	return connections.EnsureNewFrontendDB(observationCtx, dsn, "gitserver-coordinator")
}
