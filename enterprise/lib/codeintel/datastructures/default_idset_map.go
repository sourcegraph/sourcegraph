package datastructures

// DefaultIDSetMap is a space-efficient map from integer keys to identifier sets. This
// map adds convenience operations that will operate on the set at a specific key, and
// create it if it does not yet exist in the map.
//
// The correlation process creates many such maps (e.g. result set and contains relations),
// the majority of which contain only a single element. This structure optimizes for the
// case where only a single key is present in the map.
//
// For concrete numbers, processing an index for aws-sdk-go produces 1.5 million singleton
// maps, and only 25k non-singleton maps.
//
// Each map starts out empty. Insertion of the first element sets a key and value field
// directly in the map struct. Insertion of a second element will promote the struct into
// a non-singleton map and heap-allocate a builtin map to hold additional keys. Maps have
// large overhead (see https://golang.org/src/runtime/map.go#L115), so we only want to pay
// this cost when we need to.
type DefaultIDSetMap struct {
	key   int            // singleton map key
	value *IDSet         // singleton map value
	m     map[int]*IDSet // non-singleton map
}

// NewDefaultIDSetMap creates a new empty default identifier set map.
func NewDefaultIDSetMap() *DefaultIDSetMap {
	return &DefaultIDSetMap{}
}

// DefaultIDSetMapWith creates a default identifier set map with the given contents.
func DefaultIDSetMapWith(m map[int]*IDSet) *DefaultIDSetMap {
	return &DefaultIDSetMap{m: m}
}

// Get returns the identifier set at the given key or nil if it does not exist.
func (sm *DefaultIDSetMap) Get(key int) *IDSet {
	if sm.key == key {
		return sm.value
	}
	if sm.m != nil {
		return sm.m[key]
	}
	return nil
}

// Delete removes the identifier set at the given key if it exists.
func (sm *DefaultIDSetMap) Delete(key int) {
	if sm.key == key {
		sm.key = 0
		sm.value = nil
	}
	if sm.m != nil {
		delete(sm.m, key)
	}
}

// Each invokes the given function with each key and identifier set in the map.
func (sm *DefaultIDSetMap) Each(f func(key int, value *IDSet)) {
	if sm.key != 0 {
		f(sm.key, sm.value)
	}
	if sm.m != nil {
		for k, v := range sm.m {
			f(k, v)
		}
	}
}

// SetLen returns the number of identifiers in the identifier set at the given key.
func (sm *DefaultIDSetMap) SetLen(key int) int {
	if sm.key == key {
		return sm.value.Len()
	}
	if sm.m != nil {
		if s, ok := sm.m[key]; ok {
			return s.Len()
		}
	}

	return 0
}

// SetContains determines if the given identifier belongs to the set at the given key.
func (sm *DefaultIDSetMap) SetContains(key, id int) bool {
	if sm.key == key {
		return sm.value.Contains(id)
	}
	if sm.m != nil {
		if s, ok := sm.m[key]; ok {
			return s.Contains(id)
		}
	}

	return false
}

// SetEach invokes the given function with each identifier in the set at the given key.
func (sm *DefaultIDSetMap) SetEach(key int, f func(id int)) {
	if sm.key == key {
		sm.value.Each(f)
		return
	}
	if sm.m != nil {
		if s, ok := sm.m[key]; ok {
			s.Each(f)
		}
	}
}

// SetAdd inserts an identifier into the set at the given key.
func (sm *DefaultIDSetMap) SetAdd(key, id int) {
	sm.getOrCreate(key).Add(id)
}

// SetUnion inserts all the identifiers of other into the set a the given key.
func (sm *DefaultIDSetMap) SetUnion(key int, other *IDSet) {
	if other == nil || other.Len() == 0 {
		return
	}

	sm.getOrCreate(key).Union(other)
}

// GetOrCreate will return the set at the given key, or create an empty set if it does not exist.
func (sm *DefaultIDSetMap) getOrCreate(key int) *IDSet {
	if sm.key == key {
		return sm.value
	}
	if sm.m != nil {
		if s, ok := sm.m[key]; ok {
			return s
		}
	}

	if sm.m != nil {
		// Already a map, just add a new key
		s := NewIDSet()
		sm.m[key] = s
		return s
	}

	if sm.key == 0 {
		// Populating a singleton set
		s := NewIDSet()
		sm.key = key
		sm.value = s
		return s
	}

	// Adding to a singleton set. Create a new map and add the
	// new value along with the old singleton value to the it
	s := NewIDSet()
	sm.m = make(map[int]*IDSet, 2)
	sm.m[key] = s
	sm.m[sm.key] = sm.value
	sm.key = 0
	sm.value = nil
	return s
}

// compareDefaultIDSetMaps returns true if the given identifier default identifier set maps
// have equivalent keys and each key contains equivalent elements.
func compareDefaultIDSetMaps(x, y *DefaultIDSetMap) bool {
	if x == nil && y == nil {
		return true
	}

	if x == nil || y == nil {
		return false
	}

	m1 := toMap(x)
	m2 := toMap(y)

	if len(m1) == len(m2) {
		for k, v := range m1 {
			if !compareIDSets(v, m2[k]) {
				return false
			}
		}
	}

	return true
}

// toMap returns a copy of the map backing the default identifier set map. This is called from
// compareDefaultIDSetMaps for testing and should not be used in the hot path.
func toMap(s *DefaultIDSetMap) map[int]*IDSet {
	if s.key != 0 {
		return map[int]*IDSet{
			s.key: s.value,
		}
	}

	if s.m != nil {
		m := map[int]*IDSet{}
		for k, v := range s.m {
			m[k] = v
		}
		return m
	}

	return nil
}
