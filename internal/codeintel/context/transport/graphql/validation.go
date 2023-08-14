package graphql

import (
	"github.com/sourcegraph/sourcegraph/lib/errors"

	resolverstubs "github.com/sourcegraph/sourcegraph/internal/codeintel/resolvers"
)

func validateGetPreciseContextInput(input *resolverstubs.GetPreciseContextInput) error {
	if input.Input.ActiveFile == "" {
		return errors.New("active file must be set")
	}
	if input.Input.ActiveFileContent == "" {
		return errors.New("active file content must be set")
	}
	if input.Input.ClosestRemoteCommitSHA == "" {
		return errors.New("closest remote commit ID must be set")
	}
	if input.Input.RepositoryName == "" {
		return errors.New("repository name must be set")
	}

	return nil
}
