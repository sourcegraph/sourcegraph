package batches

import (
	"time"

	"github.com/sourcegraph/sourcegraph/internal/api"
	"github.com/sourcegraph/sourcegraph/internal/extsvc"
)

type CodehostCapability string

const (
	CodehostCapabilityLabels          CodehostCapability = "Labels"
	CodehostCapabilityDraftChangesets CodehostCapability = "DraftChangesets"
)

type CodehostCapabilities map[CodehostCapability]bool

// SupportedExternalServices are the external service types currently supported
// by the batch changes feature. Repos that are associated with external services
// whose type is not in this list will simply be filtered out from the search
// results.
var SupportedExternalServices = map[string]CodehostCapabilities{
	extsvc.TypeGitHub:          {CodehostCapabilityLabels: true, CodehostCapabilityDraftChangesets: true},
	extsvc.TypeBitbucketServer: {},
	extsvc.TypeGitLab:          {CodehostCapabilityLabels: true, CodehostCapabilityDraftChangesets: true},
}

// IsRepoSupported returns whether the given ExternalRepoSpec is supported by
// the batch changes feature, based on the external service type.
func IsRepoSupported(spec *api.ExternalRepoSpec) bool {
	_, ok := SupportedExternalServices[spec.ServiceType]
	return ok
}

// IsKindSupported returns whether the given extsvc Kind is supported by
// batch changes.
func IsKindSupported(extSvcKind string) bool {
	_, ok := SupportedExternalServices[extsvc.KindToType(extSvcKind)]
	return ok
}

func ExternalServiceSupports(extSvcType string, capability CodehostCapability) bool {
	if es, ok := SupportedExternalServices[extSvcType]; ok {
		val, ok := es[capability]
		return ok && val
	}
	return false
}

// Keyer represents items that return a unique key
type Keyer interface {
	Key() string
}

func unixMilliToTime(ms int64) time.Time {
	return time.Unix(0, ms*int64(time.Millisecond))
}
