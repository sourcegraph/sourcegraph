import { getContext, setContext } from 'svelte'
import { readable, writable, type Readable, type Writable } from 'svelte/store'

import type { Settings, TemporarySettingsStorage } from '$lib/shared'

import type { AuthenticatedUser, FeatureFlag } from '../routes/layout.gql'

export { themeSetting, theme, isLightTheme } from './theme'

// Only exported to be used for mocking tests
// TODO (fkling): Find a better way to initialize mocked contexts and stores
export const KEY = '__sourcegraph__'

export interface SourcegraphContext {
    settings: Readable<Settings | null>
    user: Readable<AuthenticatedUser | null>
    temporarySettingsStorage: Readable<TemporarySettingsStorage>
    featureFlags: Readable<FeatureFlag[]>
}

export function setAppContext(context: SourcegraphContext): void {
    setContext<SourcegraphContext>(KEY, context)
}

export function getStores(): SourcegraphContext {
    return getContext<SourcegraphContext>(KEY)
}

/**
 * This store returns the currently logged in user.
 */
export const user = {
    subscribe(subscriber: (user: AuthenticatedUser | null) => void) {
        const { user } = getStores()
        return user.subscribe(subscriber)
    },
}

/**
 * This store returns the user's settings.
 */
export const settings = {
    subscribe(subscriber: (settings: Settings | null) => void) {
        const { settings } = getStores()
        return settings.subscribe(subscriber)
    },
}

/**
 * A store that updates every second to return the current time.
 */
export const currentDate: Readable<Date> = readable(new Date(), set => {
    set(new Date())
    const interval = setInterval(() => set(new Date()), 1000)
    return () => clearInterval(interval)
})

/**
 * This store syncs the provided value with localStorage. Values must be JSON (de)seralizable.
 */
export function createLocalWritable<T>(localStorageKey: string, defaultValue: T): Writable<T> {
    const { subscribe, set, update } = writable(defaultValue, set => {
        const existingValue = localStorage.getItem(localStorageKey)
        if (existingValue) {
            set(JSON.parse(existingValue))
        }
    })

    return {
        subscribe,
        set: value => {
            set(value)
            localStorage.setItem(localStorageKey, JSON.stringify(value))
        },
        update: fn => {
            update(value => {
                const newValue = fn(value)
                localStorage.setItem(localStorageKey, JSON.stringify(newValue))
                return newValue
            })
        },
    }
}

export const scrollAll = writable(false)
