package conversion

import (
	"context"

	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/datastructures"
	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/pathexistence"
)

// prune removes references to documents in the given correlation state that do not exist in
// the git clone at the target commit. This is a necessary step as documents not in git will
// not be the source of any queries (and take up unnecessary space in the converted index),
// and may be the target of a definition or reference (and references a file we do not have).
func prune(ctx context.Context, state *State, root string, getChildren pathexistence.GetChildrenFunc) error {
	paths := make([]string, 0, len(state.DocumentData))
	for _, uri := range state.DocumentData {
		paths = append(paths, uri)
	}

	checker, err := pathexistence.NewExistenceChecker(ctx, root, paths, getChildren)
	if err != nil {
		return err
	}

	for documentID, uri := range state.DocumentData {
		if !checker.Exists(uri) {
			// Document does not exist in git
			delete(state.DocumentData, documentID)
		}
	}

	pruneFromDefinitionReferences(state, state.DefinitionData)
	pruneFromDefinitionReferences(state, state.ReferenceData)
	return nil
}

func pruneFromDefinitionReferences(state *State, definitionReferenceData map[int]*datastructures.DefaultIDSetMap) {
	for _, documentRanges := range definitionReferenceData {
		documentRanges.Each(func(documentID int, rangeIDs *datastructures.IDSet) {
			if _, ok := state.DocumentData[documentID]; !ok {
				// Document was pruned, remove reference
				documentRanges.Delete(documentID)
			}
		})
	}
}
