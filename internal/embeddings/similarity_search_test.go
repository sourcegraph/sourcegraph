package embeddings

import (
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Data below was generated using the testdata/generate_similarity_search_test_data.py script.

// Each line represents a separate embedding.
var embeddings = []int8{
	64, 83, 70,
	73, 56, 86,
	40, 81, 88,
	47, 97, 65,
	66, 108, 8,
	13, 3, 126,
	64, 72, 81,
	83, 48, 82,
	22, 121, 27,
	103, 57, 45,
	35, 104, 61,
	85, 2, 93,
	60, 61, 93,
	97, 51, 62,
	91, 7, 87,
	119, 37, 22,
}

// Each line represents a separate query.
var queries = []int8{
	53, 61, 97,
	51, 115, 11,
	37, 29, 117,
}

// Each subarray contains ranked nearest neighbors for each query.
var ranks = [][]int{
	{12, 6, 1, 2, 7, 0, 3, 13, 10, 14, 11, 9, 5, 8, 4, 15},
	{4, 8, 10, 3, 0, 6, 2, 9, 13, 1, 12, 7, 15, 14, 11, 5},
	{5, 12, 1, 2, 11, 7, 6, 14, 0, 13, 3, 10, 9, 15, 8, 4},
}

func TestSimilaritySearch(t *testing.T) {
	numRows, numQueries, columnDimension := 16, 3, 3
	index := EmbeddingIndex{
		Embeddings:      embeddings,
		ColumnDimension: columnDimension,
		RowMetadata:     []RepoEmbeddingRowMetadata{},
	}

	for i := range numRows {
		index.RowMetadata = append(index.RowMetadata, RepoEmbeddingRowMetadata{FileName: strconv.Itoa(i)})
	}

	for _, numWorkers := range []int{0, 1, 2, 3, 5, 8, 9, 16, 20, 33} {
		for _, numResults := range []int{32} {
			for q := range numQueries {
				t.Run(fmt.Sprintf("find nearest neighbors query=%d numResults=%d numWorkers=%d", q, numResults, numWorkers), func(t *testing.T) {
					query := queries[q*columnDimension : (q+1)*columnDimension]
					results := index.SimilaritySearch(query, numResults, WorkerOptions{NumWorkers: numWorkers, MinRowsToSplit: 0}, SearchOptions{}, "", "")
					resultRowNums := make([]int, len(results))
					for i, r := range results {
						resultRowNums[i], _ = strconv.Atoi(r.FileName)
					}
					expectedResults := ranks[q]
					require.Equal(t, expectedResults[:min(numResults, len(expectedResults))], resultRowNums)
				})
			}
		}
	}
}

func TestSplitRows(t *testing.T) {
	tests := []struct {
		numRows        int
		numWorkers     int
		minRowsToSplit int
		want           []partialRows
	}{
		{
			numRows:    0,
			numWorkers: 1,
			want:       []partialRows{{0, 0}},
		},
		{
			numRows:    128,
			numWorkers: 1,
			want:       []partialRows{{0, 128}},
		},
		{
			numRows:    16,
			numWorkers: 4,
			want:       []partialRows{{0, 4}, {4, 8}, {8, 12}, {12, 16}},
		},
		{
			numRows:    5,
			numWorkers: 4,
			want:       []partialRows{{0, 2}, {2, 4}, {4, 5}, {5, 5}},
		},
		{
			numRows:    16,
			numWorkers: 3,
			want:       []partialRows{{0, 6}, {6, 12}, {12, 16}},
		},
		{
			numRows:        20,
			numWorkers:     5,
			minRowsToSplit: 20,
			want:           []partialRows{{0, 20}},
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("numRows=%d numWorkers=%d", tt.numRows, tt.numWorkers), func(t *testing.T) {
			got := splitRows(tt.numRows, tt.numWorkers, tt.minRowsToSplit)
			require.Equal(t, tt.want, got)
		})
	}
}

func getRandomEmbeddings(prng *rand.Rand, numElements int) []int8 {
	slice := make([]int8, numElements)
	for idx := range slice {
		slice[idx] = int8(prng.Int())
	}
	return slice
}

func BenchmarkSimilaritySearch(b *testing.B) {
	prng := rand.New(rand.NewSource(0))

	numRows := 1_000_000
	numResults := 100
	columnDimension := 1536
	index := &EmbeddingIndex{
		Embeddings:      getRandomEmbeddings(prng, numRows*columnDimension),
		ColumnDimension: columnDimension,
		RowMetadata:     make([]RepoEmbeddingRowMetadata, numRows),
	}
	query := getRandomEmbeddings(prng, columnDimension)

	b.ResetTimer()

	for _, numWorkers := range []int{1, 2, 4, 8, 16} {
		b.Run(fmt.Sprintf("numWorkers=%d", numWorkers), func(b *testing.B) {
			start := time.Now()
			for range b.N {
				_ = index.SimilaritySearch(query, numResults, WorkerOptions{NumWorkers: numWorkers}, SearchOptions{}, "", "")
			}
			m := float64(numRows) * float64(b.N) / time.Since(start).Seconds()
			b.ReportMetric(m, "embeddings/s")
			b.ReportMetric(m/float64(numWorkers), "embeddings/s/worker")
		})
	}
}

func TestScore(t *testing.T) {
	var ranks []float32
	for i := 1; i < len(embeddings); i++ {
		ranks = append(ranks, float32(i))
	}

	columnDimension := 3
	index := &EmbeddingIndex{
		Embeddings:      embeddings,
		ColumnDimension: columnDimension,
		Ranks:           ranks,
	}
	// embeddings[0] = 64, 83, 70,
	// queries[0:3] = 53, 61, 97,
	scoreDetails := index.score(queries[0:columnDimension], 0, SearchOptions{UseDocumentRanks: true})

	// Check that the score is correct
	expectedScore := scoreSimilarityWeight * ((64 * 53) + (83 * 61) + (70 * 97))
	if math.Abs(float64(scoreDetails.Score-expectedScore)) > 0.0001 {
		t.Fatalf("Expected score %d, but got %d", expectedScore, scoreDetails.Score)
	}
}
