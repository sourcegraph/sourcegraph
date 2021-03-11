import React from 'react'
import { Timestamp } from '../../../components/time/Timestamp'
import CreationIcon from 'mdi-react/CreationIcon'
import { Link } from '../../../../../shared/src/components/Link'
import { SupersedingBatchSpecFields } from '../../../graphql-operations'
import { parseISO } from 'date-fns'
import { DismissibleAlert } from '../../../components/DismissibleAlert'

export interface SupersedingBatchSpecAlertProps {
    spec: SupersedingBatchSpecFields | null
}

export const SupersedingBatchSpecAlert: React.FunctionComponent<SupersedingBatchSpecAlertProps> = ({ spec }) => {
    if (!spec) {
        return <></>
    }

    const { applyURL, createdAt } = spec
    return (
        <DismissibleAlert
            className="alert alert-info"
            partialStorageKey={`superseding-spec-${parseISO(spec.createdAt).getTime()}`}
        >
            <div className="d-flex align-items-center">
                <div className="d-none d-md-block">
                    <CreationIcon className="icon icon-inline mr-2" />
                </div>
                <div className="flex-grow-1">
                    A <Link to={applyURL}>modified batch spec</Link> was uploaded but not applied{' '}
                    <Timestamp date={createdAt} />.
                </div>
            </div>
        </DismissibleAlert>
    )
}
