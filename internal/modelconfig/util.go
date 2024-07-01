package modelconfig

import "github.com/sourcegraph/sourcegraph/internal/modelconfig/types"

// RedactServerSideConfig modifies the provided ModelConfiguration data in-place to remove
// all server-side configuration data.
func RedactServerSideConfig(doc *types.ModelConfiguration) {
	for i := range doc.Providers {
		doc.Providers[i].ServerSideConfig = nil
	}
	for i := range doc.Models {
		doc.Models[i].ServerSideConfig = nil
	}
}
