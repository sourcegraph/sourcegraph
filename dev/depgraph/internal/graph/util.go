package graph

import "strings"

// relative trims the given root from the given path.
func relative(path, root string) string {
	return strings.TrimPrefix(strings.TrimPrefix(path, root), "/")
}
