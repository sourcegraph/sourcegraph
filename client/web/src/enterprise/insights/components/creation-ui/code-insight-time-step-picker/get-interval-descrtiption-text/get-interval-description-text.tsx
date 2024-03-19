import type { ReactNode } from 'react'

import type { Duration } from 'date-fns'

import type { InsightStep } from '../../../../pages/insights/creation/search-insight'

// eslint-disable-next-line @typescript-eslint/no-namespace
declare namespace Intl {
    type ListType = 'conjunction' | 'disjunction'

    interface ListFormatOptions {
        localeMatcher?: 'lookup' | 'best fit'
        type?: ListType
        style?: 'long' | 'short' | 'narrow'
    }

    class ListFormat {
        constructor(locales?: string | string[], options?: ListFormatOptions)

        public format: (items: string[]) => string
    }
}

const LIST_FORMATTER = (() => {
    try {
        return new Intl.ListFormat('en', { style: 'long', type: 'conjunction' })
    } catch {
        return {
            format(items: string[]) {
                return items.join(', ')
            },
        }
    }
})()

const INTERVALS = [
    { type: 'years', inMinutes: 60 * 24 * 7 * 5 * 12 },
    { type: 'months', inMinutes: 60 * 24 * 7 * 5 },
    { type: 'weeks', inMinutes: 60 * 24 * 7 },
    { type: 'days', inMinutes: 60 * 24 },
    { type: 'hours', inMinutes: 60 },
]

interface DescriptionTextOptions {
    numberOfPoints: number
    stepType: InsightStep
    stepValue: number
}

export function getDescriptionText(options: DescriptionTextOptions): ReactNode {
    const { stepType, stepValue, numberOfPoints } = options
    // Remove s at the end of stepType value, in the singular. We need to do this
    // because Intl accepts only singular value of units.
    const unit = stepType.slice(0, -1)

    const intervalText = formatDuration({ [stepType]: stepValue * (numberOfPoints - 1) })
    const everyUnit = stepValue.toLocaleString('en-GB', {
        unit,
        style: 'unit',
        unitDisplay: 'long',
    })

    const everyUnitText = stepValue < 2 ? everyUnit.slice(2) : everyUnit

    return (
        <span>
            Show the past <b>{intervalText} of data</b>, one datapoint every {everyUnitText}. This insight provides{' '}
            {numberOfPoints} datapoints.
        </span>
    )
}

export function formatDuration(duration: Duration): string {
    const intervalParts = []
    let intervalInMinutes = toMinutes(duration)

    for (const interval of INTERVALS) {
        const amount = Math.floor(intervalInMinutes / interval.inMinutes)

        intervalInMinutes -= amount * interval.inMinutes

        if (amount !== 0) {
            intervalParts.push({ unit: interval.type.slice(0, -1), value: amount })
        }
    }

    const formattedIntervals = intervalParts.map(interval =>
        interval.value.toLocaleString('en-GB', {
            unit: interval.unit,
            style: 'unit',
            unitDisplay: 'long',
        })
    )

    return LIST_FORMATTER.format(formattedIntervals)
}

const toMinutes = (duration: Duration): number => {
    const { minutes = 0, hours = 0, days = 0, weeks = 0, months = 0, years = 0 } = duration

    return (
        minutes +
        hours * 60 +
        days * 24 * 60 +
        weeks * 60 * 24 * 7 +
        months * 60 * 24 * 7 * 5 +
        years * 60 * 24 * 7 * 5 * 12
    )
}
