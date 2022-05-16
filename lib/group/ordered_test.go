package group

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestOrderedGroup(t *testing.T) {
	sleepReturn := func(d time.Duration, r int) func() int {
		return func() int {
			time.Sleep(d)
			return r
		}
	}

	t.Run("in order", func(t *testing.T) {
		var results []int
		g := NewParallelOrdered(10, func(i int) { results = append(results, i) })
		g.Submit(sleepReturn(5*time.Millisecond, 1))
		g.Submit(sleepReturn(10*time.Millisecond, 2))
		g.Submit(sleepReturn(15*time.Millisecond, 3))
		g.Done()
		require.Equal(t, []int{1, 2, 3}, results)
	})

	t.Run("out of order", func(t *testing.T) {
		var results []int
		g := NewParallelOrdered(10, func(i int) { results = append(results, i) })
		g.Submit(sleepReturn(15*time.Millisecond, 1))
		g.Submit(sleepReturn(10*time.Millisecond, 2))
		g.Submit(sleepReturn(5*time.Millisecond, 3))
		g.Done()
		require.Equal(t, []int{1, 2, 3}, results)
	})

	t.Run("no parallel", func(t *testing.T) {
		var results []int
		g := NewParallelOrdered(1, func(i int) { results = append(results, i) })
		g.Submit(sleepReturn(15*time.Millisecond, 1))
		g.Submit(sleepReturn(10*time.Millisecond, 2))
		g.Submit(sleepReturn(5*time.Millisecond, 3))
		g.Done()
		require.Equal(t, []int{1, 2, 3}, results)
	})

	t.Run("very parallel", func(t *testing.T) {
		var results []int
		g := NewParallelOrdered(20, func(i int) { results = append(results, i) })
		expected := make([]int, 100)
		for i := 0; i < 100; i++ {
			g.Submit(sleepReturn(10*time.Millisecond, i))
			expected[i] = i
		}
		g.Done()
		require.Equal(t, expected, results)
	})
}
