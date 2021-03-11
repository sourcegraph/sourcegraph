package batches

import "github.com/sourcegraph/sourcegraph/internal/extsvc"

// CodeHost represents one configured external code host available on this Sourcegraph instance.
type CodeHost struct {
	ExternalServiceType string
	ExternalServiceID   string
	RequiresSSH         bool
}

// IsSupported returns true, when this code host is supported by
// the batch changes feature.
func (c *CodeHost) IsSupported() bool {
	return IsKindSupported(extsvc.TypeToKind(c.ExternalServiceType))
}
