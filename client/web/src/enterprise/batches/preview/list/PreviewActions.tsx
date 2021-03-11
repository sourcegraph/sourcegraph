import React from 'react'
import { ChangesetApplyPreviewFields, ChangesetSpecOperation } from '../../../../graphql-operations'
import BlankCircleIcon from 'mdi-react/CheckboxBlankCircleOutlineIcon'
import ImportIcon from 'mdi-react/ImportIcon'
import UploadIcon from 'mdi-react/UploadIcon'
import TrashIcon from 'mdi-react/TrashIcon'
import UploadNetworkIcon from 'mdi-react/UploadNetworkIcon'
import SourceBranchRefreshIcon from 'mdi-react/SourceBranchRefreshIcon'
import SourceBranchSyncIcon from 'mdi-react/SourceBranchSyncIcon'
import SourceBranchCheckIcon from 'mdi-react/SourceBranchCheckIcon'
import BeakerQuestionIcon from 'mdi-react/BeakerQuestionIcon'
import classNames from 'classnames'
import CloseCircleOutlineIcon from 'mdi-react/CloseCircleOutlineIcon'

export interface PreviewActionsProps {
    node: ChangesetApplyPreviewFields
    className?: string
}

export const PreviewActions: React.FunctionComponent<PreviewActionsProps> = ({ node, className }) => (
    <div className={classNames('d-flex flex-column align-items-left justify-content-center', className)}>
        <PreviewActionsContent node={node} />
    </div>
)

const PreviewActionsContent: React.FunctionComponent<Pick<PreviewActionsProps, 'node'>> = ({ node }) => {
    if (node.__typename === 'HiddenChangesetApplyPreview') {
        return <PreviewActionNoAction reason={NoActionReasonStrings[NoActionReason.NO_ACCESS]} />
    }
    if (node.operations.length === 0) {
        return <PreviewActionNoAction />
    }
    return (
        <>
            {node.operations.map((operation, index) => (
                <PreviewAction
                    operation={operation}
                    operations={node.operations}
                    key={operation}
                    className={classNames(index !== node.operations.length - 1 && 'mb-1')}
                />
            ))}
        </>
    )
}

interface PreviewActionProps {
    operation: ChangesetSpecOperation
    operations: ChangesetSpecOperation[]
    className?: string
}

const PreviewAction: React.FunctionComponent<PreviewActionProps> = ({ operation, operations, className }) => {
    switch (operation) {
        case ChangesetSpecOperation.IMPORT:
            return <PreviewActionImport className={className} />
        case ChangesetSpecOperation.PUBLISH:
            return <PreviewActionPublish className={className} />
        case ChangesetSpecOperation.PUBLISH_DRAFT:
            return <PreviewActionPublishDraft className={className} />
        case ChangesetSpecOperation.CLOSE:
            return <PreviewActionClose className={className} />
        case ChangesetSpecOperation.REOPEN:
            return <PreviewActionReopen className={className} />
        case ChangesetSpecOperation.UNDRAFT:
            return <PreviewActionUndraft className={className} />
        case ChangesetSpecOperation.UPDATE:
            return <PreviewActionUpdate className={className} />
        case ChangesetSpecOperation.PUSH:
            return <PreviewActionPush className={className} />
        case ChangesetSpecOperation.DETACH:
            return <PreviewActionDetach className={className} />
        case ChangesetSpecOperation.SYNC:
        case ChangesetSpecOperation.SLEEP:
            // We don't want to expose these states.
            return null
        default:
            return <PreviewActionUnknown operations={operations.join(' => ')} className={className} />
    }
}

const iconClassNames = 'm-0 text-nowrap'

export const PreviewActionPublish: React.FunctionComponent<{ label?: string; className?: string }> = ({
    label = 'Publish',
    className,
}) => (
    <div className={classNames(className, iconClassNames)}>
        <UploadIcon className="icon-inline mr-1" data-tooltip="This changeset will be published to its code host" />
        <span>{label}</span>
    </div>
)
export const PreviewActionPublishDraft: React.FunctionComponent<{ label?: string; className?: string }> = ({
    label = 'Publish draft',
    className,
}) => (
    <div className={classNames(className, iconClassNames)}>
        <UploadIcon
            className="text-muted mr-1 icon-inline"
            data-tooltip="This changeset will be published as a draft to its code host"
        />
        <span>{label}</span>
    </div>
)
export const PreviewActionImport: React.FunctionComponent<{ label?: string; className?: string }> = ({
    label = 'Import',
    className,
}) => (
    <div className={classNames(className, iconClassNames)}>
        <ImportIcon
            className="icon-inline mr-1"
            data-tooltip="This changeset will be imported and tracked in this batch change"
        />
        <span>{label}</span>
    </div>
)
export const PreviewActionClose: React.FunctionComponent<{ label?: string; className?: string }> = ({
    label = 'Close',
    className,
}) => (
    <div className={classNames(className, iconClassNames)}>
        <CloseCircleOutlineIcon
            className="text-danger mr-1 icon-inline"
            data-tooltip="This changeset will be closed on the code host"
        />
        <span>{label}</span>
    </div>
)
export const PreviewActionDetach: React.FunctionComponent<{ label?: string; className?: string }> = ({
    label = 'Detach',
    className,
}) => (
    <div className={classNames(className, iconClassNames)}>
        <TrashIcon
            className="text-danger mr-1 icon-inline"
            data-tooltip="This changeset will be removed from the batch change"
        />
        <span>{label}</span>
    </div>
)
export const PreviewActionReopen: React.FunctionComponent<{ label?: string; className?: string }> = ({
    label = 'Reopen',
    className,
}) => (
    <div className={classNames(className, iconClassNames)}>
        <SourceBranchRefreshIcon
            className="icon-inline text-success mr-1"
            data-tooltip="This changeset will be reopened on the code host"
        />
        <span>{label}</span>
    </div>
)
export const PreviewActionUndraft: React.FunctionComponent<{ label?: string; className?: string }> = ({
    label = 'Undraft',
    className,
}) => (
    <div className={classNames(className, iconClassNames)}>
        <SourceBranchCheckIcon
            className="icon-inline text-success mr-1"
            data-tooltip="This changeset will be marked as ready for review on the code host"
        />
        <span>{label}</span>
    </div>
)
export const PreviewActionUpdate: React.FunctionComponent<{ label?: string; className?: string }> = ({
    label = 'Update',
    className,
}) => (
    <div className={classNames(className, iconClassNames)}>
        <SourceBranchSyncIcon
            className="icon-inline mr-1"
            data-tooltip="This changeset will be updated on the code host"
        />
        <span>{label}</span>
    </div>
)
export const PreviewActionPush: React.FunctionComponent<{ label?: string; className?: string }> = ({
    label = 'Push',
    className,
}) => (
    <div className={classNames(className, iconClassNames)}>
        <UploadNetworkIcon className="icon-inline mr-1" data-tooltip="A new commit will be pushed to the code host" />
        <span>{label}</span>
    </div>
)
export const PreviewActionUnknown: React.FunctionComponent<{ className?: string; operations: string }> = ({
    operations,
    className,
}) => (
    <div className={classNames(className, iconClassNames)}>
        <BeakerQuestionIcon
            className="icon-inline mr-1"
            data-tooltip={`The operation ${operations} can't yet be displayed.`}
        />
        <span>Unknown</span>
    </div>
)
export enum NoActionReason {
    NO_ACCESS = 'no-access',
}
export const NoActionReasonStrings: Record<NoActionReason, string> = {
    [NoActionReason.NO_ACCESS]: "You don't have access to the repository this changeset spec targets.",
}
export const PreviewActionNoAction: React.FunctionComponent<{ className?: string; reason?: string }> = ({
    className,
    reason,
}) => (
    <div className={classNames(className, iconClassNames, 'text-muted')}>
        <BlankCircleIcon className="icon-inline mr-1" data-tooltip={reason} />
        <span>No action</span>
    </div>
)
