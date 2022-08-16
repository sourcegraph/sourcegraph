package streaming

import (
	"sort"
)

type aggregated struct {
	resultBufferSize int
	smallestResult   *Aggregate
	Results          map[string]int32
	OtherCount       OtherCount
}

type Aggregate struct {
	Label string
	Count int32
}

type OtherCount struct {
	ResultCount int32
	GroupCount  int32
}

// Add performs best-effort aggregation for a (label, count) search result.
func (a *aggregated) Add(label string, count int32) {
	// 1. We have a match in our in-memory map. Update and update the smallest result.
	// 2. We haven't hit the max buffer size. Add to our in-memory map and update the smallest result.
	// 3. We don't have a match but have a better result than our smallest. Update the overflow by ejected smallest.
	// 4. We don't have a match or a better result. Update the overflow by the hit count.
	if _, ok := a.Results[label]; !ok {
		if len(a.Results) < a.resultBufferSize {
			a.Results[label] = count
			a.updateSmallestAggregate()
		} else {
			newResult := &Aggregate{label, count}
			if a.smallestResult.Less(newResult) {
				a.updateOtherCount(a.smallestResult.Count, 1)
				delete(a.Results, a.smallestResult.Label)
				a.Results[label] = count
				a.updateSmallestAggregate()
			} else {
				a.updateOtherCount(count, 1)
			}
		}
	} else {
		a.Results[label] += count
		a.updateSmallestAggregate()
	}
}

// findSmallestAggregate finds the result with the smallest count and returns it.
func (a *aggregated) findSmallestAggregate() *Aggregate {
	var smallestAggregate *Aggregate
	for label, count := range a.Results {
		tempSmallest := &Aggregate{label, count}
		if smallestAggregate == nil || tempSmallest.Less(smallestAggregate) {
			smallestAggregate = tempSmallest
		}
	}
	return smallestAggregate
}

func (a *aggregated) updateSmallestAggregate() {
	smallestResult := a.findSmallestAggregate()
	if smallestResult != nil {
		a.smallestResult = smallestResult
	}
}

func (a *aggregated) updateOtherCount(resultCount, groupCount int32) {
	a.OtherCount.ResultCount += resultCount
	a.OtherCount.GroupCount += groupCount
}

// SortAggregate sorts aggregated results into a slice of descending order.
func (a aggregated) SortAggregate() []*Aggregate {
	aggregateSlice := make([]*Aggregate, 0, len(a.Results))
	for val, count := range a.Results {
		aggregateSlice = append(aggregateSlice, &Aggregate{val, count})
	}
	// Sort in descending order.
	sort.Slice(aggregateSlice, func(i int, j int) bool {
		return aggregateSlice[j].Less(aggregateSlice[i])
	})

	return aggregateSlice
}

func (a *Aggregate) Less(b *Aggregate) bool {
	if b == nil {
		return false
	}
	if a.Count == b.Count {
		// Sort alphabetically if of same count.
		return a.Label <= b.Label
	}
	return a.Count < b.Count
}
