package conversion

import (
	"errors"

	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/semantic"
)

type QueryResult struct {
	Definitions []semantic.LocationData
	References  []semantic.LocationData
	Hover       string
	Monikers    []semantic.MonikerData
}

func Query(bundle *GroupedBundleDataMaps, path string, line, character int) ([]QueryResult, error) {
	document, exists := bundle.Documents[path]
	if !exists {
		return nil, errors.New("path does not exist in bundle")
	}

	var result []QueryResult
	for _, rng := range semantic.FindRanges(document.Ranges, line, character) {
		result = append(result, Resolve(bundle, document, rng))
	}

	return result, nil
}

func Resolve(bundle *GroupedBundleDataMaps, document semantic.DocumentData, rng semantic.RangeData) QueryResult {
	hover := document.HoverResults[rng.HoverResultID]
	var monikers []semantic.MonikerData
	for _, monikerID := range rng.MonikerIDs {
		monikers = append(monikers, document.Monikers[monikerID])
	}

	return QueryResult{
		Definitions: resolveLocations(bundle, rng.DefinitionResultID),
		References:  resolveLocations(bundle, rng.ReferenceResultID),
		Hover:       hover,
		Monikers:    monikers,
	}
}

func resolveLocations(bundle *GroupedBundleDataMaps, resultID semantic.ID) []semantic.LocationData {
	var locations []semantic.LocationData
	docIDRngIDs, chunk := getDefRef(resultID, bundle.Meta, bundle.ResultChunks)
	for _, docIDRngID := range docIDRngIDs {
		path := chunk.DocumentPaths[docIDRngID.DocumentID]
		rng := bundle.Documents[path].Ranges[docIDRngID.RangeID]
		locations = append(locations, semantic.LocationData{
			URI:            path,
			StartLine:      rng.StartLine,
			StartCharacter: rng.StartCharacter,
			EndLine:        rng.EndLine,
			EndCharacter:   rng.EndCharacter,
		})
	}
	return locations
}
