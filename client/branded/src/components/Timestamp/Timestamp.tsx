import React, { useEffect, useMemo, useState } from 'react'

import { UTCDate } from '@date-fns/utc'
import { format, parseISO, formatDistance, formatDistanceStrict } from 'date-fns'

import { Tooltip } from '@sourcegraph/wildcard'

interface TimestampProps {
    /** The date (if string, in ISO 8601 format). */
    date: string | Date | number

    /** Omit the "about". */
    noAbout?: boolean

    /** Omit the "ago". */
    noAgo?: boolean

    /** Function that returns the current time (for stability in visual tests). */
    now?: () => Date

    /** Whether to use exact timestamps (i.e. omit "less than", "about", etc.) */
    strict?: boolean

    /** Whether to show absolute timestamp and show relative one in tooltip */
    preferAbsolute?: boolean

    /** Optional semantic timestamp format */
    timestampFormat?: TimestampFormat

    utc?: boolean
}

export enum TimestampFormat {
    FULL_TIME = 'HH:mm:ss',
    FULL_DATE = 'yyyy-MM-dd',
    FULL_DATE_TIME = 'yyyy-MM-dd pp',
}

const RERENDER_INTERVAL_MSEC = 7000

/**
 * Displays a date's relative time ("... ago") and shows the full date on hover. Re-renders
 * periodically to ensure the relative time string is up-to-date.
 */
export const Timestamp: React.FunctionComponent<React.PropsWithChildren<TimestampProps>> = ({
    date,
    noAbout = false,
    noAgo = false,
    strict = false,
    now = Date.now,
    preferAbsolute = false,
    timestampFormat,
    utc = false,
}) => {
    const [label, setLabel] = useState<string>(calculateLabel(date, now, strict, noAbout, noAgo))
    useEffect(() => {
        // Update the label
        setLabel(calculateLabel(date, now, strict, noAbout, noAgo))

        // Refresh the label periodically
        const intervalHandle = window.setInterval(
            () => setLabel(calculateLabel(date, now, strict, noAbout, noAgo)),
            RERENDER_INTERVAL_MSEC
        )
        return () => {
            window.clearInterval(intervalHandle)
        }
    }, [date, noAbout, noAgo, now, strict])

    const tooltip = useMemo(() => {
        let parsedDate = typeof date === 'string' ? parseISO(date) : new Date(date)
        if (utc) {
            parsedDate = new UTCDate(date)
        }
        const dateHasTime = date.toString().includes('T')
        const defaultFormat = dateHasTime ? TimestampFormat.FULL_DATE_TIME : TimestampFormat.FULL_DATE
        return format(parsedDate, timestampFormat ?? defaultFormat) + (utc ? ' UTC' : '')
    }, [date, timestampFormat, utc])

    return (
        <Tooltip content={preferAbsolute ? label : tooltip}>
            <span className="timestamp">{preferAbsolute ? tooltip : label}</span>
        </Tooltip>
    )
}

function calculateLabel(
    date: string | Date | number,
    now: () => Date | number,
    strict: boolean,
    noAbout: boolean,
    noAgo: boolean
): string {
    let label: string
    if (strict) {
        label = formatDistanceStrict(typeof date === 'string' ? parseISO(date) : date, now(), {
            addSuffix: !noAgo,
        })
    } else {
        label = formatDistance(typeof date === 'string' ? parseISO(date) : date, now(), {
            addSuffix: !noAgo,
            includeSeconds: true,
        })
    }
    if (noAbout) {
        label = label.replace('about ', '')
    }
    return label
}
