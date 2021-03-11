package resolvers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/dbstore"
	store "github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/dbstore"
	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/lsifstore"
	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/semantic"
)

// adjustedUpload pairs an upload visible from the current target commit with the
// current target path and position adjusted so that it matches the data within the
// underlying index.
type adjustedUpload struct {
	Upload               store.Dump
	AdjustedPath         string
	AdjustedPosition     lsifstore.Position
	AdjustedPathInBundle string
}

// adjustUploads adjusts the current target path and the given position for each upload visible
// from the current target commit. If an upload cannot be adjusted, it will be omitted from the
// returned slice.
func (r *queryResolver) adjustUploads(ctx context.Context, line, character int) ([]adjustedUpload, error) {
	adjustedUploads := make([]adjustedUpload, 0, len(r.uploads))
	for i := range r.uploads {
		adjustedUpload, ok, err := r.adjustUpload(ctx, line, character, r.uploads[i])
		if err != nil {
			return nil, err
		}
		if ok {
			adjustedUploads = append(adjustedUploads, adjustedUpload)
		}
	}

	return adjustedUploads, nil
}

// adjustUpload adjusts the current target path and the given position for the given upload. If
// the upload cannot be adjusted, a false-valued flag is returned.
func (r *queryResolver) adjustUpload(ctx context.Context, line, character int, upload store.Dump) (adjustedUpload, bool, error) {
	position := lsifstore.Position{
		Line:      line,
		Character: character,
	}

	adjustedPath, adjustedPosition, ok, err := r.positionAdjuster.AdjustPosition(ctx, upload.Commit, r.path, position, false)
	if err != nil || !ok {
		return adjustedUpload{}, false, errors.Wrap(err, "positionAdjuster.AdjustPosition")
	}

	return adjustedUpload{
		Upload:               upload,
		AdjustedPath:         adjustedPath,
		AdjustedPosition:     adjustedPosition,
		AdjustedPathInBundle: strings.TrimPrefix(adjustedPath, upload.Root),
	}, true, nil
}

// definitionUploads returns the set of uploads that provide any of the given monikers. This method will
// not return uploads for commits which are unknown to gitserver.
func (r *queryResolver) definitionUploads(ctx context.Context, orderedMonikers []semantic.QualifiedMonikerData) ([]store.Dump, error) {
	uploads, err := r.dbStore.DefinitionDumps(ctx, orderedMonikers)
	if err != nil {
		return nil, errors.Wrap(err, "dbstore.DefinitionDumps")
	}

	return filterUploadsWithCommits(ctx, r.cachedCommitChecker, uploads)
}

// monikerLimit is the maximum number of monikers that can be returned from orderedMonikers.
const monikerLimit = 10

// orderedMonikers returns the set of monikers attached to the ranges specified by the given upload list.
// If kind is a non-empty string, monikers with a distinct kind are ignored.
//
// The return slice is ordered by visible upload, then by specificity, i.e., monikers attached to enclosed
// ranges before before monikers attached to enclosing ranges. Monikers are de-duplicated, such that the
// second (third, ...) occurrences are removed.
func (r *queryResolver) orderedMonikers(ctx context.Context, adjustedUploads []adjustedUpload, kind string) ([]semantic.QualifiedMonikerData, error) {
	monikerSet := newQualifiedMonikerSet()

	for i := range adjustedUploads {
		rangeMonikers, err := r.lsifStore.MonikersByPosition(
			ctx,
			adjustedUploads[i].Upload.ID,
			adjustedUploads[i].AdjustedPathInBundle,
			adjustedUploads[i].AdjustedPosition.Line,
			adjustedUploads[i].AdjustedPosition.Character,
		)
		if err != nil {
			return nil, errors.Wrap(err, "lsifStore.MonikersByPosition")
		}

		for _, monikers := range rangeMonikers {
			for _, moniker := range monikers {
				if moniker.PackageInformationID == "" || (kind != "" && moniker.Kind != kind) {
					continue
				}

				packageInformationData, _, err := r.lsifStore.PackageInformation(
					ctx,
					adjustedUploads[i].Upload.ID,
					adjustedUploads[i].AdjustedPathInBundle,
					string(moniker.PackageInformationID),
				)
				if err != nil {
					return nil, errors.Wrap(err, "lsifStore.PackageInformation")
				}

				monikerSet.add(semantic.QualifiedMonikerData{
					MonikerData:            moniker,
					PackageInformationData: packageInformationData,
				})

				if len(monikerSet.monikers) >= monikerLimit {
					return monikerSet.monikers, nil
				}
			}
		}
	}

	return monikerSet.monikers, nil
}

// monikerLocations returns the set of locations defined by any of the given uploads tagged with any of
// the given monikers.
func (r *queryResolver) monikerLocations(ctx context.Context, uploads []dbstore.Dump, orderedMonikers []semantic.QualifiedMonikerData, tableName string, limit, offset int) ([]lsifstore.Location, int, error) {
	ids := make([]int, 0, len(uploads))
	for i := range uploads {
		ids = append(ids, uploads[i].ID)
	}

	args := make([]semantic.MonikerData, 0, len(orderedMonikers))
	for _, moniker := range orderedMonikers {
		args = append(args, moniker.MonikerData)
	}

	locations, totalCount, err := r.lsifStore.BulkMonikerResults(ctx, tableName, ids, args, limit, offset)
	if err != nil {
		return nil, 0, errors.Wrap(err, "lsifStore.BulkMonikerResults")
	}

	return locations, totalCount, nil
}

// adjustLocations translates a set of locations into an equivalent set of locations in the requested
// commit.
func (r *queryResolver) adjustLocations(ctx context.Context, uploadsByID map[int]dbstore.Dump, locations []lsifstore.Location) ([]AdjustedLocation, error) {
	adjustedLocations := make([]AdjustedLocation, 0, len(locations))
	for _, location := range locations {
		adjustedLocation, err := r.adjustLocation(ctx, uploadsByID[location.DumpID], location)
		if err != nil {
			return nil, err
		}

		adjustedLocations = append(adjustedLocations, adjustedLocation)
	}

	return adjustedLocations, nil
}

// adjustLocation translates a location (relative to the indexed commit) into an equivalent location in
// the requested commit.
func (r *queryResolver) adjustLocation(ctx context.Context, dump store.Dump, location lsifstore.Location) (AdjustedLocation, error) {
	adjustedCommit, adjustedRange, err := r.adjustRange(ctx, dump.RepositoryID, dump.Commit, dump.Root+location.Path, location.Range)
	if err != nil {
		return AdjustedLocation{}, err
	}

	return AdjustedLocation{
		Dump:           dump,
		Path:           dump.Root + location.Path,
		AdjustedCommit: adjustedCommit,
		AdjustedRange:  adjustedRange,
	}, nil
}

// adjustRange translates a range (relative to the indexed commit) into an equivalent range in the requested
// commit.
func (r *queryResolver) adjustRange(ctx context.Context, repositoryID int, commit, path string, rn lsifstore.Range) (string, lsifstore.Range, error) {
	if repositoryID != r.repositoryID {
		// No diffs between distinct repositories
		return commit, rn, nil
	}

	if _, adjustedRange, ok, err := r.positionAdjuster.AdjustRange(ctx, commit, path, rn, true); err != nil {
		return "", lsifstore.Range{}, errors.Wrap(err, "positionAdjuster.AdjustRange")
	} else if ok {
		return r.commit, adjustedRange, nil
	}

	return commit, rn, nil
}

// filterUploadsWithCommits removes the uploads for commits which are unknown to gitserver from the given
// lice. The slice is filtered in-place and returned (to update the slice length).
func filterUploadsWithCommits(ctx context.Context, cachedCommitChecker *cachedCommitChecker, uploads []dbstore.Dump) ([]dbstore.Dump, error) {
	filtered := uploads[:0]

	for i := range uploads {
		commitExists, err := cachedCommitChecker.exists(ctx, uploads[i].RepositoryID, uploads[i].Commit)
		if err != nil {
			return nil, err
		}
		if !commitExists {
			continue
		}

		filtered = append(filtered, uploads[i])
	}

	return filtered, nil
}

func intsToString(ints []int) string {
	segments := make([]string, 0, len(ints))
	for _, id := range ints {
		segments = append(segments, strconv.Itoa(id))
	}

	return strings.Join(segments, ", ")
}

func uploadIDsToString(vs []dbstore.Dump) string {
	ids := make([]string, 0, len(vs))
	for _, v := range vs {
		ids = append(ids, strconv.Itoa(v.ID))
	}

	return strings.Join(ids, ", ")
}

func monikersToString(vs []semantic.QualifiedMonikerData) string {
	strs := make([]string, 0, len(vs))
	for _, v := range vs {
		strs = append(strs, fmt.Sprintf("%s:%s:%s", v.Scheme, v.Identifier, v.Version))
	}

	return strings.Join(strs, ", ")
}
