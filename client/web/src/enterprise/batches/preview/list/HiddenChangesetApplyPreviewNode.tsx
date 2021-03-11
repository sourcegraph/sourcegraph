import React from 'react'
import { ChangesetSpecType, HiddenChangesetApplyPreviewFields } from '../../../../graphql-operations'
import InfoCircleOutlineIcon from 'mdi-react/InfoCircleOutlineIcon'
import { PreviewActions } from './PreviewActions'
import { ChangesetStatusCell } from '../../detail/changesets/ChangesetStatusCell'
import { ChangesetState } from '../../../../../../shared/src/graphql-operations'
import { PreviewNodeIndicator } from './PreviewNodeIndicator'

export interface HiddenChangesetApplyPreviewNodeProps {
    node: HiddenChangesetApplyPreviewFields
}

export const HiddenChangesetApplyPreviewNode: React.FunctionComponent<HiddenChangesetApplyPreviewNodeProps> = ({
    node,
}) => (
    <>
        <span className="hidden-changeset-apply-preview-node__list-cell d-none d-sm-block" />
        <HiddenChangesetApplyPreviewNodeStatusCell
            node={node}
            className="hidden-changeset-apply-preview-node__list-cell d-block d-sm-flex hidden-changeset-apply-preview-node__current-state"
        />
        <PreviewNodeIndicator node={node} />
        <PreviewActions
            node={node}
            className="hidden-changeset-apply-preview-node__list-cell hidden-changeset-apply-preview-node__action"
        />
        <div className="hidden-changeset-apply-preview-node__list-cell d-flex flex-column hidden-changeset-apply-preview-node__information">
            <h3 className="text-muted">
                {node.targets.__typename === 'HiddenApplyPreviewTargetsAttach' ||
                node.targets.__typename === 'HiddenApplyPreviewTargetsUpdate' ? (
                    <>
                        {node.targets.changesetSpec.type === ChangesetSpecType.EXISTING && (
                            <>Import changeset from a private repository</>
                        )}
                        {node.targets.changesetSpec.type === ChangesetSpecType.BRANCH && (
                            <>Create changeset in a private repository</>
                        )}
                    </>
                ) : (
                    <>Detach changeset in a private repository</>
                )}
            </h3>
            <span className="text-danger">
                No action will be taken on apply.{' '}
                <InfoCircleOutlineIcon
                    className="icon-inline"
                    data-tooltip="You have no permissions to access this repository."
                />
            </span>
        </div>
        <span />
        <span />
    </>
)

const HiddenChangesetApplyPreviewNodeStatusCell: React.FunctionComponent<
    HiddenChangesetApplyPreviewNodeProps & { className?: string }
> = ({ node, className }) => {
    if (node.targets.__typename === 'HiddenApplyPreviewTargetsAttach') {
        return <ChangesetStatusCell state={ChangesetState.UNPUBLISHED} className={className} />
    }
    return <ChangesetStatusCell state={node.targets.changeset.state} className={className} />
}
