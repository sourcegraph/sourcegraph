package semantic

// HashKey hashes a string identifier into the range [0, maxIndex)`. The
// hash algorithm here is similar ot the one used in Java's String.hashCode.
// This implementation is identical to the TypeScript version used before
// the port to Go so that we can continue to read old conversions without
// a migration.
func HashKey(id ID, maxIndex int) int {
	hash := int32(0)
	for _, c := range string(id) {
		hash = (hash << 5) - hash + c
	}

	if hash < 0 {
		hash = -hash
	}

	return int(hash % int32(maxIndex))
}
