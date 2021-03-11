import React from 'react'
import {
    ExternalChangesetFields,
    GitBranchChangesetDescriptionFields,
    ChangesetState,
} from '../../../../graphql-operations'
import { LinkOrSpan } from '../../../../../../shared/src/components/LinkOrSpan'
import ExternalLinkIcon from 'mdi-react/ExternalLinkIcon'
import { ChangesetLabel } from './ChangesetLabel'
import { Link } from '../../../../../../shared/src/components/Link'
import { ChangesetLastSynced } from './ChangesetLastSynced'
import classNames from 'classnames'

export interface ExternalChangesetInfoCellProps {
    node: ExternalChangesetFields
    viewerCanAdminister: boolean
    className?: string
}

export const ExternalChangesetInfoCell: React.FunctionComponent<ExternalChangesetInfoCellProps> = ({
    node,
    viewerCanAdminister,
    className,
}) => (
    <div className={classNames('d-flex flex-column', className)}>
        <div className="m-0">
            <h3 className="m-0 d-block d-md-inline">
                <LinkOrSpan
                    to={node.externalURL?.url ?? undefined}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="mr-2"
                >
                    {(isImporting(node) || importingFailed(node)) && (
                        <>
                            Importing changeset
                            {node.externalID && <> #{node.externalID} </>}
                        </>
                    )}
                    {!isImporting(node) && !importingFailed(node) && (
                        <>
                            {node.title}
                            {node.externalID && <> (#{node.externalID}) </>}
                        </>
                    )}
                    {node.externalURL?.url && (
                        <>
                            {' '}
                            <ExternalLinkIcon size="1rem" />
                        </>
                    )}
                </LinkOrSpan>
            </h3>
            {node.labels.length > 0 && (
                <span className="d-block d-md-inline-block">
                    {node.labels.map(label => (
                        <ChangesetLabel label={label} key={label.text} />
                    ))}
                </span>
            )}
        </div>
        <div>
            <span className="mr-2 d-block d-mdinline-block">
                <Link to={node.repository.url} target="_blank" rel="noopener noreferrer">
                    {node.repository.name}
                </Link>{' '}
                {hasHeadReference(node) && (
                    <div className="d-block d-sm-inline-block">
                        <span className="badge badge-secondary text-monospace">{headReference(node)}</span>
                    </div>
                )}
            </span>
            {![
                ChangesetState.FAILED,
                ChangesetState.PROCESSING,
                ChangesetState.RETRYING,
                ChangesetState.UNPUBLISHED,
            ].includes(node.state) && (
                <ChangesetLastSynced changeset={node} viewerCanAdminister={viewerCanAdminister} />
            )}
        </div>
    </div>
)

function isImporting(node: ExternalChangesetFields): boolean {
    return node.state === ChangesetState.PROCESSING && !hasHeadReference(node)
}

function importingFailed(node: ExternalChangesetFields): boolean {
    return node.state === ChangesetState.FAILED && !hasHeadReference(node)
}

function headReference(node: ExternalChangesetFields): string | undefined {
    if (hasHeadReference(node)) {
        return node.currentSpec?.description.headRef
    }
    return undefined
}

function hasHeadReference(
    node: ExternalChangesetFields
): node is ExternalChangesetFields & {
    currentSpec: ExternalChangesetFields & { description: GitBranchChangesetDescriptionFields }
} {
    return node.currentSpec?.description.__typename === 'GitBranchChangesetDescription'
}
