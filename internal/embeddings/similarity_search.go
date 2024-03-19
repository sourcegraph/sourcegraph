package embeddings

import (
	"container/heap"
	"math"
	"sort"

	"github.com/sourcegraph/conc"
	"github.com/sourcegraph/sourcegraph/internal/api"
)

type nearestNeighbor struct {
	index        int
	scoreDetails SearchScoreDetails
}

type nearestNeighborsHeap struct {
	neighbors []nearestNeighbor
}

func (nn *nearestNeighborsHeap) Len() int { return len(nn.neighbors) }

func (nn *nearestNeighborsHeap) Less(i, j int) bool {
	return nn.neighbors[i].scoreDetails.Score < nn.neighbors[j].scoreDetails.Score
}

func (nn *nearestNeighborsHeap) Swap(i, j int) {
	nn.neighbors[i], nn.neighbors[j] = nn.neighbors[j], nn.neighbors[i]
}

func (nn *nearestNeighborsHeap) Push(x any) {
	nn.neighbors = append(nn.neighbors, x.(nearestNeighbor))
}

func (nn *nearestNeighborsHeap) Pop() any {
	old := nn.neighbors
	n := len(old)
	x := old[n-1]
	nn.neighbors = old[0 : n-1]
	return x
}

func (nn *nearestNeighborsHeap) Peek() nearestNeighbor {
	return nn.neighbors[0]
}

func newNearestNeighborsHeap() *nearestNeighborsHeap {
	nn := &nearestNeighborsHeap{neighbors: make([]nearestNeighbor, 0)}
	heap.Init(nn)
	return nn
}

type partialRows struct {
	start int
	end   int
}

// splitRows splits nRows into nWorkers equal (or nearly equal) sized chunks.
func splitRows(numRows int, numWorkers int, minRowsToSplit int) []partialRows {
	if numWorkers == 1 || numRows <= numWorkers || numRows <= minRowsToSplit {
		return []partialRows{{0, numRows}}
	}
	nRowsPerWorker := int(math.Ceil(float64(numRows) / float64(numWorkers)))

	rowsPerWorker := make([]partialRows, numWorkers)
	for i := range numWorkers {
		rowsPerWorker[i] = partialRows{
			start: min(i*nRowsPerWorker, numRows),
			end:   min((i+1)*nRowsPerWorker, numRows),
		}
	}
	return rowsPerWorker
}

type WorkerOptions struct {
	NumWorkers int
	// MinRowsToSplit indicates the minimum number of rows that should be split
	// among the workers. If numRows <= MinRowsToSplit, then we use a single worker
	// to process the index, regardless of the NumWorkers option.
	MinRowsToSplit int
}

// SimilaritySearch finds the `nResults` most similar rows to a query vector. It uses the cosine similarity metric.
// IMPORTANT: The vectors in the embedding index have to be normalized for similarity search to work correctly.
func (index *EmbeddingIndex) SimilaritySearch(
	query []int8,
	numResults int,
	workerOptions WorkerOptions,
	opts SearchOptions,
	repoName api.RepoName,
	revision api.CommitID,
) []EmbeddingSearchResult {
	if numResults == 0 || len(index.Embeddings) == 0 {
		return nil
	}

	numRows := len(index.RowMetadata)
	// Cannot request more results than there are rows.
	numResults = min(numRows, numResults)
	// We need at least 1 worker.
	numWorkers := max(1, workerOptions.NumWorkers)

	// Split index rows among the workers. Each worker will run a partial similarity search on the assigned rows.
	rowsPerWorker := splitRows(numRows, numWorkers, workerOptions.MinRowsToSplit)
	heaps := make([]*nearestNeighborsHeap, len(rowsPerWorker))

	if len(rowsPerWorker) > 1 {
		var wg conc.WaitGroup
		for workerIdx := range len(rowsPerWorker) {
			// Capture the loop variable value so we can use it in the closure below.
			workerIdx := workerIdx
			wg.Go(func() {
				heaps[workerIdx] = index.partialSimilaritySearch(query, numResults, rowsPerWorker[workerIdx], opts)
			})
		}
		wg.Wait()
	} else {
		// Run the similarity search directly when we have a single worker to eliminate the concurrency overhead.
		heaps[0] = index.partialSimilaritySearch(query, numResults, rowsPerWorker[0], opts)
	}

	// Collect all heap neighbors from workers into a single array.
	neighbors := make([]nearestNeighbor, 0, len(rowsPerWorker)*numResults)
	for _, heap := range heaps {
		if heap != nil {
			neighbors = append(neighbors, heap.neighbors...)
		}
	}
	// And re-sort it according to the score (descending).
	sort.Slice(neighbors, func(i, j int) bool { return neighbors[i].scoreDetails.Score > neighbors[j].scoreDetails.Score })

	// Take top neighbors and return them as results.
	results := make([]EmbeddingSearchResult, numResults)

	for idx := range min(numResults, len(neighbors)) {
		metadata := index.RowMetadata[neighbors[idx].index]
		results[idx] = EmbeddingSearchResult{
			RepoName:     repoName,
			Revision:     revision,
			FileName:     metadata.FileName,
			StartLine:    metadata.StartLine,
			EndLine:      metadata.EndLine,
			ScoreDetails: neighbors[idx].scoreDetails,
		}
	}

	return results
}

func (index *EmbeddingIndex) partialSimilaritySearch(query []int8, numResults int, partialRows partialRows, opts SearchOptions) *nearestNeighborsHeap {
	nRows := partialRows.end - partialRows.start
	if nRows <= 0 {
		return nil
	}
	numResults = min(nRows, numResults)

	nnHeap := newNearestNeighborsHeap()
	for i := partialRows.start; i < partialRows.start+numResults; i++ {
		scoreDetails := index.score(query, i, opts)
		heap.Push(nnHeap, nearestNeighbor{index: i, scoreDetails: scoreDetails})
	}

	for i := partialRows.start + numResults; i < partialRows.end; i++ {
		scoreDetails := index.score(query, i, opts)
		// Add row if it has greater similarity than the smallest similarity in the heap.
		// This way we ensure keep a set of the highest similarities in the heap.
		if scoreDetails.Score > nnHeap.Peek().scoreDetails.Score {
			heap.Pop(nnHeap)
			heap.Push(nnHeap, nearestNeighbor{index: i, scoreDetails: scoreDetails})
		}
	}

	return nnHeap
}

const (
	scoreFileRankWeight   int32 = 1
	scoreSimilarityWeight int32 = 2
)

func (index *EmbeddingIndex) score(query []int8, i int, opts SearchOptions) SearchScoreDetails {
	similarityScore := scoreSimilarityWeight * Dot(index.Row(i), query)

	// handle missing ranks
	rankScore := int32(0)
	if opts.UseDocumentRanks && len(index.Ranks) > i {
		// The file rank represents a log (base 2) count. The log ranks should be
		// bounded at 32, but we cap it just in case to ensure it falls in the range [0,
		// 1]. I am not using math.Min here to avoid the back and forth conversion
		// between float64 and float32.
		normalizedRank := index.Ranks[i] / 32.0
		if normalizedRank > 1.0 {
			normalizedRank = 1.0
		}
		rankScore = int32(float32(scoreFileRankWeight) * normalizedRank)
	}

	return SearchScoreDetails{
		Score:           similarityScore + rankScore,
		SimilarityScore: similarityScore,
		RankScore:       rankScore,
	}
}

type SearchOptions struct {
	UseDocumentRanks bool
}
