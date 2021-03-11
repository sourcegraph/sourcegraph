package pathexistence

import "path/filepath"

// dirWithoutDot returns the directory name of the given path. Unlike filepath.Dir,
// this function will return an empty string (instead of a `.`) to indicate an empty
// directory name.
func dirWithoutDot(path string) string {
	if dir := filepath.Dir(path); dir != "." {
		return dir
	}
	return ""
}
