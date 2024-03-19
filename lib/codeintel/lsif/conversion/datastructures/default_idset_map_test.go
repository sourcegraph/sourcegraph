package datastructures

import (
	"fmt"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
)

// TODO: Add some fuzz tests when we move to Go 1.18.

func TestDefaultIDSetMapAdd(t *testing.T) {
	for _, numUnrelatedKeys := range []int{0, 1, 16} {
		for _, max := range []int{SmallSetThreshold / 2, SmallSetThreshold, SmallSetThreshold * 16} {
			name := fmt.Sprintf("numUnrelatedKeys=%d max=%d", numUnrelatedKeys, max)

			t.Run(name, func(t *testing.T) {
				m := NewDefaultIDSetMap()
				for i := range numUnrelatedKeys {
					m.AddID(1000+i, 42)
				}

				for i := 1; i <= max; i++ {
					m.AddID(50, i)
				}

				if m.NumIDsForKey(50) != max {
					t.Errorf("unexpected length. want=%d have=%d", max, m.NumIDsForKey(50))
					return
				}

				for i := 1; i <= max; i++ {
					if !m.Contains(50, i) {
						t.Errorf("unexpected contains. want=%v have=%v", true, m.Contains(50, i))
					}
				}
			})
		}
	}
}

func TestDefaultIDSetMapUnion(t *testing.T) {
	for _, numUnrelatedKeys := range []int{0, 1, 16} {
		for _, max := range []int{16, 10000} {
			name := fmt.Sprintf("numUnrelatedKeys=%d max=%d", numUnrelatedKeys, max)

			t.Run(name, func(t *testing.T) {
				m := NewDefaultIDSetMap()
				for i := range numUnrelatedKeys {
					m.AddID(1000+i, 42)
				}

				for i := 1; i <= max; i++ {
					if i%2 == 0 {
						m.AddID(50, i)
					}
					if i%3 == 0 {
						m.AddID(51, i)
					}
				}

				m.UnionIDSet(50, m.Get(51))

				if m.NumIDsForKey(50) != (max/2)+(max/3)-(max/6) {
					t.Errorf("unexpected length. want=%d have=%d", (max/2)+(max/3)-(max/6), m.NumIDsForKey(50))
				}

				for i := 1; i <= max/2; i++ {
					expected := (i%2 == 0) || (i%3 == 0)

					if m.Contains(50, i) != expected {
						t.Errorf("unexpected contains. want=%v have=%v", expected, m.Contains(50, i))
					}
				}
			})
		}
	}
}

func TestDefaultIDSetMapDelete(t *testing.T) {
	for _, unrelatedKey := range []int{0, 1, 16} {
		m := NewDefaultIDSetMap()
		for i := range unrelatedKey {
			m.AddID(1000+i, 42)
		}

		m.AddID(50, 51)
		m.Delete(50)

		if v := m.Get(50); v != nil {
			t.Errorf("unexpected set: %v", v)
		}
	}
}

func TestDefaultIDSetMapMultipleValues(t *testing.T) {
	m := NewDefaultIDSetMap()
	m.AddID(50, 51)
	m.AddID(50, 52)
	m.AddID(51, 53)
	m.AddID(51, 54)
	m.AddID(52, 55)

	for value, expectedSet := range map[int]*IDSet{
		50: IDSetWith(51, 52),
		51: IDSetWith(53, 54),
		52: IDSetWith(55),
		53: nil,
	} {
		name := fmt.Sprintf("value=%d", value)

		t.Run(name, func(t *testing.T) {
			if diff := cmp.Diff(expectedSet, m.Get(value), Comparers...); diff != "" {
				t.Errorf("unexpected set (-want +got):\n%s", diff)
			}
		})
	}
}

// Regression tests

func TestDefaultIDSetMap_Each(t *testing.T) {
	sm := NewDefaultIDSetMap()
	sm.AddID(0, 1)
	counter := 0
	sm.Each(func(_ int, _ *IDSet) {
		counter++
	})
	require.Equal(t, counter, 1)
}

func TestDefaultIDSetMap_NumValuesForKey(t *testing.T) {
	sm := NewDefaultIDSetMap()
	require.NotPanics(t, func() {
		sm.NumIDsForKey(0)
	})
}

func TestDefaultIDSetMap_Contains(t *testing.T) {
	sm := NewDefaultIDSetMap()
	require.NotPanics(t, func() {
		_ = sm.Contains(0, 1)
	})
}

func TestDefaultIDSetMap_EachID(t *testing.T) {
	sm := NewDefaultIDSetMap()
	num := 30
	require.NotPanics(t,
		func() { sm.EachID(0, func(_ int) { num++ }) },
	)
	require.Equal(t, 30, num)
}

func TestDefaultIDSetMap_AddID(t *testing.T) {
	sm := NewDefaultIDSetMap()
	require.NotPanics(t, func() {
		sm.AddID(0, 22)
	})
}

func TestDefaultIDSetMap_UnionIDSet(t *testing.T) {
	sm := NewDefaultIDSetMap()
	idSet := NewIDSet()
	idSet.Add(3)
	require.NotPanics(t, func() {
		sm.UnionIDSet(0, idSet)
	})
}

func TestDefaultIDSetMap_getOrCreate(t *testing.T) {
	sm := NewDefaultIDSetMap()
	require.NotNil(t, sm.getOrCreate(0))
}

func TestDefaultIDSetMap_Pop(t *testing.T) {
	sm := NewDefaultIDSetMap()
	sm.AddID(0, 1)
	sm.AddID(1, 1)

	sm.Pop(2)
	require.Equal(t, mapStateHeap, sm.state())
	expect := DefaultIDSetMapWith(map[int]*IDSet{
		0: IDSetWith(1),
		1: IDSetWith(1),
	})
	if diff := cmp.Diff(expect, sm, Comparers...); diff != "" {
		t.Errorf("unexpected state (-want +got):\n%s", diff)
	}

	sm.Pop(0)
	require.Equal(t, mapStateInline, sm.state())
	expect = DefaultIDSetMapWith(map[int]*IDSet{1: IDSetWith(1)})
	if diff := cmp.Diff(expect, sm, Comparers...); diff != "" {
		t.Errorf("unexpected state (-want +got):\n%s", diff)
	}

	sm.Pop(1)
	require.Equal(t, mapStateEmpty, sm.state())
	expect = DefaultIDSetMapWith(map[int]*IDSet{})
	if diff := cmp.Diff(expect, sm, Comparers...); diff != "" {
		t.Errorf("unexpected state (-want +got):\n%s", diff)
	}
}

func TestDefaultIDSetMap_UnorderedKeys(t *testing.T) {
	sm := NewDefaultIDSetMap()
	require.Equal(t, 0, len(sm.UnorderedKeys()))
	sm.AddID(0, 1)
	sm.AddID(0, 2)
	require.Equal(t, []int{0}, sm.UnorderedKeys())
	sm.AddID(1, 2)
	sortedKeys := sm.UnorderedKeys()
	sort.Ints(sortedKeys)
	require.Equal(t, []int{0, 1}, sortedKeys)
	sm.Delete(1)
	require.Equal(t, []int{0}, sm.UnorderedKeys())
}
