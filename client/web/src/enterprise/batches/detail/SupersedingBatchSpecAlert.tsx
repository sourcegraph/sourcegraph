import { parseISO } from 'date-fns'
import React from 'react'

import { RouterLink } from '@sourcegraph/wildcard'

import { DismissibleAlert } from '../../../components/DismissibleAlert'
import { Timestamp } from '../../../components/time/Timestamp'
import { SupersedingBatchSpecFields } from '../../../graphql-operations'

export interface SupersedingBatchSpecAlertProps {
    spec: SupersedingBatchSpecFields | null
}

export const SupersedingBatchSpecAlert: React.FunctionComponent<SupersedingBatchSpecAlertProps> = ({ spec }) => {
    if (!spec) {
        return <></>
    }

    const { applyURL, createdAt } = spec

    if (applyURL === null) {
        return null
    }

    return (
        <DismissibleAlert
            className="alert-info"
            partialStorageKey={`superseding-spec-${parseISO(spec.createdAt).getTime()}`}
        >
            <div className="d-flex align-items-center">
                <div className="flex-grow-1">
                    A <RouterLink to={applyURL}>modified batch spec</RouterLink> is ready but not applied since{' '}
                    <Timestamp date={createdAt} noAbout={true} />.
                </div>
            </div>
        </DismissibleAlert>
    )
}
