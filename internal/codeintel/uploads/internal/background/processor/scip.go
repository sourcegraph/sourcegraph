package processor

import (
	"bytes"
	"context"
	"io"

	"github.com/sourcegraph/log"
	"github.com/sourcegraph/scip/bindings/go/scip"
	"go.opentelemetry.io/otel/attribute"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/internal/lsifstore"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/internal/collections"
	"github.com/sourcegraph/sourcegraph/internal/observation"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/pathexistence"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
)

type firstPassResult struct {
	metadata              *scip.Metadata
	externalSymbolsByName map[string]*scip.SymbolInformation
	relativePaths         []string
	documentCountByPath   map[string]int
}

func aggregateExternalSymbolsAndPaths(indexReader *gzipReadSeeker) (firstPassResult, error) {
	var metadata *scip.Metadata
	var paths []string
	externalSymbolsByName := make(map[string]*scip.SymbolInformation, 1024)
	documentCountByPath := make(map[string]int, 1)
	indexVisitor := scip.IndexVisitor{
		VisitMetadata: func(m *scip.Metadata) {
			metadata = m
		},
		// Assumption: Post-processing of documents is much more expensive than
		// pure deserialization, so we don't optimize the visitation here to support
		// only deserializing the RelativePath and skipping other fields.
		VisitDocument: func(d *scip.Document) {
			paths = append(paths, d.RelativePath)
			documentCountByPath[d.RelativePath] = documentCountByPath[d.RelativePath] + 1
		},
		VisitExternalSymbol: func(s *scip.SymbolInformation) {
			externalSymbolsByName[s.Symbol] = s
		},
	}
	if err := indexVisitor.ParseStreaming(indexReader); err != nil {
		return firstPassResult{}, err
	}
	if err := indexReader.seekToStart(); err != nil {
		return firstPassResult{}, err
	}
	return firstPassResult{metadata, externalSymbolsByName, paths, documentCountByPath}, nil
}

type documentOneShotIterator struct {
	ignorePaths  collections.Set[string]
	indexSummary firstPassResult
	indexReader  gzipReadSeeker
}

var _ lsifstore.SCIPDocumentVisitor = &documentOneShotIterator{}

func (it *documentOneShotIterator) VisitAllDocuments(
	ctx context.Context,
	logger log.Logger,
	p *lsifstore.ProcessedPackageData,
	doIt func(lsifstore.ProcessedSCIPDocument) error,
) error {
	repeatedDocumentsByPath := make(map[string][]*scip.Document, 1)
	packageSet := map[precise.Package]bool{}

	var outerError error = nil

	secondPassVisitor := scip.IndexVisitor{VisitDocument: func(currentDocument *scip.Document) {
		path := currentDocument.RelativePath
		if it.ignorePaths.Has(path) {
			return
		}
		document := currentDocument
		if docCount := it.indexSummary.documentCountByPath[path]; docCount > 1 {
			samePathDocs := append(repeatedDocumentsByPath[path], document)
			repeatedDocumentsByPath[path] = samePathDocs
			if len(samePathDocs) != docCount {
				// The document will be processed later when all other Documents
				// with the same path are seen.
				return
			}
			flattenedDoc := scip.FlattenDocuments(samePathDocs)
			delete(repeatedDocumentsByPath, path)
			if len(flattenedDoc) != 1 {
				logger.Warn("FlattenDocuments should return a single Document as input slice contains Documents"+
					" with the same RelativePath",
					log.String("path", path),
					log.Int("obtainedCount", len(flattenedDoc)))
				return
			}
			document = flattenedDoc[0]
		}

		if ctx.Err() != nil {
			outerError = ctx.Err()
			return
		}
		if err := doIt(processDocument(document, it.indexSummary.externalSymbolsByName)); err != nil {
			outerError = err
			return
		}

		// While processing this document, stash the unique packages of each symbol name
		// in the document. If there is an occurrence that defines that symbol, mark that
		// package as being one that we define (rather than simply reference).

		for _, symbol := range document.Symbols {
			if pkg, ok := packageFromSymbol(symbol.Symbol); ok {
				// no-op if key exists; add false if key is absent
				packageSet[pkg] = packageSet[pkg] || false
			}

			for _, relationship := range symbol.Relationships {
				if pkg, ok := packageFromSymbol(relationship.Symbol); ok {
					// no-op if key exists; add false if key is absent
					packageSet[pkg] = packageSet[pkg] || false
				}
			}
		}

		for _, occurrence := range document.Occurrences {
			if occurrence.Symbol == "" || scip.IsLocalSymbol(occurrence.Symbol) {
				continue
			}

			if pkg, ok := packageFromSymbol(occurrence.Symbol); ok {
				if isDefinition := scip.SymbolRole_Definition.Matches(occurrence); isDefinition {
					packageSet[pkg] = true
				} else {
					// no-op if key exists; add false if key is absent
					packageSet[pkg] = packageSet[pkg] || false
				}
			}
		}
	},
	}
	if err := secondPassVisitor.ParseStreaming(&it.indexReader); err != nil {
		logger.Warn("error on second pass over SCIP index; should've hit it in the first pass",
			log.Error(err))
	}
	if outerError != nil {
		return outerError
	}
	// Reset state in case we want to read documents again
	if err := it.indexReader.seekToStart(); err != nil {
		return err
	}

	// Now that we've populated our index-global packages map, separate them into ones that
	// we define and ones that we simply reference. The closing of the documents channel at
	// the end of this function will signal that these lists have been populated.

	for pkg, hasDefinition := range packageSet {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if hasDefinition {
			p.Packages = append(p.Packages, pkg)
		} else {
			p.PackageReferences = append(p.PackageReferences, precise.PackageReference{Package: pkg})
		}
	}

	return nil
}

// prepareSCIPDataStream performs a streaming traversal of the index to get some preliminary
// information, and creates a SCIPDataStream that can be used to write Documents into the database.
//
// Package information can be obtained when documents are visited.
func prepareSCIPDataStream(
	ctx context.Context,
	indexReader gzipReadSeeker,
	root string,
	getChildren pathexistence.GetChildrenFunc,
) (lsifstore.SCIPDataStream, error) {
	indexSummary, err := aggregateExternalSymbolsAndPaths(&indexReader)
	if err != nil {
		return lsifstore.SCIPDataStream{}, err
	}

	ignorePaths, err := ignorePaths(ctx, indexSummary.relativePaths, root, getChildren)
	if err != nil {
		return lsifstore.SCIPDataStream{}, err
	}

	metadata := lsifstore.ProcessedMetadata{
		TextDocumentEncoding: indexSummary.metadata.TextDocumentEncoding.String(),
		ToolName:             indexSummary.metadata.ToolInfo.Name,
		ToolVersion:          indexSummary.metadata.ToolInfo.Version,
		ToolArguments:        indexSummary.metadata.ToolInfo.Arguments,
		ProtocolVersion:      int(indexSummary.metadata.Version),
	}

	return lsifstore.SCIPDataStream{
		Metadata:         metadata,
		DocumentIterator: &documentOneShotIterator{ignorePaths, indexSummary, indexReader},
	}, nil
}

// Copied from io.ReadAll, but uses the given initial size for the buffer to
// attempt to reduce temporary slice allocations during large reads. If the
// given size is zero, then this function has the same behavior as io.ReadAll.
func readAllWithSizeHint(r io.Reader, n int64) ([]byte, error) {
	if n == 0 {
		return io.ReadAll(r)
	}

	buf := bytes.NewBuffer(make([]byte, 0, n))
	_, err := io.Copy(buf, r)
	return buf.Bytes(), err
}

// ignorePaths returns a set consisting of the relative paths of documents in the give
// slice that are not resolvable via Git.
func ignorePaths(ctx context.Context, documentRelativePaths []string, root string, getChildren pathexistence.GetChildrenFunc) (collections.Set[string], error) {
	checker, err := pathexistence.NewExistenceChecker(ctx, root, documentRelativePaths, getChildren)
	if err != nil {
		return nil, err
	}

	ignorePathSet := collections.NewSet[string]()
	for _, documentRelativePath := range documentRelativePaths {
		if !checker.Exists(documentRelativePath) {
			ignorePathSet.Add(documentRelativePath)
		}
	}

	return ignorePathSet, nil
}

// processDocument canonicalizes and serializes the given document for persistence.
func processDocument(document *scip.Document, externalSymbolsByName map[string]*scip.SymbolInformation) lsifstore.ProcessedSCIPDocument {
	// Stash path here as canonicalization removes it
	path := document.RelativePath
	canonicalizeDocument(document, externalSymbolsByName)

	return lsifstore.ProcessedSCIPDocument{
		Path:     path,
		Document: document,
	}
}

// canonicalizeDocument ensures that the fields of the given document are ordered in a
// deterministic manner (when it would not otherwise affect the data semantics). This pass
// has a two-fold benefit:
//
// (1) equivalent document payloads will share a canonical form, so they will hash to the
// same value when being inserted into the codeintel-db, and
// (2) consumers of canonical-form documents can rely on order of fields for quicker access,
// such as binary search through symbol names or occurrence ranges.
func canonicalizeDocument(document *scip.Document, externalSymbolsByName map[string]*scip.SymbolInformation) {
	// We store the relative path outside of the document payload so that renames do not
	// necessarily invalidate the document payload. When returning a SCIP document to the
	// consumer of a codeintel API, we reconstruct this relative path.
	document.RelativePath = ""

	// Denormalize external symbols into each referencing document
	injectExternalSymbols(document, externalSymbolsByName)

	// Order the remaining fields deterministically
	_ = scip.CanonicalizeDocument(document)
}

// injectExternalSymbols adds symbol information objects from the external symbols into the document
// if there is an occurrence that references the external symbol name and no local symbol information
// exists.
func injectExternalSymbols(document *scip.Document, externalSymbolsByName map[string]*scip.SymbolInformation) {
	// Build set of existing definitions
	definitionsSet := make(map[string]struct{}, len(document.Symbols))
	for _, symbol := range document.Symbols {
		definitionsSet[symbol.Symbol] = struct{}{}
	}

	// Build a set of occurrence and symbol relationship references
	referencesSet := make(map[string]struct{}, len(document.Symbols))
	for _, symbol := range document.Symbols {
		for _, relationship := range symbol.Relationships {
			referencesSet[relationship.Symbol] = struct{}{}
		}
	}
	for _, occurrence := range document.Occurrences {
		if occurrence.Symbol == "" || scip.IsLocalSymbol(occurrence.Symbol) {
			continue
		}

		referencesSet[occurrence.Symbol] = struct{}{}
	}

	// Add any references that do not have an associated definition
	for len(referencesSet) > 0 {
		// Collect unreferenced symbol names for new symbols. This can happen if we have
		// a set of external symbols that reference each other. The references set acts
		// as the frontier of our search.
		newReferencesSet := map[string]struct{}{}

		for symbolName := range referencesSet {
			if _, ok := definitionsSet[symbolName]; ok {
				continue
			}
			definitionsSet[symbolName] = struct{}{}

			symbol, ok := externalSymbolsByName[symbolName]
			if !ok {
				continue
			}

			// Add new definition for referenced symbol
			document.Symbols = append(document.Symbols, symbol)

			// Populate new frontier
			for _, relationship := range symbol.Relationships {
				newReferencesSet[relationship.Symbol] = struct{}{}
			}
		}

		// Continue resolving references while we added new symbols
		referencesSet = newReferencesSet
	}
}

// packageFromSymbol parses the given symbol name and returns its package scheme, name, and version.
// If the symbol name could not be parsed, a false-valued flag is returned.
func packageFromSymbol(symbolName string) (precise.Package, bool) {
	symbol, err := scip.ParseSymbol(symbolName)
	if err != nil {
		return precise.Package{}, false
	}
	if symbol.Package == nil {
		return precise.Package{}, false
	}
	if symbol.Package.Name == "" || symbol.Package.Version == "" {
		return precise.Package{}, false
	}

	pkg := precise.Package{
		Scheme:  symbol.Scheme,
		Manager: symbol.Package.Manager,
		Name:    symbol.Package.Name,
		Version: symbol.Package.Version,
	}
	return pkg, true
}

// writeSCIPDocuments iterates over the documents in the index and:
// - Assembles package information
// - Writes processed documents into the given store targeting codeintel-db
func writeSCIPDocuments(
	ctx context.Context,
	logger log.Logger,
	lsifStore lsifstore.Store,
	upload shared.Upload,
	scipDataStream lsifstore.SCIPDataStream,
	trace observation.TraceLogger,
) (pkgData lsifstore.ProcessedPackageData, err error) {
	return pkgData, lsifStore.WithTransaction(ctx, func(tx lsifstore.Store) error {
		if err := tx.InsertMetadata(ctx, upload.ID, scipDataStream.Metadata); err != nil {
			return err
		}

		var scipWriter lsifstore.SCIPWriter

		if upload.Indexer == shared.SyntacticIndexer {
			scipWriter, err = tx.NewSyntacticSCIPWriter(ctx, upload.ID)
		} else {
			scipWriter, err = tx.NewPreciseSCIPWriter(ctx, upload.ID)
		}
		if err != nil {
			return err
		}

		var numDocuments uint32
		processDoc := func(document lsifstore.ProcessedSCIPDocument) error {
			numDocuments += 1
			if err := scipWriter.InsertDocument(ctx, document.Path, document.Document); err != nil {
				return err
			}
			return nil
		}
		if err := scipDataStream.DocumentIterator.VisitAllDocuments(ctx, logger, &pkgData, processDoc); err != nil {
			return err
		}
		trace.AddEvent("TODO Domain Owner", attribute.Int64("numDocuments", int64(numDocuments)))

		count, err := scipWriter.Flush(ctx)
		if err != nil {
			return err
		}
		trace.AddEvent("TODO Domain Owner", attribute.Int64("numSymbols", int64(count)))

		pkgData.Normalize()
		return nil
	})
}
