package types

import (
	codenavshared "github.com/sourcegraph/sourcegraph/internal/codeintel/codenav/shared"
)

type PreciseContext struct {
	Symbol            PreciseSymbolReference
	DefinitionSnippet string                       `json:"-"` // json tags temporary
	Location          codenavshared.UploadLocation `json:"-"` // json tags temporary

	// TODO: redundant with location
	RepositoryName string `json:"-"` // json tags temporary
	Filepath       string `json:"-"` // json tags temporary
}

type PreciseSymbolReference struct {
	ScipName         string
	DescriptorSuffix string
	FuzzyName        *string
}
