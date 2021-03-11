package datastructures

// IDSet is a space-efficient set of integer identifiers.
//
// The correlation process creates many sets (e.g., range/moniker relations), most of which
// contain a small handful of elements. There are a fewer number of sets which have a large
// number of elements (e.g., contains relations). This structure tries to hit a balance
// between having a space-efficient representation of small sets, while not affecting the
// add/contains performance of larger sets.
//
// For concrete numbers, here is the distribution of set sizes captured while processing an
// index for aws-sdk-go:
//
// +-----------+----------+
// | size      | num sets |
// +-----------+----------+
// | 1         |  2535310 |
// | 2         |   337648 |
// | 3         |   130404 |
// | 4         |    36795 |
// | 5         |    18968 |
// | 6         |     4456 |
// | 7         |     7060 |
// | 8         |     2834 |
// | 9         |     5327 |
// | 10        |     5753 |
// | 11        |     2795 |
// | 12        |     2913 |
// | 13        |     1404 |
// | 14        |     2686 |
// | 15        |     1089 |
// | 16        |     1281 |
// | 17-312329 |    13224 |
// +-----------+----------+
//
// Each set starts out as "small", where operations operate on a slice. Insertion and contain
// operations require a linear scan, but this is alright as the values are packed together and
// should reside in the same cache line.
//
// Once a set exceeds the small set threshold, it is upgraded to a "large" set, where the
// elements of the set are written to an int-keyed map. Maps have a larger overhead than slices
// (see https://golang.org/src/runtime/map.go#L115), so we only want to pay this cost when the
// performance of using a slice outweighs the memory savings.
type IDSet struct {
	s []int            // small set
	m map[int]struct{} // large set
}

// SmallSetThreshold is the maximum number of elements in a small set. If the size
// of a set will exceed this size on insert, it will be converted into a large set.
const SmallSetThreshold = 16

// NewIDSet creates a new empty identifier set.
func NewIDSet() *IDSet {
	return &IDSet{}
}

// IDSetWith creates an identifier set populated with the given identifiers.
func IDSetWith(ids ...int) *IDSet {
	s := NewIDSet()

	s.ensure(len(ids))
	for _, id := range ids {
		s.add(id)
	}

	return s
}

// Len returns the number of identifiers in the identifier set.
func (s *IDSet) Len() int {
	return len(s.s) + len(s.m)
}

// Contains determines if the given identifier belongs to the set.
func (s *IDSet) Contains(id int) bool {
	for _, v := range s.s {
		if id == v {
			return true
		}
	}

	_, ok := s.m[id]
	return ok
}

// Each invokes the given function with each identifier of the set.
func (s *IDSet) Each(f func(id int)) {
	for _, id := range s.s {
		f(id)
	}
	for id := range s.m {
		f(id)
	}
}

// Add inserts an identifier into the set.
func (s *IDSet) Add(id int) {
	s.ensure(1)
	s.add(id)
}

// Union inserts all the identifiers of other into the set.
func (s *IDSet) Union(other *IDSet) {
	if other == nil {
		return
	}

	if other.m == nil {
		s.ensure(len(other.s))
		for _, id := range other.s {
			s.add(id)
		}
	} else {
		s.ensure(len(other.m))
		for id := range other.m {
			s.add(id)
		}
	}
}

// add inserts an identifier into the set. This method assumes that ensure has
// already been called.
func (s *IDSet) add(id int) {
	if s.m != nil {
		s.m[id] = struct{}{}
	} else if !s.Contains(id) {
		s.s = append(s.s, id)
	}
}

// Min returns the minimum identifier of the set. If there are no identifiers,
// this method returns a false-valued flag.
func (s *IDSet) Min() (int, bool) {
	min := 0
	for _, id := range s.s {
		if min == 0 || id < min {
			min = id
		}
	}

	for id := range s.m {
		if min == 0 || id < min {
			min = id
		}
	}

	return min, s.Len() > 0
}

// Pop removes an an arbitrary identifier from the set and assigns it to the
// given target. If there are no identifier, this method returns false.
func (s *IDSet) Pop(id *int) bool {
	if n := len(s.s); n > 0 {
		*id, s.s = s.s[n-1], s.s[:n-1]
		return true
	}

	for v := range s.m {
		*id = v
		delete(s.m, v)
		return true
	}

	return false
}

// ensure will convert a small set to a large set if adding n elements would cause
// the set to exceed the small set threshold.
func (s *IDSet) ensure(n int) {
	if s.m != nil || len(s.s)+n <= SmallSetThreshold {
		return
	}

	m := make(map[int]struct{}, len(s.s)+n)
	for _, id := range s.s {
		m[id] = struct{}{}
	}

	s.m = m
	s.s = nil
}

// compareIDSets returns true if the given identifier sets contains equivalent elements.
func compareIDSets(x, y *IDSet) (found bool) {
	if x == nil && y == nil {
		return true
	}

	if x == nil || y == nil || x.Len() != y.Len() {
		return false
	}

	found = true
	x.Each(func(i int) { found = found && y.Contains(i) })
	return found
}
