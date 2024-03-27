import { pluralize } from '@sourcegraph/common'
import type { Progress, Skipped } from '@sourcegraph/shared/src/search/stream'

export const abbreviateNumber = (number: number): string => {
    if (number < 1e3) {
        return number.toString()
    }
    if (number >= 1e3 && number < 1e6) {
        return (number / 1e3).toFixed(1) + 'k'
    }
    if (number >= 1e6 && number < 1e9) {
        return (number / 1e6).toFixed(1) + 'm'
    }
    return (number / 1e9).toFixed(1) + 'b'
}

export const getProgressText = (progress: Progress): { visibleText: string; readText: string; duration: number } => {
    const contentWithoutTimeUnit =
        `${abbreviateNumber(progress.matchCount)}` +
        `${limitHit(progress) ? '+' : ''} ${pluralize('result', progress.matchCount)} in ` +
        `${(progress.durationMs / 1000).toFixed(2)}`
    const visibleText = `${contentWithoutTimeUnit}s`
    const readText = `${contentWithoutTimeUnit} seconds`
    const duration = progress.durationMs
    return { visibleText, readText, duration }
}

export const limitHit = (progress: Progress): boolean =>
    progress.skipped.some(skipped => skipped.reason.indexOf('-limit') > 0)

const severityToNumber = (severity: Skipped['severity']): number => {
    switch (severity) {
        case 'error': {
            return 1
        }
        case 'warn': {
            return 2
        }
        case 'info': {
            return 3
        }
    }
}

const severityComparer = (a: Skipped, b: Skipped): number => {
    const aSev = severityToNumber(a.severity)
    const bSev = severityToNumber(b.severity)

    return aSev - bSev
}

export const sortBySeverity = (skipped: Skipped[]): Skipped[] => skipped.slice().sort(severityComparer)
