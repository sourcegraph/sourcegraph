package datastructures

// DisjointIDSet is a modified disjoint set or union-find data structure. This allows
// linking items together and retrieving the set of all items for a given set.
type DisjointIDSet = DefaultIDSetMap

// NewDisjointIDSet creates a new empty disjoint identifier set.
func NewDisjointIDSet() *DisjointIDSet {
	return &DisjointIDSet{}
}

// DisjointIDSetWith creates a disjoint identifier set with the given linked pairs.
func DisjointIDSetWith(pairs ...int) *DisjointIDSet {
	if len(pairs)%2 != 0 {
		panic("DisjointIDSetWith must be supplied pairs of values")
	}

	s := NewDisjointIDSet()
	for i := 0; i < len(pairs); i += 2 {
		s.Link(pairs[i], pairs[i+1])
	}

	return s
}

// Link composes the connected components containing the given identifiers. If one or
// the other value is already in the set, then the sets of the two values will merge.
func (sm *DisjointIDSet) Link(id1, id2 int) {
	sm.SetAdd(id1, id2)
	sm.SetAdd(id2, id1)
}

// ExtractSet returns a set of all values reachable from the given source value. The
// resulting set would be reachable using any of the values as the source.
func (sm *DisjointIDSet) ExtractSet(id int) *IDSet {
	s := NewIDSet()
	frontier := IDSetWith(id)

	var v int
	for frontier.Pop(&v) {
		if !s.Contains(v) {
			s.Add(v)
			frontier.Union(sm.Get(v))
		}
	}

	return s
}
