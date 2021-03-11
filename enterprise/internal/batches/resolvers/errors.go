package resolvers

import "fmt"

type ErrIDIsZero struct{}

func (e ErrIDIsZero) Error() string {
	return "invalid node id"
}

func (e ErrIDIsZero) Extensions() map[string]interface{} {
	return map[string]interface{}{"code": "ErrIDIsZero"}
}

type ErrBatchChangesDisabled struct{}

func (e ErrBatchChangesDisabled) Error() string {
	return "batch changes are disabled. Set 'batchChanges.enabled' in the site configuration to enable the feature."
}

func (e ErrBatchChangesDisabled) Extensions() map[string]interface{} {
	return map[string]interface{}{"code": "ErrBatchChangesDisabled"}
}

type ErrBatchChangesDisabledForUser struct{}

func (e ErrBatchChangesDisabledForUser) Error() string {
	return "batch changes are disabled for non-site-admin users"
}

func (e ErrBatchChangesDisabledForUser) Extensions() map[string]interface{} {
	return map[string]interface{}{"code": "ErrBatchChangesDisabledForUser"}
}

// ErrBatchChangesUnlicensed wraps an underlying licensing.featureNotActivatedError
// to add an error code.
type ErrBatchChangesUnlicensed struct{ error }

func (e ErrBatchChangesUnlicensed) Extensions() map[string]interface{} {
	return map[string]interface{}{"code": "ErrBatchChangesUnlicensed"}
}

type ErrBatchChangesDotcom struct{}

func (e ErrBatchChangesDotcom) Error() string {
	return "access to batch changes on Sourcegraph.com is currently not available"
}

func (e ErrBatchChangesDotcom) Extensions() map[string]interface{} {
	return map[string]interface{}{"code": "ErrBatchChangesDotCom"}
}

type ErrEnsureBatchChangeFailed struct{}

func (e ErrEnsureBatchChangeFailed) Error() string {
	return "a batch change in the given namespace and with the given name exists but does not match the given ID"
}

func (e ErrEnsureBatchChangeFailed) Extensions() map[string]interface{} {
	return map[string]interface{}{"code": "ErrEnsureBatchChangeFailed"}
}

type ErrApplyClosedBatchChange struct{}

func (e ErrApplyClosedBatchChange) Error() string {
	return "existing batch change matched by batch spec is closed"
}

func (e ErrApplyClosedBatchChange) Extensions() map[string]interface{} {
	return map[string]interface{}{"code": "ErrApplyClosedBatchChange"}
}

type ErrMatchingBatchChangeExists struct{}

func (e ErrMatchingBatchChangeExists) Error() string {
	return "a batch change matching the given batch spec already exists"
}

func (e ErrMatchingBatchChangeExists) Extensions() map[string]interface{} {
	return map[string]interface{}{"code": "ErrMatchingBatchChangeExists"}
}

type ErrDuplicateCredential struct{}

func (e ErrDuplicateCredential) Error() string {
	return "a credential for this code host already exists"
}

func (e ErrDuplicateCredential) Extensions() map[string]interface{} {
	return map[string]interface{}{"code": "ErrDuplicateCredential"}
}

type ErrVerifyCredentialFailed struct {
	SourceErr error
}

func (e ErrVerifyCredentialFailed) Error() string {
	return fmt.Sprintf("Failed to verify the credential:\n```\n%s\n```\n", e.SourceErr)
}

func (e ErrVerifyCredentialFailed) Extensions() map[string]interface{} {
	return map[string]interface{}{"code": "ErrVerifyCredentialFailed"}
}
