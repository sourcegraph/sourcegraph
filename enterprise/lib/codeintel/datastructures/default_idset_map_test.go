package datastructures

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDefaultIDSetMapAdd(t *testing.T) {
	for _, numUnrelatedKeys := range []int{0, 1, 16} {
		for _, max := range []int{SmallSetThreshold / 2, SmallSetThreshold, SmallSetThreshold * 16} {
			name := fmt.Sprintf("numUnrelatedKeys=%d max=%d", numUnrelatedKeys, max)

			t.Run(name, func(t *testing.T) {
				m := NewDefaultIDSetMap()
				for i := 0; i < numUnrelatedKeys; i++ {
					m.SetAdd(1000+i, 42)
				}

				for i := 1; i <= max; i++ {
					m.SetAdd(50, i)
				}

				if m.SetLen(50) != max {
					t.Errorf("unexpected length. want=%d have=%d", max, m.SetLen(50))
					return
				}

				for i := 1; i <= max; i++ {
					if !m.SetContains(50, i) {
						t.Errorf("unexpected contains. want=%v have=%v", true, m.SetContains(50, i))
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
				for i := 0; i < numUnrelatedKeys; i++ {
					m.SetAdd(1000+i, 42)
				}

				for i := 1; i <= max; i++ {
					if i%2 == 0 {
						m.SetAdd(50, i)
					}
					if i%3 == 0 {
						m.SetAdd(51, i)
					}
				}

				m.SetUnion(50, m.Get(51))

				if m.SetLen(50) != (max/2)+(max/3)-(max/6) {
					t.Errorf("unexpected length. want=%d have=%d", (max/2)+(max/3)-(max/6), m.SetLen(50))
				}

				for i := 1; i <= max/2; i++ {
					expected := (i%2 == 0) || (i%3 == 0)

					if m.SetContains(50, i) != expected {
						t.Errorf("unexpected contains. want=%v have=%v", expected, m.SetContains(50, i))
					}
				}
			})
		}
	}
}

func TestDefaultIDSetMapDelete(t *testing.T) {
	for _, unrelatedKey := range []int{0, 1, 16} {
		m := NewDefaultIDSetMap()
		for i := 0; i < unrelatedKey; i++ {
			m.SetAdd(1000+i, 42)
		}

		m.SetAdd(50, 51)
		m.Delete(50)

		if v := m.Get(50); v != nil {
			t.Errorf("unexpected set: %v", v)
		}
	}
}

func TestDefaultIDSetMapMultipleValues(t *testing.T) {
	m := NewDefaultIDSetMap()
	m.SetAdd(50, 51)
	m.SetAdd(50, 52)
	m.SetAdd(51, 53)
	m.SetAdd(51, 54)
	m.SetAdd(52, 55)

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
