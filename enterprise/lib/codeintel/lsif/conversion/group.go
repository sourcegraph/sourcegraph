package conversion

import (
	"context"
	"math"
	"sort"
	"strings"

	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/bloomfilter"
	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/datastructures"
	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/semantic"
)

// GroupedBundleData{Chans,Maps} is a view of a correlation State that sorts data by it's containing document
// and shared data into sharded result chunks. The fields of this type are what is written to
// persistent storage and what is read in the query path. The Chans version allows pipelining
// and parallelizing the work, while the Maps version can be modified for e.g. local development
// via the REPL or patching for incremental indexing.
type GroupedBundleDataChans struct {
	Meta              semantic.MetaData
	Documents         chan semantic.KeyedDocumentData
	ResultChunks      chan semantic.IndexedResultChunkData
	Definitions       chan semantic.MonikerLocations
	References        chan semantic.MonikerLocations
	Packages          []semantic.Package
	PackageReferences []semantic.PackageReference
}

type GroupedBundleDataMaps struct {
	Meta              semantic.MetaData
	Documents         map[string]semantic.DocumentData
	ResultChunks      map[int]semantic.ResultChunkData
	Definitions       map[string]map[string][]semantic.LocationData
	References        map[string]map[string][]semantic.LocationData
	Packages          []semantic.Package
	PackageReferences []semantic.PackageReference
}

const MaxNumResultChunks = 1000
const ResultsPerResultChunk = 500

func getDefinitionResultID(r Range) int { return r.DefinitionResultID }
func getReferenceResultID(r Range) int  { return r.ReferenceResultID }

// groupBundleData converts a raw (but canonicalized) correlation State into a GroupedBundleData.
func groupBundleData(ctx context.Context, state *State, dumpID int) (*GroupedBundleDataChans, error) {
	numResults := len(state.DefinitionData) + len(state.ReferenceData)
	numResultChunks := int(math.Min(
		MaxNumResultChunks,
		math.Max(
			1,
			math.Floor(float64(numResults)/ResultsPerResultChunk),
		),
	))

	meta := semantic.MetaData{NumResultChunks: numResultChunks}
	documents := serializeBundleDocuments(ctx, state)
	resultChunks := serializeResultChunks(ctx, state, numResultChunks)
	definitionRows := gatherMonikersLocations(ctx, state, state.DefinitionData, getDefinitionResultID)
	referenceRows := gatherMonikersLocations(ctx, state, state.ReferenceData, getReferenceResultID)
	packages := gatherPackages(state, dumpID)
	packageReferences, err := gatherPackageReferences(state, dumpID)
	if err != nil {
		return nil, err
	}

	return &GroupedBundleDataChans{
		Meta:              meta,
		Documents:         documents,
		ResultChunks:      resultChunks,
		Definitions:       definitionRows,
		References:        referenceRows,
		Packages:          packages,
		PackageReferences: packageReferences,
	}, nil
}

func serializeBundleDocuments(ctx context.Context, state *State) chan semantic.KeyedDocumentData {
	ch := make(chan semantic.KeyedDocumentData)

	go func() {
		defer close(ch)

		for documentID, uri := range state.DocumentData {
			if strings.HasPrefix(uri, "..") {
				continue
			}

			data := semantic.KeyedDocumentData{
				Path:     uri,
				Document: serializeDocument(state, documentID),
			}

			select {
			case ch <- data:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch
}

func serializeDocument(state *State, documentID int) semantic.DocumentData {
	document := semantic.DocumentData{
		Ranges:             make(map[semantic.ID]semantic.RangeData, state.Contains.SetLen(documentID)),
		HoverResults:       map[semantic.ID]string{},
		Monikers:           map[semantic.ID]semantic.MonikerData{},
		PackageInformation: map[semantic.ID]semantic.PackageInformationData{},
		Diagnostics:        make([]semantic.DiagnosticData, 0, state.Diagnostics.SetLen(documentID)),
	}

	state.Contains.SetEach(documentID, func(rangeID int) {
		rangeData := state.RangeData[rangeID]

		monikerIDs := make([]semantic.ID, 0, state.Monikers.SetLen(rangeID))
		state.Monikers.SetEach(rangeID, func(monikerID int) {
			moniker := state.MonikerData[monikerID]
			monikerIDs = append(monikerIDs, toID(monikerID))

			document.Monikers[toID(monikerID)] = semantic.MonikerData{
				Kind:                 moniker.Kind,
				Scheme:               moniker.Scheme,
				Identifier:           moniker.Identifier,
				PackageInformationID: toID(moniker.PackageInformationID),
			}

			if moniker.PackageInformationID != 0 {
				packageInformation := state.PackageInformationData[moniker.PackageInformationID]
				document.PackageInformation[toID(moniker.PackageInformationID)] = semantic.PackageInformationData{
					Name:    packageInformation.Name,
					Version: packageInformation.Version,
				}
			}
		})

		document.Ranges[toID(rangeID)] = semantic.RangeData{
			StartLine:          rangeData.Start.Line,
			StartCharacter:     rangeData.Start.Character,
			EndLine:            rangeData.End.Line,
			EndCharacter:       rangeData.End.Character,
			DefinitionResultID: toID(rangeData.DefinitionResultID),
			ReferenceResultID:  toID(rangeData.ReferenceResultID),
			HoverResultID:      toID(rangeData.HoverResultID),
			MonikerIDs:         monikerIDs,
		}

		if rangeData.HoverResultID != 0 {
			hoverData := state.HoverData[rangeData.HoverResultID]
			document.HoverResults[toID(rangeData.HoverResultID)] = hoverData
		}
	})

	state.Diagnostics.SetEach(documentID, func(diagnosticID int) {
		for _, diagnostic := range state.DiagnosticResults[diagnosticID] {
			document.Diagnostics = append(document.Diagnostics, semantic.DiagnosticData{
				Severity:       diagnostic.Severity,
				Code:           diagnostic.Code,
				Message:        diagnostic.Message,
				Source:         diagnostic.Source,
				StartLine:      diagnostic.StartLine,
				StartCharacter: diagnostic.StartCharacter,
				EndLine:        diagnostic.EndLine,
				EndCharacter:   diagnostic.EndCharacter,
			})
		}
	})

	return document
}

func serializeResultChunks(ctx context.Context, state *State, numResultChunks int) chan semantic.IndexedResultChunkData {
	chunkAssignments := make(map[int][]int, numResultChunks)
	for id := range state.DefinitionData {
		index := semantic.HashKey(toID(id), numResultChunks)
		chunkAssignments[index] = append(chunkAssignments[index], id)
	}
	for id := range state.ReferenceData {
		index := semantic.HashKey(toID(id), numResultChunks)
		chunkAssignments[index] = append(chunkAssignments[index], id)
	}

	ch := make(chan semantic.IndexedResultChunkData)

	go func() {
		defer close(ch)

		for index, resultIDs := range chunkAssignments {
			if len(resultIDs) == 0 {
				continue
			}

			documentPaths := map[semantic.ID]string{}
			rangeIDsByResultID := make(map[semantic.ID][]semantic.DocumentIDRangeID, len(resultIDs))

			for _, resultID := range resultIDs {
				documentRanges, ok := state.DefinitionData[resultID]
				if !ok {
					documentRanges = state.ReferenceData[resultID]
				}

				rangeIDMap := map[semantic.ID]int{}
				var documentIDRangeIDs []semantic.DocumentIDRangeID

				documentRanges.Each(func(documentID int, rangeIDs *datastructures.IDSet) {
					docID := toID(documentID)
					documentPaths[docID] = state.DocumentData[documentID]

					rangeIDs.Each(func(rangeID int) {
						rangeIDMap[toID(rangeID)] = rangeID

						documentIDRangeIDs = append(documentIDRangeIDs, semantic.DocumentIDRangeID{
							DocumentID: docID,
							RangeID:    toID(rangeID),
						})
					})
				})

				// Sort locations by containing document path then by offset within the text
				// document (in reading order). This provides us with an obvious and deterministic
				// ordering of a result set over multiple API requests.

				sort.Sort(sortableDocumentIDRangeIDs{
					state:         state,
					documentPaths: documentPaths,
					rangeIDMap:    rangeIDMap,
					s:             documentIDRangeIDs,
				})

				rangeIDsByResultID[toID(resultID)] = documentIDRangeIDs
			}

			data := semantic.IndexedResultChunkData{
				Index: index,
				ResultChunk: semantic.ResultChunkData{
					DocumentPaths:      documentPaths,
					DocumentIDRangeIDs: rangeIDsByResultID,
				},
			}

			select {
			case ch <- data:
			case <-ctx.Done():
				return
			}
		}
	}()

	return ch
}

// sortableDocumentIDRangeIDs implements sort.Interface for document/range id pairs.
type sortableDocumentIDRangeIDs struct {
	state         *State
	documentPaths map[semantic.ID]string
	rangeIDMap    map[semantic.ID]int
	s             []semantic.DocumentIDRangeID
}

func (s sortableDocumentIDRangeIDs) Len() int      { return len(s.s) }
func (s sortableDocumentIDRangeIDs) Swap(i, j int) { s.s[i], s.s[j] = s.s[j], s.s[i] }
func (s sortableDocumentIDRangeIDs) Less(i, j int) bool {
	iDocumentID := s.s[i].DocumentID
	jDocumentID := s.s[j].DocumentID
	iRange := s.state.RangeData[s.rangeIDMap[s.s[i].RangeID]]
	jRange := s.state.RangeData[s.rangeIDMap[s.s[j].RangeID]]

	if s.documentPaths[iDocumentID] != s.documentPaths[jDocumentID] {
		return s.documentPaths[iDocumentID] <= s.documentPaths[jDocumentID]
	}

	if cmp := iRange.Start.Line - jRange.Start.Line; cmp != 0 {
		return cmp < 0

	}

	return iRange.Start.Character-jRange.Start.Character < 0
}

func gatherMonikersLocations(ctx context.Context, state *State, data map[int]*datastructures.DefaultIDSetMap, getResultID func(r Range) int) chan semantic.MonikerLocations {
	monikers := datastructures.NewDefaultIDSetMap()
	for rangeID, r := range state.RangeData {
		if resultID := getResultID(r); resultID != 0 {
			monikers.SetUnion(resultID, state.Monikers.Get(rangeID))
		}
	}

	idsBySchemeByIdentifier := map[string]map[string][]int{}
	for id := range data {
		monikerIDs := monikers.Get(id)
		if monikerIDs == nil {
			continue
		}

		monikerIDs.Each(func(monikerID int) {
			moniker := state.MonikerData[monikerID]
			idsByIdentifier, ok := idsBySchemeByIdentifier[moniker.Scheme]
			if !ok {
				idsByIdentifier = map[string][]int{}
				idsBySchemeByIdentifier[moniker.Scheme] = idsByIdentifier
			}
			idsByIdentifier[moniker.Identifier] = append(idsByIdentifier[moniker.Identifier], id)
		})
	}

	ch := make(chan semantic.MonikerLocations)

	go func() {
		defer close(ch)

		for scheme, idsByIdentifier := range idsBySchemeByIdentifier {
			for identifier, ids := range idsByIdentifier {
				var locations []semantic.LocationData
				for _, id := range ids {
					data[id].Each(func(documentID int, rangeIDs *datastructures.IDSet) {
						uri := state.DocumentData[documentID]
						if strings.HasPrefix(uri, "..") {
							return
						}

						rangeIDs.Each(func(id int) {
							r := state.RangeData[id]

							locations = append(locations, semantic.LocationData{
								URI:            uri,
								StartLine:      r.Start.Line,
								StartCharacter: r.Start.Character,
								EndLine:        r.End.Line,
								EndCharacter:   r.End.Character,
							})
						})
					})
				}

				if len(locations) == 0 {
					continue
				}

				// Sort locations by containing document path then by offset within the text
				// document (in reading order). This provides us with an obvious and deterministic
				// ordering of a result set over multiple API requests.

				sort.Sort(sortableLocations(locations))

				data := semantic.MonikerLocations{
					Scheme:     scheme,
					Identifier: identifier,
					Locations:  locations,
				}

				select {
				case ch <- data:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return ch
}

// sortableLocations implements sort.Interface for locations.
type sortableLocations []semantic.LocationData

func (s sortableLocations) Len() int      { return len(s) }
func (s sortableLocations) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s sortableLocations) Less(i, j int) bool {
	if s[i].URI != s[j].URI {
		return s[i].URI <= s[j].URI
	}

	if cmp := s[i].StartLine - s[j].StartLine; cmp != 0 {
		return cmp < 0
	}

	return s[i].StartCharacter < s[j].StartCharacter
}

func gatherPackages(state *State, dumpID int) []semantic.Package {
	uniques := make(map[string]semantic.Package, state.ExportedMonikers.Len())
	state.ExportedMonikers.Each(func(id int) {
		source := state.MonikerData[id]
		packageInfo := state.PackageInformationData[source.PackageInformationID]

		uniques[makeKey(source.Scheme, packageInfo.Name, packageInfo.Version)] = semantic.Package{
			Scheme:  source.Scheme,
			Name:    packageInfo.Name,
			Version: packageInfo.Version,
		}
	})

	packages := make([]semantic.Package, 0, len(uniques))
	for _, v := range uniques {
		packages = append(packages, v)
	}

	return packages
}

func gatherPackageReferences(state *State, dumpID int) ([]semantic.PackageReference, error) {
	type ExpandedPackageReference struct {
		Scheme      string
		Name        string
		Version     string
		Identifiers []string
	}

	uniques := make(map[string]ExpandedPackageReference, state.ImportedMonikers.Len())
	state.ImportedMonikers.Each(func(id int) {
		source := state.MonikerData[id]
		packageInfo := state.PackageInformationData[source.PackageInformationID]

		key := makeKey(source.Scheme, packageInfo.Name, packageInfo.Version)
		uniques[key] = ExpandedPackageReference{
			Scheme:      source.Scheme,
			Name:        packageInfo.Name,
			Version:     packageInfo.Version,
			Identifiers: append(uniques[key].Identifiers, source.Identifier),
		}
	})

	packageReferences := make([]semantic.PackageReference, 0, len(uniques))
	for _, v := range uniques {
		filter, err := bloomfilter.CreateFilter(v.Identifiers)
		if err != nil {
			return nil, errors.Wrap(err, "bloomfilter.CreateFilter")
		}

		packageReferences = append(packageReferences, semantic.PackageReference{
			Scheme:  v.Scheme,
			Name:    v.Name,
			Version: v.Version,
			Filter:  filter,
		})
	}

	return packageReferences, nil
}

// CAUTION: Data is not deep copied.
func GroupedBundleDataMapsToChans(ctx context.Context, maps *GroupedBundleDataMaps) *GroupedBundleDataChans {
	documentChan := make(chan semantic.KeyedDocumentData, len(maps.Documents))
	go func() {
		defer close(documentChan)
		for path, doc := range maps.Documents {
			select {
			case documentChan <- semantic.KeyedDocumentData{
				Path:     path,
				Document: doc,
			}:
			case <-ctx.Done():
				return
			}
		}
	}()
	resultChunkChan := make(chan semantic.IndexedResultChunkData, len(maps.ResultChunks))
	go func() {
		defer close(resultChunkChan)

		for idx, chunk := range maps.ResultChunks {
			select {
			case resultChunkChan <- semantic.IndexedResultChunkData{
				Index:       idx,
				ResultChunk: chunk,
			}:
			case <-ctx.Done():
				return
			}
		}
	}()
	monikerDefsChan := make(chan semantic.MonikerLocations)
	go func() {
		defer close(monikerDefsChan)

		for scheme, identMap := range maps.Definitions {
			for ident, locations := range identMap {
				select {
				case monikerDefsChan <- semantic.MonikerLocations{
					Scheme:     scheme,
					Identifier: ident,
					Locations:  locations,
				}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()
	monikerRefsChan := make(chan semantic.MonikerLocations)
	go func() {
		defer close(monikerRefsChan)

		for scheme, identMap := range maps.References {
			for ident, locations := range identMap {
				select {
				case monikerRefsChan <- semantic.MonikerLocations{
					Scheme:     scheme,
					Identifier: ident,
					Locations:  locations,
				}:
				case <-ctx.Done():
					return
				}
			}
		}
	}()

	return &GroupedBundleDataChans{
		Meta:              maps.Meta,
		Documents:         documentChan,
		ResultChunks:      resultChunkChan,
		Definitions:       monikerDefsChan,
		References:        monikerRefsChan,
		Packages:          maps.Packages,
		PackageReferences: maps.PackageReferences,
	}
}

// CAUTION: Data is not deep copied.
func GroupedBundleDataChansToMaps(ctx context.Context, chans *GroupedBundleDataChans) *GroupedBundleDataMaps {
	documentMap := make(map[string]semantic.DocumentData)
	for keyedDocumentData := range chans.Documents {
		documentMap[keyedDocumentData.Path] = keyedDocumentData.Document
	}
	resultChunkMap := make(map[int]semantic.ResultChunkData)
	for indexedResultChunk := range chans.ResultChunks {
		resultChunkMap[indexedResultChunk.Index] = indexedResultChunk.ResultChunk
	}
	monikerDefsMap := make(map[string]map[string][]semantic.LocationData)
	for monikerDefs := range chans.Definitions {
		identMap, exists := monikerDefsMap[monikerDefs.Scheme]
		if !exists {
			identMap = make(map[string][]semantic.LocationData)
		}
		identMap[monikerDefs.Identifier] = monikerDefs.Locations
	}
	monikerRefsMap := make(map[string]map[string][]semantic.LocationData)
	for monikerRefs := range chans.References {
		identMap, exists := monikerRefsMap[monikerRefs.Scheme]
		if !exists {
			identMap = make(map[string][]semantic.LocationData)
		}
		identMap[monikerRefs.Identifier] = monikerRefs.Locations
	}

	return &GroupedBundleDataMaps{
		Meta:              chans.Meta,
		Documents:         documentMap,
		ResultChunks:      resultChunkMap,
		Definitions:       monikerDefsMap,
		References:        monikerRefsMap,
		Packages:          chans.Packages,
		PackageReferences: chans.PackageReferences,
	}
}
