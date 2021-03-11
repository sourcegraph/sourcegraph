import React from 'react'
import { Markdown } from '../../../../../shared/src/components/Markdown'
import { renderMarkdown } from '../../../../../shared/src/util/markdown'
import { Link } from '../../../../../shared/src/components/Link'
import classNames from 'classnames'
import * as H from 'history'
import { Timestamp } from '../../../components/time/Timestamp'
import { ListBatchChange } from '../../../graphql-operations'
import {
    ChangesetStatusOpen,
    ChangesetStatusClosed,
    ChangesetStatusMerged,
} from '../detail/changesets/ChangesetStatusCell'

export interface BatchChangeNodeProps {
    node: ListBatchChange
    /** Used for testing purposes. Sets the current date */
    now?: () => Date
    history: H.History
    displayNamespace: boolean
}

/**
 * An item in the list of batch changes.
 */
export const BatchChangeNode: React.FunctionComponent<BatchChangeNodeProps> = ({
    node,
    history,
    now = () => new Date(),
    displayNamespace,
}) => (
    <>
        <span className="batch-change-node__separator" />
        {!node.closedAt && <span className="batch-change-node__badge badge badge-success text-uppercase">Open</span>}
        {node.closedAt && <span className="batch-change-node__badge badge badge-danger text-uppercase">Closed</span>}
        <div className="batch-change-node__content">
            <div className="m-0 d-md-flex d-block align-items-baseline">
                <h3 className="m-0 d-md-inline-block d-block batch-change-node__title">
                    {displayNamespace && (
                        <div className="d-md-inline-block d-block">
                            <Link
                                className="text-muted test-batches-namespace-link"
                                to={`${node.namespace.url}/batch-changes`}
                            >
                                {node.namespace.namespaceName}
                            </Link>
                            <span className="text-muted d-inline-block mx-1">/</span>
                        </div>
                    )}
                    <Link className="test-batches-link mr-2" to={node.url}>
                        {node.name}
                    </Link>
                </h3>
                <small className="text-muted d-sm-block">
                    created <Timestamp date={node.createdAt} now={now} />
                </small>
            </div>
            <Markdown
                className={classNames('text-truncate d-none d-md-block', !node.description && 'text-muted font-italic')}
                dangerousInnerHTML={
                    node.description ? renderMarkdown(node.description, { plainText: true }) : 'No description'
                }
                history={history}
            />
        </div>
        <ChangesetStatusOpen
            className="d-block d-sm-flex"
            label={<span className="text-muted">{node.changesetsStats.open} open</span>}
        />
        <ChangesetStatusClosed
            className="d-block d-sm-flex text-center"
            label={<span className="text-muted">{node.changesetsStats.closed} closed</span>}
        />
        <ChangesetStatusMerged
            className="d-block d-sm-flex"
            label={<span className="text-muted">{node.changesetsStats.merged} merged</span>}
        />
    </>
)
