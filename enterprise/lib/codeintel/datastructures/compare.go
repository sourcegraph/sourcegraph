package datastructures

import "github.com/google/go-cmp/cmp"

var Comparers = []cmp.Option{
	IDSetComparer,
	DefaultIDSetMapComparer,
}

// IDSetComparer is a github.com/google/go-cmp/cmp comparer that can be
// supplied to the cmp.Diff method to determine if two identifier sets
// contain the same set of identifiers.
var IDSetComparer = cmp.Comparer(compareIDSets)

// DefaultIDSetMapComparer is a github.com/google/go-cmp/cmp comparer that can
// be supplied to the cmp.Diff method to determine if two identifier sets contain
// the same set of identifiers.
var DefaultIDSetMapComparer = cmp.Comparer(compareDefaultIDSetMaps)
