import type { ApolloError } from '@apollo/client'
import type { GraphQLError } from 'graphql'
import { isEqual } from 'lodash'

/**
 * Formats an interval in seconds as a human readable form.
 * Examples:
 * 60s will render as 1 minute
 * 60*60s will render as 1 hour
 * 24*60*60s will render as 1 day
 * 24*60*60+60*60+60+5 will render as 1 day 1 hour 1 minute 5 seconds
 */
export function prettyInterval(seconds: number): string {
    let result = ''
    const days = Math.floor(seconds / (24 * 60 * 60))
    seconds -= days * 24 * 60 * 60
    const hours = Math.floor(seconds / (60 * 60))
    seconds -= hours * 60 * 60
    const minutes = Math.floor(seconds / 60)
    seconds -= minutes * 60

    if (days > 0) {
        result += `${days} day${days > 1 ? 's' : ''} `
    }
    if (hours > 0) {
        result += `${hours} hour${hours > 1 ? 's' : ''} `
    }
    if (minutes > 0) {
        result += `${minutes} minute${minutes > 1 ? 's' : ''} `
    }
    if (seconds > 0) {
        result += `${seconds} second${seconds > 1 ? 's' : ''}`
    }

    return result.trim()
}

export function errorForPath(error: ApolloError | undefined, path: (string | number)[]): GraphQLError | undefined {
    return error?.graphQLErrors.find(error => isEqual(error.path, path))
}

export const numberFormatter = new Intl.NumberFormat()
