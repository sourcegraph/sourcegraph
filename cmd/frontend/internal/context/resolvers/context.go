package resolvers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/sourcegraph/conc/iter"
	"github.com/sourcegraph/log"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/cody"
	"github.com/sourcegraph/sourcegraph/internal/conf"
	"github.com/sourcegraph/sourcegraph/internal/dotcom"
	"github.com/sourcegraph/sourcegraph/internal/httpcli"
	"github.com/sourcegraph/sourcegraph/schema"

	"github.com/sourcegraph/sourcegraph/cmd/frontend/graphqlbackend"
	"github.com/sourcegraph/sourcegraph/cmd/frontend/internal/codycontext"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/trace"
	"github.com/sourcegraph/sourcegraph/internal/types"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"

	cohere "github.com/cohere-ai/cohere-go/v2"
	"github.com/cohere-ai/cohere-go/v2/client"
)

func NewResolver(db database.DB, gitserverClient gitserver.Client, contextClient *codycontext.CodyContextClient, logger log.Logger) graphqlbackend.CodyContextResolver {
	return &Resolver{
		db:                  db,
		gitserverClient:     gitserverClient,
		contextClient:       contextClient,
		logger:              logger,
		intentApiHttpClient: httpcli.UncachedExternalDoer,
		intentBackendConfig: conf.CodyIntentConfig(),
		reranker:            conf.CodyReranker(),
		cohereConfig:        conf.CodyRerankerCohereConfig(),
	}
}

type Resolver struct {
	db                  database.DB
	gitserverClient     gitserver.Client
	contextClient       *codycontext.CodyContextClient
	logger              log.Logger
	intentApiHttpClient httpcli.Doer
	intentBackendConfig *schema.IntentDetectionAPI
	reranker            conf.CodyRerankerBackend
	cohereConfig        *schema.CodyRerankerCohere
}

func (r *Resolver) RecordContext(ctx context.Context, args graphqlbackend.RecordContextArgs) (*graphqlbackend.EmptyResponse, error) {
	err := r.contextApiEnabled(ctx)
	if err != nil {
		return nil, err
	}
	retrieverUsed, retrieverDiscarded := map[string]int{}, map[string]int{}
	for _, item := range args.UsedContextItems {
		retrieverUsed[item.Retriever]++
	}
	for _, item := range args.DiscardedContextItems {
		retrieverDiscarded[item.Retriever]++
	}
	fields := []log.Field{log.String("interactionID", args.InteractionID), log.Int("includedItemCount", len(args.UsedContextItems)), log.Int("excludedItemCount", len(args.DiscardedContextItems))}
	for r, cnt := range retrieverUsed {
		fields = append(fields, log.Int(r+"-used", cnt))
	}
	for r, cnt := range retrieverDiscarded {
		fields = append(fields, log.Int(r+"-discarded", cnt))
	}
	r.logger.Info("recording context", fields...)
	return nil, nil
}

func (r *Resolver) RankContext(ctx context.Context, args graphqlbackend.RankContextArgs) (graphqlbackend.RankContextResolver, error) {
	err := r.contextApiEnabled(ctx)
	if err != nil {
		return nil, err
	}
	ranker, used, err := r.rerank(ctx, args)
	if err != nil {
		return nil, err
	}
	res := rankContextResponse{
		ranker: string(ranker),
		used:   used,
	}
	r.logger.Info("ranking context", log.String("interactionID", args.InteractionID), log.String("ranker", res.ranker), log.Int("contextItemCount", len(args.ContextItems)))
	return res, nil
}

func (r *Resolver) GetCodyContext(ctx context.Context, args graphqlbackend.GetContextArgs) (_ []graphqlbackend.ContextResultResolver, err error) {
	repoIDs, err := graphqlbackend.UnmarshalRepositoryIDs(args.Repos)
	if err != nil {
		return nil, err
	}

	repos, err := r.db.Repos().GetReposSetByIDs(ctx, repoIDs...)
	if err != nil {
		return nil, err
	}

	repoNameIDs := make([]types.RepoIDName, len(repoIDs))
	for i, repoID := range repoIDs {
		repo, ok := repos[repoID]
		if !ok {
			// GetReposSetByIDs does not error if a repo could not be found.
			return nil, errors.Newf("could not find repo with id %d", int32(repoID))
		}

		repoNameIDs[i] = types.RepoIDName{ID: repoID, Name: repo.Name}
	}

	fileChunks, err := r.contextClient.GetCodyContext(ctx, codycontext.GetContextArgs{
		Repos:            repoNameIDs,
		Query:            args.Query,
		CodeResultsCount: args.CodeResultsCount,
		TextResultsCount: args.TextResultsCount,
	})
	if err != nil {
		return nil, err
	}

	tr, ctx := trace.New(ctx, "resolveChunks")
	defer tr.EndWithErr(&err)

	return iter.MapErr(fileChunks, func(fileChunk *codycontext.FileChunkContext) (graphqlbackend.ContextResultResolver, error) {
		return r.fileChunkToResolver(ctx, fileChunk)
	})
}

// ChatIntent is a quick-and-dirty way to expose our intent detection model to Cody clients.
// Yes, it does things that should not be done in production code - for now it is just a proof of concept for demos.
func (r *Resolver) ChatIntent(ctx context.Context, args graphqlbackend.ChatIntentArgs) (graphqlbackend.IntentResolver, error) {
	err := r.contextApiEnabled(ctx)
	if err != nil {
		return nil, err
	}
	backend := r.intentBackendConfig
	if backend == nil || backend.Default == nil {
		return nil, errors.New("intent detection backend not configured")
	}
	intentRequest := intentApiRequest{Query: args.Query}
	buf, err := json.Marshal(&intentRequest)
	if err != nil {
		return nil, err
	}
	intentResponse, err := r.sendIntentRequest(ctx, *backend.Default, buf)
	// ignore cancellation from top-level context - we allow extra requests to extend beyond the lifetime of parent request, but we'll rely on short timeouts to make sure they don't last too long
	extraContext := context.WithoutCancel(ctx)
	iter.ForEach(backend.Extra, func(extraBackend **schema.BackendAPIConfig) {
		if *extraBackend == nil {
			return
		}
		response, err := r.sendIntentRequest(extraContext, **extraBackend, buf)
		if err != nil {
			r.logger.Warn("error fetching intent from extra backend", log.String("interactionID", args.InteractionID), log.String("backend", (*extraBackend).Url), log.Error(err))
			return
		}
		r.logger.Debug("fetched intent from extra backend", log.String("interactionID", args.InteractionID), log.String("backend", (*extraBackend).Url), log.String("query", args.Query), log.String("intent", response.Intent), log.Float64("score", response.Score))
	})
	if err != nil {
		return nil, err
	}
	r.logger.Info("detecting intent", log.String("interactionID", args.InteractionID), log.String("query", args.Query), log.String("intent", intentResponse.Intent), log.Float64("score", intentResponse.Score))
	return &chatIntentResponse{intent: intentResponse.Intent, score: intentResponse.Score}, nil
}

func (r *Resolver) sendIntentRequest(ctx context.Context, backend schema.BackendAPIConfig, request []byte) (*intentApiResponse, error) {
	// Fail-fast
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// Proof-of-concept warning - this needs to be deployed behind Cody Gateway, or exposed with HTTPS and authentication.
	req, err := http.NewRequestWithContext(ctx, "POST", backend.Url, bytes.NewReader(request))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if backend.AuthHeader != "" {
		req.Header.Set("Authorization", backend.AuthHeader)
	}
	resp, err := r.intentApiHttpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var intentResponse intentApiResponse
	err = json.Unmarshal(body, &intentResponse)
	if err != nil {
		return nil, err
	}
	return &intentResponse, nil
}

func (r *Resolver) contextApiEnabled(ctx context.Context) error {
	if !dotcom.SourcegraphDotComMode() {
		return errors.New("this feature is only available on sourcegraph.com")
	}
	if isEnabled, reason := cody.IsCodyEnabled(ctx, r.db); !isEnabled {
		return errors.Newf("cody is not enabled: %s", reason)
	}
	if err := cody.CheckVerifiedEmailRequirement(ctx, r.db, r.logger); err != nil {
		return err
	}
	return nil
}

type intentApiRequest struct {
	Query string `json:"query"`
}

type intentApiResponse struct {
	Intent string  `json:"intent"`
	Score  float64 `json:"score"`
}

type chatIntentResponse struct {
	intent string
	score  float64
}

func (r *chatIntentResponse) Intent() string {
	return r.intent
}
func (r *chatIntentResponse) Score() float64 {
	return r.score
}

// The rough size of a file chunk in runes. The value 1024 is due to historical reasons -- Cody context was once based
// on embeddings, and we chunked files into ~1024 characters (aiming for 256 tokens, assuming each token takes 4
// characters on average).
//
// Ideally, the caller would pass a token 'budget' and we'd use a tokenizer and attempt to exactly match this budget.
const chunkSizeRunes = 1024

func (r *Resolver) fileChunkToResolver(ctx context.Context, chunk *codycontext.FileChunkContext) (graphqlbackend.ContextResultResolver, error) {
	repoResolver := graphqlbackend.NewMinimalRepositoryResolver(r.db, r.gitserverClient, chunk.RepoID, chunk.RepoName)

	commitResolver := graphqlbackend.NewGitCommitResolver(r.db, r.gitserverClient, repoResolver, chunk.CommitID, nil)
	stat, err := r.gitserverClient.Stat(ctx, chunk.RepoName, chunk.CommitID, chunk.Path)
	if err != nil {
		return nil, err
	}

	gitTreeEntryResolver := graphqlbackend.NewGitTreeEntryResolver(r.db, r.gitserverClient, graphqlbackend.GitTreeEntryResolverOpts{
		Commit: commitResolver,
		Stat:   stat,
	})

	// Populate content ahead of time so we can do it concurrently
	content, err := gitTreeEntryResolver.Content(ctx, &graphqlbackend.GitTreeContentPageArgs{
		StartLine: pointers.Ptr(int32(chunk.StartLine)),
	})
	if err != nil {
		return nil, err
	}

	numLines := countLines(content, chunkSizeRunes)
	endLine := chunk.StartLine + numLines - 1 // subtract 1 because endLine is inclusive
	return graphqlbackend.NewFileChunkContextResolver(gitTreeEntryResolver, chunk.StartLine, endLine), nil
}

func (r *Resolver) rerank(ctx context.Context, args graphqlbackend.RankContextArgs) (conf.CodyRerankerBackend, []int32, error) {
	if r.reranker == conf.CodyRerankerIdentity {
		var used []int32
		for i := range args.ContextItems {
			used = append(used, int32(i))
		}
		return conf.CodyRerankerIdentity, used, nil
	}
	co := client.NewClient(client.WithToken(r.cohereConfig.ApiKey))

	req := &cohere.RerankRequest{
		Query: args.Query,
		Model: cohere.String(r.cohereConfig.Model),
	}
	for _, ci := range args.ContextItems {
		req.Documents = append(req.Documents, &cohere.RerankRequestDocumentsItem{String: ci.Content})
	}
	resp, err := co.Rerank(ctx, req)
	if err != nil {
		r.logger.Error("cohere reranking error", log.String("interactionId", args.InteractionID), log.String("query", args.Query), log.Error(err))
		return conf.CodyRerankerCohere, nil, err
	}
	var used []int32
	for _, r := range resp.Results {
		used = append(used, int32(r.Index))
	}
	return conf.CodyRerankerCohere, used, nil
}

// countLines finds the number of lines corresponding to the number of runes. We 'round down'
// to ensure that we don't return more characters than our budget.
func countLines(content string, numRunes int) int {
	if len(content) == 0 {
		return 0
	}

	if content[len(content)-1] != '\n' {
		content += "\n"
	}

	runes := []rune(content)
	truncated := runes[:min(len(runes), numRunes)]
	in := []byte(string(truncated))
	return bytes.Count(in, []byte("\n"))
}

type rankContextResponse struct {
	ranker    string
	used      []int32
	discarded []int32
}

func (r rankContextResponse) Ranker() string {
	return r.ranker
}

func (r rankContextResponse) Used() []int32 {
	return r.used
}

func (r rankContextResponse) Discarded() []int32 {
	return r.discarded
}

var _ graphqlbackend.RankContextResolver = &rankContextResponse{}
