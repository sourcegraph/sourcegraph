import classNames from 'classnames'
import React, { FunctionComponent } from 'react'
import { LSIFIndexState, LSIFUploadState } from '../../../graphql-operations'
import { CodeIntelStateDescription } from './CodeIntelStateDescription'
import { CodeIntelStateIcon } from './CodeIntelStateIcon'

export interface CodeIntelStateBannerProps {
    typeName: string
    pluralTypeName: string
    state: LSIFUploadState | LSIFIndexState
    placeInQueue?: number | null
    failure?: string | null
    className?: string
}

export const CodeIntelStateBanner: FunctionComponent<CodeIntelStateBannerProps> = ({
    typeName,
    pluralTypeName,
    state,
    placeInQueue,
    failure,
    className = 'alert-primary',
}) => (
    <div className={classNames('alert', className)}>
        <span className="icon-inline">
            <CodeIntelStateIcon state={state} />
        </span>
        <span className="ml-2">
            <CodeIntelStateDescription
                state={state}
                placeInQueue={placeInQueue}
                failure={failure}
                typeName={typeName}
                pluralTypeName={pluralTypeName}
            />
        </span>
    </div>
)
