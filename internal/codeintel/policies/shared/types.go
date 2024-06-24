package shared

import "time"

type ConfigurationPolicy struct {
	ID                        int
	RepositoryID              *int
	RepositoryPatterns        *[]string
	Name                      string
	Type                      GitObjectType
	Pattern                   string
	Protected                 bool
	RetentionEnabled          bool
	RetentionDuration         *time.Duration
	RetainIntermediateCommits bool
	IndexingEnabled           bool
	IndexCommitMaxAge         *time.Duration
	IndexIntermediateCommits  bool
	EmbeddingEnabled          bool
}

type GitObjectType string

const (
	GitObjectTypeCommit GitObjectType = "GIT_COMMIT"
	GitObjectTypeTag    GitObjectType = "GIT_TAG"
	GitObjectTypeTree   GitObjectType = "GIT_TREE"
)

type RetentionPolicyMatchCandidate struct {
	*ConfigurationPolicy
	Matched           bool
	ProtectingCommits []string
}

type GetConfigurationPoliciesOptions struct {
	// RepositoryID indicates that only configuration policies that apply to the
	// specified repository (directly or via pattern) should be returned. This value
	// has no effect when equal to zero.
	RepositoryID int

	// Term is a string to search within the configuration title.
	Term string

	// If supplied, filter the policies by their protected flag.
	Protected *bool

	// ForIndexing indicates that configuration policies with data retention enabled
	// should be returned (or filtered).
	ForDataRetention *bool

	// ForPreciseIndexing indicates that configuration policies with precise indexing enabled should
	// be returned (or filtered).
	ForPreciseIndexing *bool

	// ForSyntacticIndexing indicates that configuration policies with syntactic indexing enabled should
	// be returned (or filtered).
	ForSyntacticIndexing *bool

	// Limit indicates the number of results to take from the result set.
	Limit int

	// Offset indicates the number of results to skip in the result set.
	Offset int
}
