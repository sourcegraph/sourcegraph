import { combineLatest, Observable, of } from 'rxjs'
import { map, distinctUntilChanged } from 'rxjs/operators'

import { isFirefox } from '@sourcegraph/common'

import { observeStorageKey } from '../../browser-extension/web-extension-api/storage'
import { isExtension } from '../context'

import { isDefaultSourcegraphUrl, observeSourcegraphURL } from './context'

const OPTION_FLAGS_SYNC_STORAGE_KEY = 'featureFlags'

export type OptionFlagKey = 'sendTelemetry' | 'allowErrorReporting'

export interface OptionFlagDefinition {
    label: string
    key: OptionFlagKey
    hidden?: boolean
}

export interface OptionFlagWithValue<T = boolean> extends OptionFlagDefinition {
    value: T
}

export type OptionFlagValues<T = boolean> = Record<OptionFlagKey, T>

export const optionFlagDefinitions: OptionFlagDefinition[] = [
    {
        key: 'sendTelemetry',
        label: 'Send telemetry',
    },
    {
        key: 'allowErrorReporting',
        label: 'Allow error reporting',
    },
]

const optionFlagDefaults: OptionFlagValues = {
    sendTelemetry: false,
    allowErrorReporting: false,
}

const assignOptionFlagValues = (values: OptionFlagValues): OptionFlagWithValue[] =>
    optionFlagDefinitions.map(flag => ({ ...flag, value: values[flag.key] }))

/**
 * Apply default values to option flags, taking a partial option flag values
 * object and returning a complete option flag values object.
 */
const applyOptionFlagDefaults = (values: Partial<OptionFlagValues> | undefined): OptionFlagValues => ({
    ...optionFlagDefaults,
    ...values,
})

/**
 * Observe the option flags object, with default values already applied.
 */
const observeOptionFlags = (): Observable<OptionFlagValues> => {
    const optionFlagsStorageObservable = isExtension
        ? observeStorageKey('sync', OPTION_FLAGS_SYNC_STORAGE_KEY)
        : of(undefined)
    return optionFlagsStorageObservable.pipe(map(applyOptionFlagDefaults))
}

/**
 * Observe an option flag value, with default value already applied.
 */
export function observeOptionFlag(key: OptionFlagKey): Observable<boolean> {
    return observeOptionFlags().pipe(
        map(value => value[key]),
        distinctUntilChanged()
    )
}

/**
 * Determine if the sendTelemetry option flag should be overridden.
 *
 * This function encapsulates the logic of when telemetry should be overridden.
 */
const shouldOverrideSendTelemetry = (isExtension: boolean): Observable<boolean> =>
    observeSourcegraphURL(isExtension).pipe(
        map(sourcegraphUrl => {
            const isFirefoxExtension = isFirefox() && isExtension
            if (!isFirefoxExtension) {
                return true
            }

            if (!isDefaultSourcegraphUrl(sourcegraphUrl)) {
                return true
            }

            return false
        })
    )

/**
 * Determine if the sendTelemetry is enabled
 */
export const observeSendTelemetry = (isExtension: boolean): Observable<boolean> =>
    combineLatest([shouldOverrideSendTelemetry(isExtension), observeOptionFlag('sendTelemetry')]).pipe(
        map(([override, sendTelemetry]) => {
            if (override) {
                return true
            }
            return sendTelemetry
        })
    )

/**
 * A list of option flags with values
 */
export const observeOptionFlagsWithValues = (isExtension: boolean): Observable<OptionFlagWithValue[]> =>
    combineLatest([observeOptionFlags(), shouldOverrideSendTelemetry(isExtension)]).pipe(
        map(([flags, override]) => {
            const definitions = assignOptionFlagValues(flags)
            if (override) {
                return definitions.filter(flag => flag.key !== 'sendTelemetry')
            }
            return definitions
        })
    )
