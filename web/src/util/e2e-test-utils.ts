import pRetry from 'p-retry'
import { OperationOptions } from 'retry'

/**
 * Retry function with more sensible defaults for e2e test assertions
 *
 * @param fn The async assertion function to retry
 * @param options Option overrides passed to pRetry
 */
export const retry = (fn: (attempt: number) => Promise<any>, options: OperationOptions = {}) =>
    pRetry(fn, { factor: 1, ...options })

/**
 * Looks up an environment variable and parses it as a boolean. Throws when not
 * set and no default is provided, or if parsing fails.
 */
export function readEnvBoolean({
    variable: variable,
    defaultValue,
}: {
    variable: string
    defaultValue?: boolean
}): boolean {
    const value = process.env[variable]
    if (value === undefined || value === '') {
        if (defaultValue === undefined) {
            throw new Error(`Environment variable ${variable} must be set.`)
        } else {
            return defaultValue
        }
    } else if (value === 'true') {
        return true
    } else if (value === 'false') {
        return false
    } else {
        throw new Error(
            `Incorrect environment variable ${variable}=${value}. Must be set to true/false or not set at all.`
        )
    }
}

/**
 * Looks up an environment variable. Throws when not set and no default is
 * provided.
 */
export function readEnvString({ variable, defaultValue }: { variable: string; defaultValue?: string }): string {
    const value = process.env[variable]
    if (value === undefined || value === '') {
        if (defaultValue === undefined) {
            throw new Error(`Environment variable ${variable} must be set.`)
        } else {
            return defaultValue
        }
    } else {
        return value
    }
}
