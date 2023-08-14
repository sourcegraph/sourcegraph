package context

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/sourcegraph/scip/bindings/go/scip"
	"go.opentelemetry.io/otel/attribute"
	"google.golang.org/protobuf/proto"

	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/authz"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav"
	codenavtypes "github.com/sourcegraph/sourcegraph/internal/codeintel/codenav"
	codenavshared "github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/shared"
	resolverstubs "github.com/sourcegraph/sourcegraph/internal/codeintel/resolvers"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/symbols"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/types"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/database"
	"github.com/sourcegraph/sourcegraph/internal/gitserver"
	"github.com/sourcegraph/sourcegraph/internal/gitserver/gitdomain"
	"github.com/sourcegraph/sourcegraph/internal/gosyntect"
	"github.com/sourcegraph/sourcegraph/internal/observation"
)

type Service struct {
	codenavSvc      CodeNavService
	repostore       database.RepoStore
	syntectClient   *gosyntect.Client
	gitserverClient gitserver.Client
	operations      *operations
}

func newService(
	observationCtx *observation.Context,
	repostore database.RepoStore,
	codenavSvc CodeNavService,
	syntectClient *gosyntect.Client,
	gitserverClient gitserver.Client,
) *Service {
	return &Service{
		codenavSvc:      codenavSvc,
		repostore:       repostore,
		syntectClient:   syntectClient,
		gitserverClient: gitserverClient,
		operations:      newOperations(observationCtx),
	}
}

// Flagrantly taken from default value in enterprise/cmd/frontend/internal/codeintel/config.go
const (
	maximumIndexesPerMonikerSearch = 500
	hunkCacheSize                  = 1000
	enableSyntect                  = true
)

func (s *Service) GetPreciseContext(ctx context.Context, args *resolverstubs.GetPreciseContextInput) (_ []*types.PreciseContext, traceLogs string, err error) {
	ctx, trace, endObservation := s.operations.getPreciseContext.With(ctx, &err, observation.Args{Attrs: []attribute.KeyValue{
		attribute.String("repositoryName", args.Input.RepositoryName),
		attribute.String("content", args.Input.ActiveFileContent),
		attribute.String("closestRemoteCommitSHA", args.Input.ClosestRemoteCommitSHA),
	}})
	defer endObservation(1, observation.Args{})

	logBuf := new(bytes.Buffer)
	debug := func(v any) {
		s, _ := json.MarshalIndent(v, "", "    ")
		fmt.Print(string(s) + "\n")
		logBuf.WriteString(string(s) + "\n")
	}
	defer func() { traceLogs = logBuf.String() }()

	filename := args.Input.ActiveFile
	content := args.Input.ActiveFileContent
	repoName := api.RepoName(args.Input.RepositoryName)
	activeRangeSelection := translateToScipRange(args.Input.ActiveFileSelectionRange)

	repo, err := s.repostore.GetByName(ctx, repoName)
	if err != nil {
		return nil, "", err
	}

	commitID, err := s.gitserverClient.ResolveRevision(ctx, repoName, args.Input.ClosestRemoteCommitSHA, gitserver.ResolveRevisionOptions{})
	if err != nil {
		return nil, "", err
	}
	closestRemoteCommitSHA := string(commitID)

	uploads, err := s.codenavSvc.GetClosestDumpsForBlob(ctx, int(repo.ID), closestRemoteCommitSHA, filename, true, "")
	if err != nil {
		return nil, "", err
	}
	trace.AddEvent("codenavSvc.GetClosestDumpsForBlob", attribute.Int("numDumps", len(uploads)))
	if len(uploads) == 0 {
		return nil, "", nil
	}

	requestArgs := codenavtypes.RequestArgs{
		RepositoryID: int(repo.ID),
		Commit:       closestRemoteCommitSHA,
		Limit:        100, //! MAGIC NUMBER
		RawCursor:    "",
	}
	hunkCache, err := codenav.NewHunkCache(hunkCacheSize)
	if err != nil {
		return nil, "", err
	}
	reqState := codenavtypes.NewRequestState(
		uploads,
		s.repostore,
		authz.DefaultSubRepoPermsChecker,
		s.gitserverClient,
		repo,
		closestRemoteCommitSHA,
		"",
		maximumIndexesPerMonikerSearch,
		hunkCache,
	)

	// DEBUGGING
	start := time.Now()
	phaseStart := start
	lap := func(values map[string]any) {
		n := time.Now()
		phaseDelta := n.Sub(phaseStart)
		cumulativeDelta := n.Sub(start)
		phaseStart = n

		values["phaseDeltaMs"] = phaseDelta
		values["phaseDelta"] = phaseDelta.String()
		values["cumulativeDeltaMs"] = cumulativeDelta
		values["cumulativeDelta"] = cumulativeDelta.String()
		debug(values)
	}
	debug(map[string]any{"event": "context api start"})
	defer func() { lap(map[string]any{"state": "context api return", "err": err}) }()
	fuzzyNameSetBySymbol := map[string]map[string]struct{}{}
	if enableSyntect {
		// PHASE 1: Run current scope through treesitter
		syntectDocument, err := s.getSCIPDocumentByContent(ctx, content, filename)
		if err != nil {
			return nil, "", err
		}
		trace.AddEvent("contextSvc.getSCIPDocumentByContent", attribute.String("filename", filename))

		fuzzySymbolNameSet := precise.NewSet[string]()
		for _, occurrence := range syntectDocument.Occurrences {
			if activeRangeSelection == nil || precise.IsOccurrenceWithinRange(activeRangeSelection, occurrence) {
				fuzzySymbolNameSet.Add(occurrence.Symbol)
			}
		}

		fuzzySymbolNames := fuzzySymbolNameSet.ToSlice()
		sort.Strings(fuzzySymbolNames)
		trace.AddEvent("fuzzySymbolNames", attribute.StringSlice("fuzzyNames", fuzzySymbolNames))

		uploadIDs := make([]int, len(uploads))
		for i, upload := range uploads {
			uploadIDs[i] = upload.ID
		}
		// DEBUGGING
		lap(map[string]any{"event": "phase 1", "fuzzySymbolNames": fuzzySymbolNames})

		// PHASE 2: Run treesitter output through a translation layer so we can do
		// the graph navigation in "SCIP-world" using proper identifiers. The following
		// code is pretty sloppy right now since we haven't consolidated on a single way
		// to "match" descriptors together. This should align in the db layer as well.
		//
		// This isn't a deep technical problem, just one of deciding on a thing and
		// conforming to/communicating it in the codebase.

		// Construct a map from syntect (fuzzy) name to a list of SCIP names matching the syntect
		// output. I'd like to have the `GetFullSCIPNameByDescriptor` method create this
		// mapping instead. This block should become a single function call after that
		// transformation.

		scipNamesByFuzzyName, err := func() (map[string][]*symbols.ExplodedSymbol, error) {
			explodedScipNames, err := s.codenavSvc.GetFullSCIPNameByDescriptor(ctx, uploadIDs, fuzzySymbolNames)
			if err != nil {
				return nil, err
			}

			// DEBUGGING
			// for _, scipName := range explodedScipNames {
			// 	debug(map[string]any{"event": "[log] exploded scip names", "descriptor suffix": scipName.DescriptorSuffix})
			// }

			explodedScipSymbolsByFuzzyName := map[string][]*symbols.ExplodedSymbol{}
			for _, fuzzyName := range fuzzySymbolNames {
				ex, err := symbols.NewExplodedSymbol(fuzzyName)
				if err != nil {
					trace.AddEvent("NewExplodedSymbol error", attribute.String("exploded symbol err", err.Error()))
					continue
				}
				var explodedScipSymbols []*symbols.ExplodedSymbol
				for _, esn := range explodedScipNames {
					// N.B. this matches what we search against in fuzzyDescriptorSuffixConditions
					if !strings.HasSuffix(esn.DescriptorSuffix, ex.DescriptorSuffix) {
						continue
					}
					explodedScipSymbols = append(explodedScipSymbols, esn)
				}

				// DEBUGGING
				if len(explodedScipSymbols) == 0 {
					ex, _ := symbols.NewExplodedSymbol(fuzzyName)
					debug(map[string]any{"event": "[log] no match for symbol", "fuzzy name": fuzzyName, "descriptor suffix": ex.DescriptorSuffix})
				}

				if len(explodedScipSymbols) > 20 {
					// DEBUGGING
					debug(map[string]any{"event": "[log] too many matches for symbol", "fuzzy name": fuzzyName, "descriptor suffix": ex.DescriptorSuffix})
					trace.AddEvent("TOO MANY RESULTS", attribute.String("syntectName", fuzzyName))
					explodedScipSymbols = nil
				}

				if len(explodedScipSymbols) > 0 {
					explodedScipSymbolsByFuzzyName[fuzzyName] = explodedScipSymbols
				}
			}

			trace.AddEvent(
				"num of explodedScipSymbolsByFuzzyName",
				attribute.Int(
					"length of explodedScipSymbolsByFuzzyName",
					len(explodedScipSymbolsByFuzzyName),
				),
			)

			return explodedScipSymbolsByFuzzyName, nil
		}()
		if err != nil {
			return nil, "", err
		}

		for fuzzyName, explodedSymbols := range scipNamesByFuzzyName {
			for _, explodedSymbol := range explodedSymbols {
				symbol := explodedSymbol.Symbol()
				if _, ok := fuzzyNameSetBySymbol[symbol]; !ok {
					fuzzyNameSetBySymbol[symbol] = map[string]struct{}{}
				}

				fuzzyNameSetBySymbol[symbol][fuzzyName] = struct{}{}
			}
		}

		// DEBUGGING
		lap(map[string]any{"event": "phase 2", "fuzzy names by symbol": fuzzyNameSetBySymbol})
	} else {
		symbolsNames, err := s.codenavSvc.GetSymbolNamesByRange(ctx, requestArgs, filename, reqState, activeRangeSelection)
		if err != nil {
			return nil, "", err
		}

		for _, symbolName := range symbolsNames {
			fuzzyNameSetBySymbol[symbolName] = map[string]struct{}{"": {}}
		}
	}

	// PHASE 3: Gather definitions for each relevant SCIP symbol

	type preciseData struct {
		SCIP     string
		Fuzzy    string
		Location []codenavshared.UploadLocation `json:"-"`
	}
	preciseDataList := []*preciseData{}

	symbolNames := make([]string, 0, len(fuzzyNameSetBySymbol))
	for symbolName := range fuzzyNameSetBySymbol {
		symbolNames = append(symbolNames, symbolName)
	}

	ul, err := s.codenavSvc.NewGetDefinitionsBySymbolNames(ctx, requestArgs, reqState, symbolNames)
	if err != nil {
		return nil, "", err
	}

	for _, location := range ul {
		for fzn := range fuzzyNameSetBySymbol[location.SymbolName] {
			preciseDataList = append(preciseDataList, &preciseData{
				SCIP:     location.SymbolName,
				Fuzzy:    fzn,
				Location: []codenavshared.UploadLocation{location},
			})
		}
	}

	trace.AddEvent("preciseDataList", attribute.Int("fuzzyName", len(preciseDataList)))

	// DEBUGGING
	lap(map[string]any{"event": "phase 3", "precise data list": preciseDataList})

	// PHASE 4: Read the files that contain a definition
	filesByRepo := map[string]struct {
		paths map[gitdomain.Pathspec]struct{}
		dump  shared.Dump
	}{}

	cache := map[string]DocumentAndText{}
	for _, pd := range preciseDataList {
		for _, l := range pd.Location {
			repoCommitKey := fmt.Sprintf("%s@%s", l.Dump.RepositoryName, l.Dump.Commit)

			px := filesByRepo[repoCommitKey].paths
			if px == nil {
				px = map[gitdomain.Pathspec]struct{}{}
			}
			px[gitdomain.Pathspec(l.Path)] = struct{}{}
			filesByRepo[repoCommitKey] = struct {
				paths map[gitdomain.Pathspec]struct{}
				dump  shared.Dump
			}{
				dump:  l.Dump,
				paths: px,
			}
		}
	}

	fileToPath := map[string]struct {
		path string
		dump shared.Dump
	}{}
	for repoCommitKey, path := range filesByRepo {
		parts := strings.Split(repoCommitKey, "@")
		repo := api.RepoName(parts[0])
		pathspec := []gitdomain.Pathspec{}
		for key := range path.paths {
			pathspec = append(pathspec, key)
		}
		opts := gitserver.ArchiveOptions{
			Treeish:   parts[1],
			Format:    gitserver.ArchiveFormatTar,
			Pathspecs: pathspec,
		}
		rc, err := s.gitserverClient.ArchiveReader(ctx, authz.DefaultSubRepoPermsChecker, repo, opts)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()

		tr := tar.NewReader(rc)
		for {
			header, err := tr.Next()
			if err != nil {
				if err != io.EOF {
					return nil, "", err
				}

				break
			}

			var buf bytes.Buffer
			if _, err := io.CopyN(&buf, tr, header.Size); err != nil {
				return nil, "", err
			}

			// Since we quoted all literal path specs on entry, we need to remove it from
			// the returned filepaths.
			file := buf.String()
			p := strings.TrimPrefix(header.Name, ":(literal)")
			fileToPath[file] = struct {
				path string
				dump shared.Dump
			}{
				path: p,
				dump: path.dump,
			}
		}
	}

	for file, path := range fileToPath {
		syntectDocs, err := s.getSCIPDocumentByContent(ctx, file, path.path)
		if err != nil {
			return nil, "", err
		}
		key := fmt.Sprintf("%s@%s:%s", path.dump.RepositoryName, path.dump.Commit, filepath.Join(path.dump.Root, path.path))
		cache[key] = NewDocumentAndText(file, syntectDocs)
	}

	// DEBUGGING
	lap(map[string]any{"event": "phase 4"})

	// PHASE 5: Extract the definitions for each of the relevant syntect symbols
	// we originally requested.
	//
	// NOTE: I make an assumption here that the symbols will be equal as
	// they were both generated by the same treesitter process. See the
	// inline note below.

	preciseResponse := []*types.PreciseContext{}
	for _, pd := range preciseDataList {
		for _, l := range pd.Location {
			key := fmt.Sprintf("%s@%s:%s", l.Dump.RepositoryName, l.Dump.Commit, filepath.Join(l.Dump.Root, l.Path))
			documentAndText := cache[key]

			for _, occ := range documentAndText.SCIP.Occurrences {
				// NOTE: assumption made; we may want to look at the precise
				// range as an alternate or additional indicator for which
				// syntect occurrences we are interested in
				if occ.Symbol != pd.Fuzzy && pd.Fuzzy != "" {
					continue
				}
				if len(occ.EnclosingRange) > 0 {
					ex, err := symbols.NewExplodedSymbol(pd.SCIP)
					if err != nil {
						return nil, "", err
					}
					fex, err := symbols.NewExplodedSymbol(pd.Fuzzy)
					if err != nil {
						return nil, "", err
					}
					var fuzzyName *string
					if fex.FuzzyDescriptorSuffix != "" {
						fuzzyName = &fex.FuzzyDescriptorSuffix
					}

					preciseResponse = append(preciseResponse, &types.PreciseContext{
						Symbol: types.PreciseSymbolReference{
							ScipName:         pd.SCIP,
							DescriptorSuffix: ex.DescriptorSuffix,
							FuzzyName:        fuzzyName,
						},
						RepositoryName:    l.Dump.RepositoryName,
						DefinitionSnippet: documentAndText.Extract(scip.NewRange(occ.EnclosingRange)),
						Filepath:          l.Path,
						Location:          l,
					})
				}
			}
		}
	}
	trace.AddEvent("preciseResponse", attribute.Int("length of preciseResponse", len(preciseResponse)))

	// DEBUGGING
	lap(map[string]any{"event": "phase 5", "response": preciseResponse})
	return preciseResponse, "", nil
}

func (s *Service) getSCIPDocumentByContent(ctx context.Context, content, fileName string) (*scip.Document, error) {
	q := gosyntect.SymbolsQuery{
		FileName: fileName,
		Content:  content,
	}

	resp, err := s.syntectClient.Symbols(ctx, &q)
	if err != nil {
		return nil, err
	}

	d, err := base64.StdEncoding.DecodeString(resp.Scip)
	if err != nil {
		fmt.Println("ERROR: ", err)
		return nil, err
	}

	var document scip.Document
	if err := proto.Unmarshal(d, &document); err != nil {
		fmt.Println("ERROR: ", err)
		return nil, err
	}

	return &document, nil
}

func (s *Service) SplitIntoEmbeddableChunks(ctx context.Context, text string, fileName string, splitOptions SplitOptions) ([]EmbeddableChunk, error) {
	return SplitIntoEmbeddableChunks(text, fileName, splitOptions), nil
}

func translateToScipRange(ar *resolverstubs.ActiveFileSelectionRangeInput) (r *scip.Range) {
	if ar == nil {
		return nil
	}

	return &scip.Range{
		Start: scip.Position{Line: int32(ar.StartLine), Character: int32(ar.StartCharacter)},
		End:   scip.Position{Line: int32(ar.EndLine), Character: int32(ar.EndCharacter)},
	}
}
