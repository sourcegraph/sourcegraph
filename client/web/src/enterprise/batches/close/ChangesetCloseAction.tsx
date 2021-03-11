import React from 'react'
import CloseCircleOutlineIcon from 'mdi-react/CloseCircleOutlineIcon'
import PlayCircleOutlineIcon from 'mdi-react/PlayCircleOutlineIcon'
import classNames from 'classnames'

const iconClassNames = 'm-0 text-nowrap d-block d-sm-flex flex-column align-items-center justify-content-center'

export const ChangesetCloseActionClose: React.FunctionComponent<{ className?: string }> = ({ className }) => (
    <div className={classNames(className, iconClassNames, 'changeset-close-action__close-flash')}>
        <CloseCircleOutlineIcon data-tooltip="This changeset will be closed on the code host when the batch change is closed." />
        <span className="text-muted">Will close</span>
    </div>
)
export const ChangesetCloseActionKept: React.FunctionComponent<{ className?: string }> = ({ className }) => (
    <div className={classNames(className, iconClassNames)}>
        <PlayCircleOutlineIcon data-tooltip="This changeset will NOT be closed on the code host when the batch change is closed." />
        <span className="text-muted">Kept open</span>
    </div>
)
