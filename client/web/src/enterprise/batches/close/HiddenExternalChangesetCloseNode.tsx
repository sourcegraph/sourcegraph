import React from 'react'
import { ChangesetCloseActionKept } from './ChangesetCloseAction'
import {
    HiddenExternalChangesetInfoCellProps,
    HiddenExternalChangesetInfoCell,
} from '../detail/changesets/HiddenExternalChangesetInfoCell'

export interface HiddenExternalChangesetCloseNodeProps {
    node: HiddenExternalChangesetInfoCellProps['node']
}

export const HiddenExternalChangesetCloseNode: React.FunctionComponent<HiddenExternalChangesetCloseNodeProps> = ({
    node,
}) => (
    <>
        <span className="d-none d-sm-block" />
        {/* Hidden changesets are always untouched, so the action will always be "kept". */}
        <ChangesetCloseActionKept className="hidden-external-changeset-close-node__action" />
        <HiddenExternalChangesetInfoCell node={node} className="hidden-external-changeset-close-node__information" />
        <span className="d-none d-sm-block" />
        <span className="d-none d-sm-block" />
        <span className="d-none d-sm-block" />
    </>
)
