import React from 'react'

import { mdiCloudAlert, mdiCloudClock } from '@mdi/js'
import classNames from 'classnames'
import { format } from 'date-fns'

import { Icon, Tooltip } from '@sourcegraph/wildcard'

interface Props {
    lastSyncedTime: string
    className?: string
}

export const LastSyncedIcon: React.FunctionComponent<React.PropsWithChildren<Props>> = props => {
    const parsedDate = new Date(Date.parse(props.lastSyncedTime))
    const formattedTime = format(parsedDate, 'yyyy-MM-dd pp')

    const oneDayAgo = new Date()
    oneDayAgo.setDate(oneDayAgo.getDate() - 1)

    const oneWeekAgo = new Date()
    oneWeekAgo.setDate(oneWeekAgo.getDate() - 7)

    let color: string | undefined
    let status: string | undefined
    let icon: string | undefined
    if (parsedDate < oneWeekAgo) {
        color = 'var(--danger)'
        status = `Warning: severely out of date, last synced:  ${formattedTime}. Please contact your administrator.`
        icon = mdiCloudAlert
    } else if (parsedDate < oneDayAgo) {
        color = 'var(--warning)'
        status = `Warning: out of date, last synced: ${formattedTime}.`
        icon = mdiCloudClock
    }

    return color !== undefined && status !== undefined && icon !== undefined ? (
        <Tooltip content={status}>
            <Icon
                tabIndex={0}
                className={classNames(props.className, 'text-muted')}
                aria-label={status}
                svgPath={icon}
                style={{ fill: color }}
            />
        </Tooltip>
    ) : null
}
