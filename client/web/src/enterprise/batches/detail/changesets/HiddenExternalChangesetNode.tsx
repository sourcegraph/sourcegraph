import React from 'react'
import { HiddenExternalChangesetFields } from '../../../../graphql-operations'
import { ChangesetStatusCell } from './ChangesetStatusCell'
import { HiddenExternalChangesetInfoCell } from './HiddenExternalChangesetInfoCell'

export interface HiddenExternalChangesetNodeProps {
    node: Pick<HiddenExternalChangesetFields, 'id' | 'nextSyncAt' | 'updatedAt' | 'state' | '__typename'>
}

export const HiddenExternalChangesetNode: React.FunctionComponent<HiddenExternalChangesetNodeProps> = ({ node }) => (
    <>
        <span className="d-none d-sm-block" />
        <ChangesetStatusCell
            state={node.state}
            className="p-2 hidden-external-changeset-node__status text-muted d-block d-sm-flex"
        />
        <HiddenExternalChangesetInfoCell node={node} className="p-2 hidden-external-changeset-node__information" />
        <span className="d-none d-sm-block" />
        <span className="d-none d-sm-block" />
        <span className="d-none d-sm-block" />
    </>
)
