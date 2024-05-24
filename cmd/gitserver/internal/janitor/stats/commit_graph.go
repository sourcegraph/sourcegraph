package stats

import (
	"bufio"
	"bytes"
	"fmt"
	"os"

	"github.com/sourcegraph/sourcegraph/cmd/gitserver/internal/common"
	"github.com/sourcegraph/sourcegraph/lib/errors"
)

// CommitGraphInfo returns information about the commit-graph of a repository.
type CommitGraphInfo struct {
	// Exists tells whether a commit-graph exists.
	Exists bool
	// CommitGraphChainLength is the length of the commit-graph chain, if it exists. If the
	// repository does not have a commit-graph chain but a monolithic commit-graph, then this
	// field will be set to 0.
	CommitGraphChainLength uint64
	// HasBloomFilters tells whether the commit-graph has bloom filters. Bloom filters are used
	// to answer the question whether a certain path has been changed in the commit the bloom
	// filter applies to.
	HasBloomFilters bool
	// HasGenerationData tells whether the commit-graph has generation data. Generation
	// data is stored as the corrected committer date, which is defined as the maximum
	// of the commit's own committer date or the corrected committer date of any of its
	// parents. This data can be used to determine whether a commit A comes after a
	// certain commit B.
	HasGenerationData bool
	// HasGenerationDataOverflow stores overflow data in case the corrected committer
	// date takes more than 31 bits to represent.
	HasGenerationDataOverflow bool
}

// CommitGraphInfoForRepository derives information about commit-graphs in the repository.
//
// Please refer to https://git-scm.com/docs/commit-graph#_file_layout for further information about
// the commit-graph format.
func CommitGraphInfoForRepository(repo common.GitDir) (CommitGraphInfo, error) {
	const chunkTableEntrySize = 12

	var info CommitGraphInfo

	commitGraphChainPath := repo.Path("objects", "info", "commit-graphs", "commit-graph-chain")

	var commitGraphPaths []string
	// We first try to read the commit-graphs-chain in the repository.
	if chainData, err := os.ReadFile(commitGraphChainPath); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return CommitGraphInfo{}, fmt.Errorf("reading commit-graphs chain: %w", err)
		}

		// If we couldn't find it, we check whether the monolithic commit-graph file exists
		// and use that instead.
		commitGraphPath := repo.Path("objects", "info", "commit-graph")
		if _, err := os.Stat(commitGraphPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return CommitGraphInfo{Exists: false}, nil
			}

			return CommitGraphInfo{}, fmt.Errorf("statting commit-graph: %w", err)
		}

		commitGraphPaths = []string{commitGraphPath}

		info.Exists = true
	} else {
		// Otherwise, if we have found the commit-graph-chain, we use the IDs it contains as
		// the set of commit-graphs to check further down below.
		ids := bytes.Split(bytes.TrimSpace(chainData), []byte{'\n'})

		commitGraphPaths = make([]string, 0, len(ids))
		for _, id := range ids {
			commitGraphPaths = append(commitGraphPaths,
				repo.Path("objects", "info", "commit-graphs", fmt.Sprintf("graph-%s.graph", id)),
			)
		}

		info.Exists = true
		info.CommitGraphChainLength = uint64(len(commitGraphPaths))
	}

	for _, graphFilePath := range commitGraphPaths {
		graphFile, err := os.Open(graphFilePath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// concurrently modified
				continue
			}
			return CommitGraphInfo{}, fmt.Errorf("read commit graph chain file: %w", err)
		}
		defer graphFile.Close()

		reader := bufio.NewReader(graphFile)
		// The header format is defined in gitformat-commit-graph(5).
		header := []byte{
			0, 0, 0, 0, // 4-byte signature: The signature is: {'C', 'G', 'P', 'H'}
			0, // 1-byte version number: Currently, the only valid version is 1.
			0, // 1-byte Hash Version
			0, // 1-byte number (C) of "chunks"
			0, // 1-byte number (B) of base commit-graphs
		}

		if n, err := reader.Read(header); err != nil {
			return CommitGraphInfo{}, fmt.Errorf("read commit graph file %q header: %w", graphFilePath, err)
		} else if n != len(header) {
			return CommitGraphInfo{}, fmt.Errorf("commit graph file %q is too small, no header", graphFilePath)
		}

		if !bytes.Equal(header[:4], []byte("CGPH")) {
			return CommitGraphInfo{}, fmt.Errorf("commit graph file %q doesn't have signature", graphFilePath)
		}
		if header[4] != 1 {
			return CommitGraphInfo{}, fmt.Errorf("commit graph file %q has unsupported version number: %v", graphFilePath, header[4])
		}

		C := header[6] // number (C) of "chunks"
		table := make([]byte, (C+1)*chunkTableEntrySize)
		if n, err := reader.Read(table); err != nil {
			return CommitGraphInfo{}, fmt.Errorf("read commit graph file %q table of contents for the chunks: %w", graphFilePath, err)
		} else if n != len(table) {
			return CommitGraphInfo{}, fmt.Errorf("commit graph file %q is too small, no table of contents", graphFilePath)
		}

		if err := graphFile.Close(); err != nil {
			return CommitGraphInfo{}, fmt.Errorf("commit graph file %q close: %w", graphFilePath, err)
		}

		if !info.HasBloomFilters {
			info.HasBloomFilters = bytes.Contains(table, []byte("BIDX")) && bytes.Contains(table, []byte("BDAT"))
		}

		if !info.HasGenerationData {
			info.HasGenerationData = bytes.Contains(table, []byte("GDA2"))
		}

		if !info.HasGenerationDataOverflow {
			info.HasGenerationDataOverflow = bytes.Contains(table, []byte("GDO2"))
		}
	}

	return info, nil
}
