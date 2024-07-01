package types

import "time"

// Workflow represents a workflow.
type Workflow struct {
	ID           int32     // the globally unique DB ID
	Name         string    // the workflow name, unique among the owner's workflows
	Description  string    // a human-readable description of the workflow
	TemplateText string    // the prompt template text
	Draft        bool      // whether the workflow is a draft
	Owner        Namespace // the workflow's owner

	CreatedAt     time.Time // when this workflow was created
	CreatedByUser *int32    // the user that created this workflow
	UpdatedAt     time.Time // when this workflow was last updated
	UpdatedByUser *int32    // the user that last updated this workflow
}
