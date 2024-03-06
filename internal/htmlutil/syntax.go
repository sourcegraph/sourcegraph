package htmlutil

import (
	"fmt"
	"sync"

	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters/html"
)

// patchChromaTypes adds "chroma-" prefix to all chroma.StandardTypes except PreWrapper.
//
// To avoid modifying global state during package import, this will only be executed
// when chroma syntax highlighing is used for the first time.
var patchChromaTypes = sync.OnceFunc(func() {
	origTypes := chroma.StandardTypes
	sourcegraphTypes := map[chroma.TokenType]string{}
	for k, v := range origTypes {
		if k == chroma.PreWrapper {
			sourcegraphTypes[k] = v
		} else {
			sourcegraphTypes[k] = fmt.Sprintf("chroma-%s", v)
		}
	}
	chroma.StandardTypes = sourcegraphTypes
})

// SyntaxHighlightingOptions customize chroma code formatter.
func SyntaxHighlightingOptions() []html.Option {
	patchChromaTypes()

	return []html.Option{
		html.WithClasses(true),
		html.WithLineNumbers(false),
	}
}
