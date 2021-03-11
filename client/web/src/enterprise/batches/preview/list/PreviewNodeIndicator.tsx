import React from 'react'
import { ChangesetApplyPreviewFields } from '../../../../graphql-operations'
import { ChangesetAddedIcon, ChangesetModifiedIcon, ChangesetRemovedIcon } from '../icons'

const containerClassName =
    'preview-node-indicator__container d-none d-sm-flex flex-column align-items-center justify-content-center align-self-stretch'

export interface PreviewNodeIndicatorProps {
    node: ChangesetApplyPreviewFields
}

export const PreviewNodeIndicator: React.FunctionComponent<PreviewNodeIndicatorProps> = ({ node }) => {
    switch (node.targets.__typename) {
        case 'HiddenApplyPreviewTargetsAttach':
        case 'VisibleApplyPreviewTargetsAttach':
            return (
                <div className={containerClassName}>
                    <span className="preview-node-indicator__attach-bar">&nbsp;</span>
                    <span className="preview-node-indicator__attach-icon d-flex justify-content-center align-items-center">
                        <ChangesetAddedIcon />
                    </span>
                    <span className="preview-node-indicator__attach-bar">&nbsp;</span>
                </div>
            )
        case 'HiddenApplyPreviewTargetsUpdate':
        case 'VisibleApplyPreviewTargetsUpdate':
            if (node.__typename === 'HiddenChangesetApplyPreview' || node.operations.length === 0) {
                // If no operations, no update :P
                return <div />
            }
            return (
                <div className={containerClassName}>
                    <span className="preview-node-indicator__update-bar">&nbsp;</span>
                    <span className="preview-node-indicator__update-icon d-flex justify-content-center align-items-center">
                        <ChangesetModifiedIcon />
                    </span>
                    <span className="preview-node-indicator__update-bar">&nbsp;</span>
                </div>
            )
        case 'HiddenApplyPreviewTargetsDetach':
        case 'VisibleApplyPreviewTargetsDetach':
            return (
                <div className={containerClassName}>
                    <span className="preview-node-indicator__detach-bar">&nbsp;</span>
                    <span className="preview-node-indicator__detach-icon d-flex justify-content-center align-items-center">
                        <ChangesetRemovedIcon />
                    </span>
                    <span className="preview-node-indicator__detach-bar">&nbsp;</span>
                </div>
            )
    }
}
