package maps

// Merge merges the source map into the destination map, overwriting existing values if necessary.
func Merge[K comparable, V any](dest, src map[K]V) map[K]V {
	if dest == nil {
		if src == nil {
			return make(map[K]V)
		}
		dest = make(map[K]V, len(src))
	}

	for k, v := range src {
		dest[k] = v
	}

	return dest
}

// MergePreservingExistingKeys merges the source map into the destination map,
// without overwriting any existing keys in the destination.
func MergePreservingExistingKeys[K comparable, V any](dest, src map[K]V) map[K]V {
	if dest == nil {
		if src == nil {
			return make(map[K]V)
		}
		dest = make(map[K]V, len(src))
	}

	for k, v := range src {
		if _, exists := dest[k]; !exists {
			dest[k] = v
		}
	}

	return dest
}
