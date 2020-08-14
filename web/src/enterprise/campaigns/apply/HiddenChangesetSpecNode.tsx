import React from 'react'
import { ChangesetSpecFields } from '../../../graphql-operations'
import { ChangesetSpecType } from '../../../../../shared/src/graphql/schema'
import InfoCircleOutlineIcon from 'mdi-react/InfoCircleOutlineIcon'
import { ChangesetSpecAction } from './ChangesetSpecAction'

export interface HiddenChangesetSpecNodeProps {
    node: ChangesetSpecFields & { __typename: 'HiddenChangesetSpec' }
}

export const HiddenChangesetSpecNode: React.FunctionComponent<HiddenChangesetSpecNodeProps> = ({ node }) => (
    <>
        <span />
        <ChangesetSpecAction spec={node} />
        <div className="d-flex flex-column">
            <h3 className="text-muted">
                {node.type === ChangesetSpecType.EXISTING && <>Import changeset from a private repository</>}
                {node.type === ChangesetSpecType.BRANCH && <>Create changeset in a private repository</>}
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
    </>
)
