import { type FunctionComponent } from 'react'

import { type WorkflowFields } from '../graphql-operations'

export const WorkflowNameWithOwner: FunctionComponent<{ workflow: Pick<WorkflowFields, 'name' | 'owner'> }> = ({
    workflow: {
        name,
        owner: { namespaceName },
    },
}) => (
    <>
        {namespaceName}/<strong>{name}</strong>
    </>
)
