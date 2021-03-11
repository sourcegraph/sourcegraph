package conversion

import (
	"sort"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/sourcegraph/sourcegraph/enterprise/lib/codeintel/semantic"
)

func sortedRangeIDs(ranges map[semantic.ID]semantic.RangeData) []semantic.ID {
	var rngIDs []semantic.ID
	for rngID := range ranges {
		rngIDs = append(rngIDs, rngID)
	}

	sort.Slice(rngIDs, func(i, j int) bool {
		return semantic.CompareRanges(ranges[rngIDs[i]], ranges[rngIDs[j]]) < 0
	})

	return rngIDs
}

func getDefRef(resultID semantic.ID, meta semantic.MetaData, resultChunks map[int]semantic.ResultChunkData) ([]semantic.DocumentIDRangeID, semantic.ResultChunkData) {
	chunkID := semantic.HashKey(resultID, meta.NumResultChunks)
	chunk := resultChunks[chunkID]
	docRngIDs := chunk.DocumentIDRangeIDs[resultID]
	return docRngIDs, chunk
}

func newID() (semantic.ID, error) {
	uuid, err := uuid.NewRandom()
	if err != nil {
		return "", err
	}
	return semantic.ID(uuid.String()), nil
}

func makeKey(parts ...string) string {
	return strings.Join(parts, ":")
}

func toID(id int) semantic.ID {
	if id == 0 {
		return semantic.ID("")
	}

	return semantic.ID(strconv.FormatInt(int64(id), 10))
}
