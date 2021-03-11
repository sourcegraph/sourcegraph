package batches

// ReconcilerOperation is an enum to distinguish between different reconciler operations.
type ReconcilerOperation string

const (
	ReconcilerOperationPush         ReconcilerOperation = "PUSH"
	ReconcilerOperationUpdate       ReconcilerOperation = "UPDATE"
	ReconcilerOperationUndraft      ReconcilerOperation = "UNDRAFT"
	ReconcilerOperationPublish      ReconcilerOperation = "PUBLISH"
	ReconcilerOperationPublishDraft ReconcilerOperation = "PUBLISH_DRAFT"
	ReconcilerOperationSync         ReconcilerOperation = "SYNC"
	ReconcilerOperationImport       ReconcilerOperation = "IMPORT"
	ReconcilerOperationClose        ReconcilerOperation = "CLOSE"
	ReconcilerOperationReopen       ReconcilerOperation = "REOPEN"
	ReconcilerOperationSleep        ReconcilerOperation = "SLEEP"
	ReconcilerOperationDetach       ReconcilerOperation = "DETACH"
)
