import React from 'react'
import classNames from 'classnames'
import InformationOutlineIcon from 'mdi-react/InformationOutlineIcon'
import { BatchChangeFields } from '../../../graphql-operations'

interface ClosedNoticeProps {
    closedAt: BatchChangeFields['closedAt']
    className?: string
}

export const ClosedNotice: React.FunctionComponent<ClosedNoticeProps> = ({ closedAt, className }) => {
    if (!closedAt) {
        return <></>
    }
    return (
        <div className={classNames('alert alert-info', className)}>
            <InformationOutlineIcon className="icon-inline" /> Information on this page may be out of date because
            changesets that only exist in closed batch changes are not synced with the code host.
        </div>
    )
}
