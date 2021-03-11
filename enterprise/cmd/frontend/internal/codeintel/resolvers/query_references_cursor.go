package resolvers

import (
	"encoding/base64"
	"encoding/json"

	"github.com/sourcegraph/sourcegraph/enterprise/internal/codeintel/stores/lsifstore"
	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/semantic"
)

// referencesCursor stores (enough of) the state of a previous References request used to
// calculate the offset into the result set to be returned by the current request.
type referencesCursor struct {
	AdjustedUploads           []cursorAdjustedUpload          `json:"adjustedUploads"`
	DefinitionUploadIDs       []int                           `json:"definitionUploadIDs"`
	DefinitionUploadIDsCached bool                            `json:"definitionUploadIDsCached"`
	OrderedMonikers           []semantic.QualifiedMonikerData `json:"orderedMonikers"`
	RemotePhase               bool                            `json:"remotePhase"`
	LocalOffset               int                             `json:"localOffset"`
	LocalBatchOffset          int                             `json:"localBatchOffset"`
	BatchIDs                  []int                           `json:"batchIDs"`
	RemoteOffset              int                             `json:"remoteOffset"`
	RemoteBatchOffset         int                             `json:"remoteBatchOffset"`
}

type cursorAdjustedUpload struct {
	DumpID               int                `json:"dumpID"`
	AdjustedPath         string             `json:"adjustedPath"`
	AdjustedPosition     lsifstore.Position `json:"adjustedPosition"`
	AdjustedPathInBundle string             `json:"adjustedPathInBundle"`
}

// decodeCursor is the inverse of encodeCursor. If the given encoded string is empty, then
// a fresh cursor is returned.
func decodeCursor(rawEncoded string) (referencesCursor, error) {
	if rawEncoded == "" {
		return referencesCursor{}, nil
	}

	raw, err := base64.RawURLEncoding.DecodeString(rawEncoded)
	if err != nil {
		return referencesCursor{}, err
	}

	var cursor referencesCursor
	err = json.Unmarshal(raw, &cursor)
	return cursor, err
}

// encodeCursor returns an encoding of the given cursor suitable for a URL or a GraphQL token.
func encodeCursor(cursor referencesCursor) string {
	rawEncoded, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(rawEncoded)
}
