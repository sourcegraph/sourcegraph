package codeintelutils

import (
	"bufio"
	"encoding/json"
	"io"

	"github.com/pkg/errors"
)

// MaxBufferSize is the maximum size of the metaData line in the dump. This should be large enough
// to be able to read the output of lsif-tsc for most cases, which will contain all glob-expanded
// file names in the indexing of JavaScript projects.
//
// Data point: lodash's metaData vertex constructed by the args `*.js test/*.js --AllowJs --checkJs`
// is 10639 characters long.
const MaxBufferSize = 128 * 1024

// ErrMetadataExceedsBuffer occurs when the first line of an LSIF index is too long to read.
var ErrMetadataExceedsBuffer = errors.New("metaData vertex exceeds buffer")

// ErrInvalidMetaDataVertex occurs when the first line of an LSIF index is not a valid metadata vertex.
var ErrInvalidMetaDataVertex = errors.New("invalid metaData vertex")

type metaDataVertex struct {
	Label    string   `json:"label"`
	ToolInfo toolInfo `json:"toolInfo"`
}

type toolInfo struct {
	Name string `json:"name"`
}

// ReadIndexerName returns the name of the tool that generated the given index contents.
// This function reads only the first line of the file, where the metadata vertex is
// assumed to be in all valid dumps.
func ReadIndexerName(r io.Reader) (string, error) {
	line, isPrefix, err := bufio.NewReaderSize(r, MaxBufferSize).ReadLine()
	if err != nil {
		return "", err
	}
	if isPrefix {
		return "", ErrMetadataExceedsBuffer
	}

	meta := metaDataVertex{}
	if err := json.Unmarshal(line, &meta); err != nil {
		return "", ErrInvalidMetaDataVertex
	}

	if meta.Label != "metaData" || meta.ToolInfo.Name == "" {
		return "", ErrInvalidMetaDataVertex
	}

	return meta.ToolInfo.Name, nil
}
