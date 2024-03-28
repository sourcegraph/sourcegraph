import { type ApolloClient, gql } from '@apollo/client'
import { isEqual } from 'lodash'
import { Observable, of, type Subscription, from, ReplaySubject, type Subscriber, fromEvent } from 'rxjs'
import { distinctUntilChanged, map, mergeAll, startWith, switchMap } from 'rxjs/operators'

import { logger } from '@sourcegraph/common'
import { fromObservableQuery } from '@sourcegraph/http-client'

import type { GetTemporarySettingsResult } from '../../graphql-operations'

import {
    getTemporarySettingOverride,
    setTemporarySettingOverride,
    temporarySettingsOverrideUpdate,
} from './localOverride'
import type { TemporarySettings } from './TemporarySettings'

export class TemporarySettingsStorage {
    private settingsBackend: SettingsBackend = new LocalStorageSettingsBackend()
    private settings: TemporarySettings = {}

    private onChange = new ReplaySubject<TemporarySettings>(1)

    private loadSubscription: Subscription | null = null
    private saveSubscription: Subscription | null = null

    public dispose(): void {
        this.loadSubscription?.unsubscribe()
        this.saveSubscription?.unsubscribe()
    }

    constructor(
        private apolloClient: ApolloClient<object> | null,
        isAuthenticatedUser: boolean,
        enableLocalOverrides: boolean = false
    ) {
        let backend: SettingsBackend
        if (isAuthenticatedUser) {
            if (!this.apolloClient) {
                throw new Error('Apollo-Client should be initialized for authenticated user')
            }

            backend = new ServersideSettingsBackend(this.apolloClient)
        } else {
            backend = new LocalStorageSettingsBackend()
        }

        if (enableLocalOverrides) {
            backend = new LocalOverrideBackend(backend)
        }

        this.setSettingsBackend(backend)
    }

    // This is public for testing purposes only so mocks can be provided.
    public setSettingsBackend(backend: SettingsBackend): void {
        this.loadSubscription?.unsubscribe()
        this.saveSubscription?.unsubscribe()

        this.settingsBackend = backend

        this.loadSubscription = this.settingsBackend.load().subscribe(settings => {
            this.settings = settings
            this.onChange.next(settings)
        })
    }

    public set<K extends keyof TemporarySettings>(key: K, value: TemporarySettings[K]): void {
        this.settings[key] = value
        this.onChange.next(this.settings)

        this.saveSubscription?.unsubscribe()
        this.saveSubscription = this.settingsBackend.edit({ [key]: value }).subscribe()
    }

    public get<K extends keyof TemporarySettings>(
        key: K,
        defaultValue?: TemporarySettings[K]
    ): Observable<TemporarySettings[K]> {
        return this.onChange.pipe(
            map(settings => (key in settings ? settings[key] : defaultValue)),
            distinctUntilChanged((a, b) => isEqual(a, b))
        )
    }
}

export interface SettingsBackend {
    load: () => Observable<TemporarySettings>
    edit: (settings: TemporarySettings) => Observable<void>
}

/**
 * Settings backend for unauthenticated users.
 * Settings are stored in `localStorage` and updated when
 * the `storage` event is fired on the window.
 */
class LocalStorageSettingsBackend implements SettingsBackend {
    private readonly TemporarySettingsKey = 'temporarySettings'

    public load(): Observable<TemporarySettings> {
        return new Observable<TemporarySettings>(observer => {
            let settingsLoaded = false

            const loadObserver = (observer: Subscriber<TemporarySettings>): void => {
                try {
                    const settings = localStorage.getItem(this.TemporarySettingsKey)
                    if (settings) {
                        const parsedSettings = JSON.parse(settings) as TemporarySettings
                        observer.next(parsedSettings)
                        settingsLoaded = true
                    }
                } catch (error: unknown) {
                    logger.error(error)
                }

                if (!settingsLoaded) {
                    observer.next({})
                }
            }

            loadObserver(observer)

            const loadCallback = (): void => {
                loadObserver(observer)
            }

            window.addEventListener('storage', loadCallback)

            return () => {
                window.removeEventListener('storage', loadCallback)
            }
        })
    }

    public edit(newSettings: TemporarySettings): Observable<void> {
        try {
            const encodedCurrentSettings = localStorage.getItem(this.TemporarySettingsKey) || '{}'
            const currentSettings = JSON.parse(encodedCurrentSettings) as TemporarySettings
            localStorage.setItem(this.TemporarySettingsKey, JSON.stringify({ ...currentSettings, ...newSettings }))
        } catch (error: unknown) {
            logger.error(error)
        }

        return of()
    }
}

/**
 * Settings backend for authenticated users that saves settings to the server.
 * Changes to settings are polled every 5 minutes.
 */
class ServersideSettingsBackend implements SettingsBackend {
    private readonly PollInterval = 1000 * 60 * 5 // 5 minutes

    private readonly GetTemporarySettingsQuery = gql`
        query GetTemporarySettings {
            temporarySettings {
                contents
            }
        }
    `

    private readonly EditTemporarySettingsMutation = gql`
        mutation EditTemporarySettings($contents: String!) {
            editTemporarySettings(settingsToEdit: $contents) {
                alwaysNil
            }
        }
    `

    constructor(private apolloClient: ApolloClient<object>) {}

    public load(): Observable<TemporarySettings> {
        const temporarySettingsQuery = this.apolloClient.watchQuery<GetTemporarySettingsResult>({
            query: this.GetTemporarySettingsQuery,
            pollInterval: this.PollInterval,
            // We can use the `cache-first` policy here because we preload temporary settings on the server,
            // and polling bypasses cache and issues network requests despite having a cached result.
            fetchPolicy: 'cache-first',
        })

        return fromObservableQuery(temporarySettingsQuery).pipe(
            map(({ data }) => {
                let parsedSettings: TemporarySettings = {}

                try {
                    const settings = data.temporarySettings.contents
                    parsedSettings = JSON.parse(settings) as TemporarySettings
                } catch (error: unknown) {
                    logger.error(error)
                }

                return parsedSettings || {}
            })
        )
    }

    public edit(newSettings: TemporarySettings): Observable<void> {
        try {
            const settingsString = JSON.stringify(newSettings)

            return from(
                this.apolloClient.mutate({
                    mutation: this.EditTemporarySettingsMutation,
                    variables: { contents: settingsString },
                    update: cache => {
                        const encodedCurrentSettings =
                            cache.readQuery<GetTemporarySettingsResult>({
                                query: this.GetTemporarySettingsQuery,
                            })?.temporarySettings.contents || '{}'
                        const currentSettings = JSON.parse(encodedCurrentSettings) as TemporarySettings

                        return cache.writeQuery<GetTemporarySettingsResult>({
                            query: this.GetTemporarySettingsQuery,
                            data: {
                                temporarySettings: {
                                    __typename: 'TemporarySettings',
                                    contents: JSON.stringify({ ...currentSettings, ...newSettings }),
                                },
                            },
                        })
                    },
                })
            ).pipe(
                map(() => {}) // Ignore return value, always empty
            )
        } catch (error: unknown) {
            logger.error(error)
        }

        return of()
    }
}

/**
 * Static in memory setting backend for testing purposes
 */
export class InMemoryMockSettingsBackend implements SettingsBackend {
    constructor(
        private settings: TemporarySettings,
        private onSettingsChanged?: (settings: TemporarySettings) => void
    ) {}
    public load(): Observable<TemporarySettings> {
        return of(this.settings)
    }
    public edit(settings: TemporarySettings): Observable<void> {
        this.settings = { ...this.settings, ...settings }
        if (this.onSettingsChanged) {
            this.onSettingsChanged(this.settings)
        }
        return of()
    }
}

/**
 * This is a dev-only backend for intercepting overridden values.
 */
class LocalOverrideBackend implements SettingsBackend {
    constructor(private backend: SettingsBackend) {}

    public load(): Observable<TemporarySettings> {
        return this.backend.load().pipe(
            switchMap(settings =>
                of(temporarySettingsOverrideUpdate, fromEvent(window, 'storage')).pipe(
                    mergeAll(),
                    startWith(settings),
                    map(() => settings)
                )
            ),
            map(settings => {
                const overriddenSettings: any = { ...settings }

                for (const key of Object.keys(settings)) {
                    const overrideValue = getTemporarySettingOverride(key as keyof TemporarySettings)
                    if (overrideValue) {
                        overriddenSettings[key] = overrideValue.value
                    }
                }
                return overriddenSettings
            })
        )
    }

    public edit(newSettings: TemporarySettings): Observable<void> {
        try {
            const newSettingsCopy: any = { ...newSettings }
            for (const [key, value] of Object.entries(newSettingsCopy)) {
                const overrideValue = getTemporarySettingOverride(key as keyof TemporarySettings)
                if (overrideValue) {
                    setTemporarySettingOverride(key as keyof TemporarySettings, { value: value as any })
                    delete newSettingsCopy[key]
                }
            }
            if (Object.keys(newSettingsCopy).length > 0) {
                return this.backend.edit(newSettingsCopy)
            }
        } catch (error: unknown) {
            logger.error(error)
        }

        return of()
    }
}
