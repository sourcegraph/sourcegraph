package graphql

import (
	"context"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/shared"
	resolverstubs "github.com/sourcegraph/sourcegraph/internal/codeintel/resolvers"
	"github.com/sourcegraph/sourcegraph/internal/codeintel/shared/resolvers/gitresolvers"
	"github.com/sourcegraph/sourcegraph/lib/errors"
	"github.com/sourcegraph/sourcegraph/lib/pointers"
)

// DefaultDiagnosticsPageSize is the diagnostic result page size when no limit is supplied.
const DefaultDiagnosticsPageSize = 100

// Diagnostics returns the diagnostics for documents with the given path prefix.
func (r *gitBlobLSIFDataResolver) Diagnostics(ctx context.Context, args *resolverstubs.LSIFDiagnosticsArgs) (_ resolverstubs.DiagnosticConnectionResolver, err error) {
	limit := int(pointers.Deref(args.First, DefaultDiagnosticsPageSize))
	if limit <= 0 {
		return nil, ErrIllegalLimit
	}

	requestArgs := codenav.PositionalRequestArgs{
		RequestArgs: codenav.RequestArgs{
			RepositoryID: r.requestState.RepositoryID,
			Commit:       r.requestState.Commit,
			Limit:        limit,
		},
		Path: r.requestState.Path,
	}
	ctx, _, endObservation := observeResolver(ctx, &err, r.operations.diagnostics, time.Second, getObservationArgs(requestArgs))
	defer endObservation()

	diagnostics, totalCount, err := r.codeNavSvc.GetDiagnostics(ctx, requestArgs, r.requestState)
	if err != nil {
		return nil, errors.Wrap(err, "codeNavSvc.GetDiagnostics")
	}

	resolvers := make([]resolverstubs.DiagnosticResolver, 0, len(diagnostics))
	for i := range diagnostics {
		resolvers = append(resolvers, newDiagnosticResolver(diagnostics[i], r.locationResolver))
	}

	return resolverstubs.NewTotalCountConnectionResolver(resolvers, 0, int32(totalCount)), nil
}

//
//

type diagnosticResolver struct {
	diagnostic       codenav.DiagnosticAtUpload
	locationResolver *gitresolvers.CachedLocationResolver
}

func newDiagnosticResolver(diagnostic codenav.DiagnosticAtUpload, locationResolver *gitresolvers.CachedLocationResolver) resolverstubs.DiagnosticResolver {
	return &diagnosticResolver{
		diagnostic:       diagnostic,
		locationResolver: locationResolver,
	}
}

func (r *diagnosticResolver) Severity() (*string, error) { return toSeverity(r.diagnostic.Severity) }
func (r *diagnosticResolver) Code() (*string, error) {
	return pointers.NonZeroPtr(r.diagnostic.Code), nil
}

func (r *diagnosticResolver) Source() (*string, error) {
	return pointers.NonZeroPtr(r.diagnostic.Source), nil
}

func (r *diagnosticResolver) Message() (*string, error) {
	return pointers.NonZeroPtr(r.diagnostic.Message), nil
}

func (r *diagnosticResolver) Location(ctx context.Context) (resolverstubs.LocationResolver, error) {
	return resolveLocation(
		ctx,
		r.locationResolver,
		shared.UploadLocation{
			Upload:       r.diagnostic.Upload,
			Path:         r.diagnostic.Path,
			TargetCommit: r.diagnostic.AdjustedCommit,
			TargetRange:  r.diagnostic.AdjustedRange,
		},
	)
}

var severities = map[int]string{
	1: "ERROR",
	2: "WARNING",
	3: "INFORMATION",
	4: "HINT",
}

func toSeverity(val int) (*string, error) {
	severity, ok := severities[val]
	if !ok {
		return nil, errors.Errorf("unknown diagnostic severity %d", val)
	}

	return &severity, nil
}
