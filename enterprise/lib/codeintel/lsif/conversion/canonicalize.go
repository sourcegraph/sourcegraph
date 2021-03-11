package conversion

import (
	"sort"

	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/datastructures"
)

// canonicalize deduplicates data in the raw correlation state and collapses range,
// result set, and moniker data that form chains via next edges.
func canonicalize(state *State) {
	fns := []func(state *State){
		canonicalizeDocuments,
		canonicalizeReferenceResults,
		canonicalizeResultSets,
		canonicalizeRanges,
	}

	for _, fn := range fns {
		fn(state)
	}
}

// canonicalizeDocuments determines if multiple documents are defined with the same URI. This can
// happen in some indexers (such as lsif-tsc) that index dependent projects into the same index
// as the target project. For each set of documents that share a path, we choose one document to
// be the canonical representative and merge the contains, definition, and reference data into the
// unique canonical document. This function guarantees that duplicate document IDs are removed from
// the correlation state.
func canonicalizeDocuments(state *State) {
	documentIDs := map[string][]int{}
	for documentID, uri := range state.DocumentData {
		documentIDs[uri] = append(documentIDs[uri], documentID)
	}
	for _, v := range documentIDs {
		sort.Ints(v)
	}

	for documentID, uri := range state.DocumentData {
		// Choose canonical document alphabetically
		if canonicalID := documentIDs[uri][0]; documentID != canonicalID {
			// Move ranges and diagnostics into the canonical document
			state.Contains.SetUnion(canonicalID, state.Contains.Get(documentID))
			state.Diagnostics.SetUnion(canonicalID, state.Diagnostics.Get(documentID))

			canonicalizeDocumentsInDefinitionReferences(state, state.DefinitionData, documentID, canonicalID)
			canonicalizeDocumentsInDefinitionReferences(state, state.ReferenceData, documentID, canonicalID)

			// Remove non-canonical document
			delete(state.DocumentData, documentID)
			state.Contains.Delete(documentID)
			state.Diagnostics.Delete(documentID)
		}
	}
}

// canonicalizeDocumentsInDefinitionReferences moves definition or reference result data from the
// given document to the given canonical document and removes all references to the non-canonical
// document.
func canonicalizeDocumentsInDefinitionReferences(state *State, definitionReferenceData map[int]*datastructures.DefaultIDSetMap, documentID, canonicalID int) {
	for _, documentRanges := range definitionReferenceData {
		if rangeIDs := documentRanges.Get(documentID); rangeIDs != nil {
			// Move definition/reference data into the canonical document
			documentRanges.SetUnion(canonicalID, rangeIDs)

			// Remove references to non-canonical document
			documentRanges.Delete(documentID)
		}
	}
}

// canonicalizeReferenceResults determines which reference results are linked together. For each
// set of linked reference results, we choose one reference result to be the canonical representative
// and merge the data into the unique canonical result set. All non-canonical results are removed from
// the correlation state and references to non-canonical results are updated to refer to the canonical
// choice.
func canonicalizeReferenceResults(state *State) {
	// Maintain a map from a reference result to its canonical identifier
	canonicalIDs := map[int]int{}

	state.LinkedReferenceResults.Each(func(referenceResultID int, v *datastructures.IDSet) {
		if _, ok := canonicalIDs[referenceResultID]; ok {
			// Already processed
			return
		}

		// Find all reachable items in this set
		linkedIDs := state.LinkedReferenceResults.ExtractSet(referenceResultID)
		canonicalID, _ := linkedIDs.Min()
		canonicalReferenceResult := state.ReferenceData[canonicalID]

		linkedIDs.Each(func(linkedID int) {
			// Mark canonical choice
			canonicalIDs[linkedID] = canonicalID

			if linkedID != canonicalID {
				state.ReferenceData[linkedID].Each(func(documentID int, rangeIDs *datastructures.IDSet) {
					// Move range data into the canonical document
					canonicalReferenceResult.SetUnion(documentID, rangeIDs)
				})
			}
		})
	})

	for id, item := range state.RangeData {
		if canonicalID, ok := canonicalIDs[item.ReferenceResultID]; ok {
			// Update reference result identifier to canonical choice
			state.RangeData[id] = item.SetReferenceResultID(canonicalID)
		}
	}

	for id, item := range state.ResultSetData {
		if canonicalID, ok := canonicalIDs[item.ReferenceResultID]; ok {
			// Update reference result identifier to canonical choice
			state.ResultSetData[id] = item.SetReferenceResultID(canonicalID)
		}
	}

	// Invert the map to get a set of canonical identifiers
	inverseMap := datastructures.NewIDSet()
	for _, canonicalID := range canonicalIDs {
		inverseMap.Add(canonicalID)
	}

	for referenceResultID := range canonicalIDs {
		if !inverseMap.Contains(referenceResultID) {
			// Remove non-canonical reference result
			delete(state.ReferenceData, referenceResultID)
		}
	}
}

// canonicalizeResultSets runs canonicalizeResultSet on each result set in the correlation state.
// This will collapse result sets down recursively so that if a result set's next element also has
// a next element, then both sets merge down into the original result set.
func canonicalizeResultSets(state *State) {
	for resultSetID, resultSetData := range state.ResultSetData {
		canonicalizeResultSetData(state, resultSetID, resultSetData)
	}

	for resultSetID := range state.ResultSetData {
		state.Monikers.SetUnion(resultSetID, gatherMonikers(state, state.Monikers.Get(resultSetID)))
	}
}

// canonicalizeResultSets "merges down" the definition, reference, and hover result identifiers
// from the element's "next" result set if such an element exists and the identifier is not already.
// defined. This also merges down the moniker ids by unioning the sets.
//
// This method is assumed to be invoked only after canonicalizeResultSets, otherwise the next element
// of a range may not have all of the necessary data to perform this canonicalization step.
func canonicalizeRanges(state *State) {
	for rangeID, rangeData := range state.RangeData {
		if nextID, nextItem, ok := next(state, rangeID); ok {
			// Merge range and next element
			rangeData = mergeNextRangeData(state, rangeID, rangeData, nextID, nextItem)
			// Delete next data to prevent us from re-performing this step
			delete(state.NextData, rangeID)
		}

		state.RangeData[rangeID] = rangeData
		state.Monikers.SetUnion(rangeID, gatherMonikers(state, state.Monikers.Get(rangeID)))
	}
}

// canonicalizeResultSets "merges down" the definition, reference, and hover result identifiers
// from the element's "next" result set if such an element exists and the identifier is not
// already defined. This also merges down the moniker ids by unioning the sets.
func canonicalizeResultSetData(state *State, id int, item ResultSet) ResultSet {
	if nextID, nextItem, ok := next(state, id); ok {
		// Recursively canonicalize the next element
		nextItem = canonicalizeResultSetData(state, nextID, nextItem)
		// Merge result set and canonicalized next element
		item = mergeNextResultSetData(state, id, item, nextID, nextItem)
		// Delete next data to prevent us from re-performing this step
		delete(state.NextData, id)
	}

	state.ResultSetData[id] = item
	return item
}

// mergeNextResultSetData merges the definition, reference, and hover result identifiers from
// nextItem into item when not already defined. The moniker identifiers of nextItem are unioned
// into the moniker identifiers of item.
func mergeNextResultSetData(state *State, itemID int, item ResultSet, nextID int, nextItem ResultSet) ResultSet {
	if item.DefinitionResultID == 0 {
		item = item.SetDefinitionResultID(nextItem.DefinitionResultID)
	}
	if item.ReferenceResultID == 0 {
		item = item.SetReferenceResultID(nextItem.ReferenceResultID)
	}
	if item.HoverResultID == 0 {
		item = item.SetHoverResultID(nextItem.HoverResultID)
	}

	state.Monikers.SetUnion(itemID, state.Monikers.Get(nextID))
	return item
}

// mergeNextRangeData merges the definition, reference, and hover result identifiers from nextItem
// into item when not already defined. The moniker identifiers of nextItem are unioned into the
// moniker identifiers of item.
func mergeNextRangeData(state *State, itemID int, item Range, nextID int, nextItem ResultSet) Range {
	if item.DefinitionResultID == 0 {
		item = item.SetDefinitionResultID(nextItem.DefinitionResultID)
	}
	if item.ReferenceResultID == 0 {
		item = item.SetReferenceResultID(nextItem.ReferenceResultID)
	}
	if item.HoverResultID == 0 {
		item = item.SetHoverResultID(nextItem.HoverResultID)
	}

	state.Monikers.SetUnion(itemID, state.Monikers.Get(nextID))
	return item
}

// gatherMonikers returns a new set of moniker identifiers based off the given set. The returned
// set will additionall contain the transitive closure of all moniker identifiers linked to any
// moniker identifier in the original set. This ignores adding any local-kind monikers to the new
// set.
func gatherMonikers(state *State, source *datastructures.IDSet) *datastructures.IDSet {
	if source == nil || source.Len() == 0 {
		return nil
	}

	monikers := datastructures.NewIDSet()

	source.Each(func(sourceID int) {
		state.LinkedMonikers.ExtractSet(sourceID).Each(func(id int) {
			if state.MonikerData[id].Kind != "local" {
				monikers.Add(id)
			}
		})
	})

	return monikers
}

// next returns the "next" identifier and result set element for the given identifier, if one exists.
func next(state *State, id int) (int, ResultSet, bool) {
	nextID, ok := state.NextData[id]
	if !ok {
		return 0, ResultSet{}, false
	}

	return nextID, state.ResultSetData[nextID], true
}
