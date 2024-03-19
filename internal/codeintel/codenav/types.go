package codenav

import (
	"strings"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/shared"
	uploadsshared "github.com/sourcegraph/sourcegraph/internal/codeintel/uploads/shared"
	"github.com/sourcegraph/sourcegraph/lib/codeintel/precise"
)

// visibleUpload pairs an upload visible from the current target commit with the
// current target path and position matched to the data within the underlying index.
type visibleUpload struct {
	Upload                uploadsshared.CompletedUpload
	TargetPath            string
	TargetPosition        shared.Position
	TargetPathWithoutRoot string
}

type qualifiedMonikerSet struct {
	monikers       []precise.QualifiedMonikerData
	monikerHashMap map[string]struct{}
}

func newQualifiedMonikerSet() *qualifiedMonikerSet {
	return &qualifiedMonikerSet{
		monikerHashMap: map[string]struct{}{},
	}
}

// add the given qualified moniker to the set if it is distinct from all elements
// currently in the set.
func (s *qualifiedMonikerSet) add(qualifiedMoniker precise.QualifiedMonikerData) {
	monikerHash := strings.Join([]string{
		qualifiedMoniker.PackageInformationData.Name,
		qualifiedMoniker.PackageInformationData.Version,
		qualifiedMoniker.MonikerData.Scheme,
		qualifiedMoniker.PackageInformationData.Manager,
		qualifiedMoniker.MonikerData.Identifier,
	}, ":")

	if _, ok := s.monikerHashMap[monikerHash]; ok {
		return
	}

	s.monikerHashMap[monikerHash] = struct{}{}
	s.monikers = append(s.monikers, qualifiedMoniker)
}

type RequestArgs struct {
	RepositoryID int
	Commit       string
	Limit        int
	RawCursor    string
}

type PositionalRequestArgs struct {
	RequestArgs
	Path      string
	Line      int
	Character int
}

// DiagnosticAtUpload is a diagnostic from within a particular upload. The adjusted commit denotes
// the target commit for which the location was adjusted (the originally requested commit).
type DiagnosticAtUpload struct {
	shared.Diagnostic
	Upload         uploadsshared.CompletedUpload
	AdjustedCommit string
	AdjustedRange  shared.Range
}

// AdjustedCodeIntelligenceRange stores definition, reference, and hover information for all ranges
// within a block of lines. The definition and reference locations have been adjusted to fit the
// target (originally requested) commit.
type AdjustedCodeIntelligenceRange struct {
	Range           shared.Range
	Definitions     []shared.UploadLocation
	References      []shared.UploadLocation
	Implementations []shared.UploadLocation
	HoverText       string
}

// Cursor is a struct that holds the state necessary to resume a locations query from a second or
// subsequent request. This struct is used internally as a request-specific context object that is
// mutated as the locations request is fulfilled. This struct is serialized to JSON then base64
// encoded to make an opaque string that is handed to a future request to get the remainder of the
// result set.
type Cursor struct {
	// the following fields...
	// track the current phase and offset within phase

	Phase                string `json:"p"`    // ""/"local", "remote", or "done"
	LocalUploadOffset    int    `json:"l_uo"` // number of consumed visible uploads
	LocalLocationOffset  int    `json:"l_lo"` // offset within locations of VisibleUploads[LocalUploadOffset:]
	RemoteUploadOffset   int    `json:"r_uo"` // number of searched (to completion) uploads
	RemoteLocationOffset int    `json:"r_lo"` // offset within locations of the current upload batch

	// the following fields...
	// track associated visible/definition uploads and current batch of referencing uploads

	VisibleUploads []CursorVisibleUpload `json:"vus"` // root uploads covering a particular code location
	DefinitionIDs  []int                 `json:"dus"` // identifiers of uploads defining relevant symbol names
	UploadIDs      []int                 `json:"rus"` // current batch of uploads in which to search

	// the following fields...
	// are populated during the local phase, used in the remote phase

	SymbolNames         []string       `json:"ss"` // symbol names extracted from visible uploads
	SkipPathsByUploadID map[int]string `json:"pm"` // paths to skip for particular uploads in the remote phase
}

type CursorVisibleUpload struct {
	UploadID              int             `json:"id"`
	TargetPath            string          `json:"path"`
	TargetPathWithoutRoot string          `json:"path_no_root"` // TODO - can store these differently?
	TargetPosition        shared.Position `json:"pos"`          // TODO - inline
}

var exhaustedCursor = Cursor{Phase: "done"}

func (c Cursor) BumpLocalLocationOffset(n, totalCount int) Cursor {
	c.LocalLocationOffset += n
	if c.LocalLocationOffset >= totalCount {
		// We've consumed this upload completely. Skip it the next time we find
		// ourselves in this loop, and ensure that we start with a zero offset on
		// the next upload we process (if any).
		c.LocalUploadOffset++
		c.LocalLocationOffset = 0
	}

	return c
}

func (c Cursor) BumpRemoteUploadOffset(n, totalCount int) Cursor {
	c.RemoteUploadOffset += n
	if c.RemoteUploadOffset >= totalCount {
		// We've consumed all upload batches
		c.RemoteUploadOffset = -1
	}

	return c
}

func (c Cursor) BumpRemoteLocationOffset(n, totalCount int) Cursor {
	c.RemoteLocationOffset += n
	if c.RemoteLocationOffset >= totalCount {
		// We've consumed the locations for this set of uploads. Reset this slice value in the
		// cursor so that the next call to this function will query the new set of uploads to
		// search in while resolving the next page. We also ensure we start on a zero offset
		// for the next page of results for a fresh set of uploads (if any).
		c.UploadIDs = nil
		c.RemoteLocationOffset = 0
	}

	return c
}

// referencesCursor stores (enough of) the state of a previous References request used to
// calculate the offset into the result set to be returned by the current request.
type ReferencesCursor struct {
	CursorsToVisibleUploads []CursorToVisibleUpload        `json:"adjustedUploads"`
	OrderedMonikers         []precise.QualifiedMonikerData `json:"orderedMonikers"`
	Phase                   string                         `json:"phase"`
	LocalCursor             LocalCursor                    `json:"localCursor"`
	RemoteCursor            RemoteCursor                   `json:"remoteCursor"`
}

// ImplementationsCursor stores (enough of) the state of a previous Implementations request used to
// calculate the offset into the result set to be returned by the current request.
type ImplementationsCursor struct {
	CursorsToVisibleUploads       []CursorToVisibleUpload        `json:"visibleUploads"`
	OrderedImplementationMonikers []precise.QualifiedMonikerData `json:"orderedImplementationMonikers"`
	OrderedExportMonikers         []precise.QualifiedMonikerData `json:"orderedExportMonikers"`
	Phase                         string                         `json:"phase"`
	LocalCursor                   LocalCursor                    `json:"localCursor"`
	RemoteCursor                  RemoteCursor                   `json:"remoteCursor"`
}

// cursorAdjustedUpload
type CursorToVisibleUpload struct {
	UploadID              int             `json:"uploadID"`
	TargetPath            string          `json:"adjustedPath"`
	TargetPosition        shared.Position `json:"adjustedPosition"`
	TargetPathWithoutRoot string          `json:"adjustedPathInBundle"`
}

// localCursor is an upload offset and a location offset within that upload.
type LocalCursor struct {
	UploadOffset int `json:"uploadOffset"`
	// The location offset within the associated upload.
	LocationOffset int `json:"locationOffset"`
}

// RemoteCursor is an upload offset, the current batch of uploads, and a location offset within the batch of uploads.
type RemoteCursor struct {
	UploadOffset   int   `json:"batchOffset"`
	UploadBatchIDs []int `json:"uploadBatchIDs"`
	// The location offset within the associated batch of uploads.
	LocationOffset int `json:"locationOffset"`
}
