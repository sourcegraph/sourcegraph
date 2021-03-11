package batches

import (
	"time"

	"github.com/sourcegraph/batch-change-utils/env"
	"github.com/sourcegraph/batch-change-utils/overridable"
	"github.com/sourcegraph/batch-change-utils/yaml"

	"github.com/sourcegraph/sourcegraph/schema"
)

func NewBatchSpecFromRaw(rawSpec string) (*BatchSpec, error) {
	c := &BatchSpec{RawSpec: rawSpec}

	return c, c.UnmarshalValidate()
}

type BatchSpec struct {
	ID     int64
	RandID string

	RawSpec string
	Spec    BatchSpecFields

	NamespaceUserID int32
	NamespaceOrgID  int32

	UserID int32

	CreatedAt time.Time
	UpdatedAt time.Time
}

// Clone returns a clone of a BatchSpec.
func (cs *BatchSpec) Clone() *BatchSpec {
	cc := *cs
	return &cc
}

// UnmarshalValidate unmarshals the RawSpec into Spec and validates it against
// the BatchSpec schema and does additional semantic validation.
func (cs *BatchSpec) UnmarshalValidate() error {
	return yaml.UnmarshalValidate(schema.BatchSpecSchemaJSON, []byte(cs.RawSpec), &cs.Spec)
}

// BatchSpecTTL specifies the TTL of BatchSpecs that haven't been applied
// yet. It's set to 1 week.
const BatchSpecTTL = 7 * 24 * time.Hour

// ExpiresAt returns the time when the BatchSpec will be deleted if not
// applied.
func (cs *BatchSpec) ExpiresAt() time.Time {
	return cs.CreatedAt.Add(BatchSpecTTL)
}

type BatchSpecFields struct {
	Name              string                       `json:"name" yaml:"name"`
	Description       string                       `json:"description,omitempty" yaml:"description,omitempty"`
	On                []BatchSpecOn                `json:"on,omitempty" yaml:"on,omitempty"`
	Steps             []BatchSpecStep              `json:"steps,omitempty" yaml:"steps,omitempty"`
	ImportChangeset   []BatchChangeImportChangeset `json:"importChangesets,omitempty" yaml:"importChangesets,omitempty"`
	ChangesetTemplate ChangesetTemplate            `json:"changesetTemplate,omitempty" yaml:"changesetTemplate,omitempty"`
}

type BatchSpecOn struct {
	RepositoriesMatchingQuery string `json:"repositoriesMatchingQuery,omitempty" yaml:"repositoriesMatchingQuery,omitempty"`
	Repository                string `json:"repository,omitempty" yaml:"repository,omitempty"`
}

type BatchSpecStep struct {
	Run       string          `json:"run" yaml:"run"`
	Container string          `json:"container" yaml:"container"`
	Env       env.Environment `json:"env,omitempty" yaml:"env,omitempty"`
}

type BatchChangeImportChangeset struct {
	Repository  string        `json:"repository" yaml:"repository"`
	ExternalIDs []interface{} `json:"externalIDs" yaml:"externalIDs"`
}

type ChangesetTemplate struct {
	Title     string                   `json:"title,omitempty" yaml:"title,omitempty"`
	Body      string                   `json:"body,omitempty" yaml:"body,omitempty"`
	Branch    string                   `json:"branch,omitempty" yaml:"branch,omitempty"`
	Commit    CommitTemplate           `json:"commit,omitempty" yaml:"commit,omitempty"`
	Published overridable.BoolOrString `json:"published,omitempty" yaml:"published,omitempty"`
}

type CommitTemplate struct {
	Message string `json:"message,omitempty" yaml:"message,omitempty"`
}
