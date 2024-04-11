package inference

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	baselua "github.com/yuin/gopher-lua"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/internal/inference/lua"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/internal/inference/luatypes"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/autoindexing/shared"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/luasandbox"
	"github.com/sourcegraph/sourcegraph/internal/luasandbox/util"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/internal/ratelimit"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/autoindex/config"
)

type Service struct {
	sandboxService                  SandboxService
	gitService                      GitService
	limiter                         *ratelimit.InstrumentedLimiter
	maximumFilesWithContentCount    int
	maximumFileWithContentSizeBytes int
	operations                      *operations
}

type invocationContext struct {
	sandbox    *luasandbox.Sandbox
	printSink  io.Writer
	gitService GitService
	repo       api.RepoID
	commit     string
	invocationFunctionTable
}

type invocationFunctionTable struct {
	linearize    func(recognizer *luatypes.Recognizer) []*luatypes.Recognizer
	callback     func(recognizer *luatypes.Recognizer) *baselua.LFunction
	scanLuaValue func(value baselua.LValue) ([]config.IndexJob, error)
}

type LimitError struct {
	description string
}

func (e LimitError) Error() string {
	return e.description
}

func newService(
	observationCtx *observation.Context,
	sandboxService SandboxService,
	gitService GitService,
	limiter *ratelimit.InstrumentedLimiter,
	maximumFilesWithContentCount int,
	maximumFileWithContentSizeBytes int,
) *Service {
	return &Service{
		sandboxService:                  sandboxService,
		gitService:                      gitService,
		limiter:                         limiter,
		maximumFilesWithContentCount:    maximumFilesWithContentCount,
		maximumFileWithContentSizeBytes: maximumFileWithContentSizeBytes,
		operations:                      newOperations(observationCtx),
	}
}

// InferIndexJobs invokes the given script in a fresh Lua sandbox. The return value of this script
// is assumed to be a table of recognizer instances. Keys conflicting with the default recognizers
// will overwrite them (to disable or change default behavior). Each recognizer's generate function
// is invoked and the resulting index jobs are combined into a flattened list.
func (s *Service) InferIndexJobs(ctx context.Context, repo api.RepoID, commit, overrideScript string) (_ *shared.InferenceResult, err error) {
	ctx, _, endObservation := s.operations.inferIndexJobs.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		repo.Attr(),
		attribute.String("commit", commit),
	}})
	defer endObservation(1, observation.Args{})

	functionTable := invocationFunctionTable{
		linearize: luatypes.LinearizeGenerator,
		callback:  func(recognizer *luatypes.Recognizer) *baselua.LFunction { return recognizer.Generator() },
		scanLuaValue: func(value baselua.LValue) ([]config.IndexJob, error) {
			return util.MapSliceOrSingleton(value, luatypes.IndexJobFromTable)
		},
	}

	jobs, logs, err := s.inferIndexJobs(ctx, repo, commit, overrideScript, functionTable)
	if err != nil {
		return nil, err
	}

	return &shared.InferenceResult{
		IndexJobs:       jobs,
		InferenceOutput: logs,
	}, nil
}

// inferIndexJobs invokes the given script in a fresh Lua sandbox. The return value of this script
// is assumed to be a table of recognizer instances. Keys conflicting with the default recognizers will
// overwrite them (to disable or change default behavior). Each recognizer's callback function is invoked
// and the resulting values are combined into a flattened list. See InferIndexJobs and InferIndexJobHints
// for concrete implementations of the given function table.
func (s *Service) inferIndexJobs(
	ctx context.Context,
	repo api.RepoID,
	commit string,
	overrideScript string,
	invocationContextMethods invocationFunctionTable,
) (_ []config.IndexJob, logs string, _ error) {
	sandbox, err := s.createSandbox(ctx)
	if err != nil {
		return nil, "", err
	}
	defer sandbox.Close()

	var buf bytes.Buffer
	defer func() { logs = buf.String() }()

	invocationContext := invocationContext{
		sandbox:                 sandbox,
		printSink:               &buf,
		gitService:              s.gitService,
		repo:                    repo,
		commit:                  commit,
		invocationFunctionTable: invocationContextMethods,
	}

	recognizers, err := s.setupRecognizers(ctx, invocationContext, overrideScript)
	if err != nil || len(recognizers) == 0 {
		return nil, logs, err
	}

	jobs, err := s.invokeRecognizers(ctx, invocationContext, recognizers)
	return jobs, logs, err
}

// createSandbox creates a Lua sandbox wih the modules loaded for use with auto indexing inference.
func (s *Service) createSandbox(ctx context.Context) (_ *luasandbox.Sandbox, err error) {
	ctx, _, endObservation := s.operations.createSandbox.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	defaultModules, err := defaultModules.Init()
	if err != nil {
		return nil, err
	}
	luaModules, err := luasandbox.LuaModulesFromFS(lua.Scripts, ".", "sg.autoindex")
	if err != nil {
		return nil, err
	}
	opts := luasandbox.CreateOptions{
		GoModules:  defaultModules,
		LuaModules: luaModules,
	}
	sandbox, err := s.sandboxService.CreateSandbox(ctx, opts)
	if err != nil {
		return nil, err
	}

	return sandbox, nil
}

// setupRecognizers runs the given default and override scripts in the given sandbox and converts the
// script return values to a list of recognizer instances.
func (s *Service) setupRecognizers(ctx context.Context, invocationContext invocationContext, overrideScript string) (_ []*luatypes.Recognizer, err error) {
	ctx, _, endObservation := s.operations.setupRecognizers.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	opts := luasandbox.RunOptions{
		PrintSink: invocationContext.printSink,
	}
	rawRecognizers, err := invocationContext.sandbox.RunScriptNamed(ctx, opts, lua.Scripts, "recognizers.lua")
	if err != nil {
		return nil, err
	}

	recognizerMap, err := luatypes.NamedRecognizersFromUserDataMap(rawRecognizers, false)
	if err != nil {
		return nil, err
	}

	if overrideScript != "" {
		rawRecognizers, err := invocationContext.sandbox.RunScript(ctx, opts, overrideScript)
		if err != nil {
			return nil, err
		}

		// Allow false values here, which will be indicated by a nil recognizer. In the loop below we will
		// add (or replace) any recognizer with the same name. To _unset_ a recognizer, we allow a user to
		// add nil as the table value.

		overrideRecognizerMap, err := luatypes.NamedRecognizersFromUserDataMap(rawRecognizers, true)
		if err != nil {
			return nil, err
		}

		for name, recognizer := range overrideRecognizerMap {
			if recognizer == nil {
				delete(recognizerMap, name)
			} else {
				recognizerMap[name] = recognizer
			}
		}
	}

	recognizers := make([]*luatypes.Recognizer, 0, len(recognizerMap))
	for _, recognizer := range recognizerMap {
		recognizers = append(recognizers, recognizer)
	}

	return recognizers, nil
}

// invokeRecognizers invokes each of the given recognizer's callback function and returns the resulting
// index job or hint values. This function is called iteratively with recognizers registered by a previous
// invocation of a recognizer. Calls to gitserver are made in as few batches across all recognizer invocations
// as possible.
func (s *Service) invokeRecognizers(
	ctx context.Context,
	invocationContext invocationContext,
	recognizers []*luatypes.Recognizer,
) (_ []config.IndexJob, err error) {
	ctx, _, endObservation := s.operations.invokeRecognizers.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	patternsForPaths := luatypes.FlattenRecognizerPatterns(recognizers, false)
	patternsForContent := luatypes.FlattenRecognizerPatterns(recognizers, true)

	// Find the list of paths that match either of the partitioned pattern sets. We will feed the
	// concrete paths from this call into the archive call that follows, at least for the concrete
	// paths that match the path-for-content patterns.
	paths, err := s.resolvePaths(ctx, invocationContext, append(patternsForPaths, patternsForContent...))
	if err != nil {
		return nil, err
	}

	contentsByPath, err := s.resolveFileContents(ctx, invocationContext, paths, patternsForContent)
	if err != nil {
		return nil, err
	}

	jobs, err := s.invokeRecognizerChains(ctx, invocationContext, recognizers, paths, contentsByPath)
	if err != nil {
		return nil, err
	}

	return jobs, err
}

// resolvePaths requests all paths matching the given combined regular expression from gitserver. This
// list will likely be a superset of any one recognizer's expected set of paths, so we'll need to filter
// the data before each individual recognizer invocation so that we only pass in what matches the set of
// patterns specific to that recognizer instance.
func (s *Service) resolvePaths(
	ctx context.Context,
	invocationContext invocationContext,
	patternsForPaths []*luatypes.PathPattern,
) (_ []string, err error) {
	ctx, traceLogger, endObservation := s.operations.resolvePaths.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	start := time.Now()
	rateLimitErr := s.limiter.Wait(ctx)
	traceLogger.AddEvent("rate_limit", attribute.Int("wait_duration_ms", int(time.Since(start).Milliseconds())))
	if rateLimitErr != nil {
		return nil, err
	}

	globs, pathspecs, err := flattenPatterns(patternsForPaths, false)
	if err != nil {
		return nil, err
	}

	// Ideally we can pass the globs we explicitly filter by below
	paths, err := invocationContext.gitService.LsFiles(ctx, invocationContext.repo, invocationContext.commit, pathspecs...)
	if err != nil {
		return nil, err
	}

	return filterPaths(paths, globs, nil), nil
}

// resolveFileContents requests the content of the paths that match the given combined regular expression.
// The contents are fetched via a single git archive call.
func (s *Service) resolveFileContents(
	ctx context.Context,
	invocationContext invocationContext,
	paths []string,
	patternsForContent []*luatypes.PathPattern,
) (_ map[string]string, err error) {
	ctx, traceLogger, endObservation := s.operations.resolveFileContents.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	relevantPaths, err := filterPathsByPatterns(paths, patternsForContent)
	if err != nil {
		return nil, err
	}
	if len(relevantPaths) == 0 {
		return nil, nil
	}

	start := time.Now()
	rateLimitErr := s.limiter.Wait(ctx)
	traceLogger.AddEvent("rate_limit", attribute.Int("wait_duration_ms", int(time.Since(start).Milliseconds())))
	if rateLimitErr != nil {
		return nil, err
	}

	opts := gitserver.ArchiveOptions{
		Treeish: invocationContext.commit,
		Format:  gitserver.ArchiveFormatTar,
		Paths:   relevantPaths,
	}
	rc, err := invocationContext.gitService.Archive(ctx, invocationContext.repo, opts)
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	contentsByPath := map[string]string{}

	tr := tar.NewReader(rc)
	for {
		header, err := tr.Next()
		if err != nil {
			if err != io.EOF {
				return nil, err
			}

			break
		}

		if len(contentsByPath) >= s.maximumFilesWithContentCount {
			return nil, LimitError{
				description: fmt.Sprintf(
					"inference limit: requested content for more than %d (%d) files",
					s.maximumFilesWithContentCount,
					len(contentsByPath),
				),
			}
		}
		if int(header.Size) > s.maximumFileWithContentSizeBytes {
			return nil, LimitError{
				description: fmt.Sprintf(
					"inference limit: requested content for a file larger than %d (%d) bytes",
					s.maximumFileWithContentSizeBytes,
					int(header.Size),
				),
			}
		}

		var buf bytes.Buffer
		if _, err := io.CopyN(&buf, tr, header.Size); err != nil {
			return nil, err
		}

		// Since we quoted all literal path specs on entry, we need to remove it from
		// the returned filepaths.
		contentsByPath[strings.TrimPrefix(header.Name, ":(literal)")] = buf.String()
	}

	return contentsByPath, nil
}

type registrationAPI struct {
	recognizers []*luatypes.Recognizer
}

// Register adds another recognizer to be run at a later point.
//
// WARNING: This function is exposed directly to Lua through the 'api' parameter
// of the generate(..) function, so changing the signature may break existing
// auto-indexing scripts.
func (api *registrationAPI) Register(recognizer *luatypes.Recognizer) {
	api.recognizers = append(api.recognizers, recognizer)
}

// invokeRecognizerChains invokes each of the given recognizer's callback function and combines
// their complete output.
func (s *Service) invokeRecognizerChains(
	ctx context.Context,
	invocationContext invocationContext,
	recognizers []*luatypes.Recognizer,
	paths []string,
	contentsByPath map[string]string,
) (jobs []config.IndexJob, _ error) {
	registrationAPI := &registrationAPI{}

	// Invoke the recognizers and gather the resulting jobs or hints
	for _, recognizer := range recognizers {
		additionalJobs, err := s.invokeRecognizerChainUntilResults(
			ctx,
			invocationContext,
			recognizer,
			registrationAPI,
			paths,
			contentsByPath,
		)
		if err != nil {
			return nil, err
		}

		jobs = append(jobs, additionalJobs...)
	}

	if len(registrationAPI.recognizers) != 0 {
		// Recursively call any recognizers that were registered from the previous invocation
		// of recognizers. This allows users to have control over conditional execution so that
		// gitserver data requests re minimal when requested with the expected query patterns.

		additionalJobs, err := s.invokeRecognizers(ctx, invocationContext, registrationAPI.recognizers)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, additionalJobs...)
	}

	return jobs, nil
}

// invokeRecognizerChainUntilResults invokes the callback function from each recognizer reachable
// from the given root recognizer. Once a non-nil error or non-empty set of results are returned
// from a recognizer, the chain invocation halts and the recognizer's index job or hint values
// are returned.
func (s *Service) invokeRecognizerChainUntilResults(
	ctx context.Context,
	invocationContext invocationContext,
	recognizer *luatypes.Recognizer,
	registrationAPI *registrationAPI,
	paths []string,
	contentsByPath map[string]string,
) ([]config.IndexJob, error) {
	for _, recognizer := range invocationContext.linearize(recognizer) {
		if jobs, err := s.invokeLinearizedRecognizer(
			ctx,
			invocationContext,
			recognizer,
			registrationAPI,
			paths,
			contentsByPath,
		); err != nil || len(jobs) > 0 {
			return jobs, err
		}
	}

	return nil, nil
}

// invokeLinearizedRecognizer invokes a single recognizer callback.
func (s *Service) invokeLinearizedRecognizer(
	ctx context.Context,
	invocationContext invocationContext,
	recognizer *luatypes.Recognizer,
	registrationAPI *registrationAPI,
	paths []string,
	contentsByPath map[string]string,
) (_ []config.IndexJob, err error) {
	ctx, _, endObservation := s.operations.invokeLinearizedRecognizer.With(ctx, &err, observation.Args{})
	defer endObservation(1, observation.Args{})

	callPaths, callContentsByPath, err := s.filterPathsForRecognizer(recognizer, paths, contentsByPath)
	if err != nil {
		return nil, err
	}
	if len(callPaths) == 0 && len(callContentsByPath) == 0 {
		return nil, nil
	}

	opts := luasandbox.RunOptions{
		PrintSink: invocationContext.printSink,
	}
	args := []any{registrationAPI, callPaths, callContentsByPath}
	value, err := invocationContext.sandbox.Call(ctx, opts, invocationContext.callback(recognizer), args...)
	if err != nil {
		return nil, err
	}

	jobs, err := invocationContext.scanLuaValue(value)
	if err != nil {
		return nil, err
	}

	return jobs, nil
}

// filterPathsForRecognizer creates a copy of the the given path slice and file content map
// that only contain elements matching the patterns attached to the given recognizer.
func (s *Service) filterPathsForRecognizer(
	recognizer *luatypes.Recognizer,
	paths []string,
	contentsByPath map[string]string,
) ([]string, map[string]string, error) {
	// Filter out paths which are not interesting to this recognizer
	filteredPaths, err := filterPathsByPatterns(paths, recognizer.Patterns(false))
	if err != nil {
		return nil, nil, err
	}

	// Filter out paths which are not interesting to this recognizer
	filteredPathsWithContent, err := filterPathsByPatterns(paths, recognizer.Patterns(true))
	if err != nil {
		return nil, nil, err
	}

	// Copy over content for remaining paths in map
	filteredContentsByPath := make(map[string]string, len(filteredPathsWithContent))
	for _, key := range filteredPathsWithContent {
		filteredContentsByPath[key] = contentsByPath[key]
	}

	return filteredPaths, filteredContentsByPath, nil
}
