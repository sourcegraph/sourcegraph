// Set globals first before any imports.
import '../../config/extension.entry'
import '../../config/background.entry'
// Polyfill before other imports.
import '../../shared/polyfills'

import type { Endpoint } from 'comlink'
import { combineLatest, merge, type Observable, of, Subject, Subscription, timer, lastValueFrom } from 'rxjs'
import {
    bufferCount,
    filter,
    groupBy,
    map,
    mergeMap,
    switchMap,
    take,
    concatMap,
    catchError,
    distinctUntilChanged,
} from 'rxjs/operators'
import addDomainPermissionToggle from 'webext-domain-permission-toggle'

import { isDefined, fetchCache } from '@sourcegraph/common'
import { type GraphQLResult, requestGraphQLCommon } from '@sourcegraph/http-client'
import { createExtensionHostWorker } from '@sourcegraph/shared/src/api/extension/worker'
import type { EndpointPair } from '@sourcegraph/shared/src/platform/context'
import { createURLWithUTM } from '@sourcegraph/shared/src/tracking/utm'

import { getHeaders } from '../../shared/backend/headers'
import { fetchSite } from '../../shared/backend/server'
import { initializeOmniboxInterface } from '../../shared/cli'
import { browserPortToMessagePort, findMessagePorts } from '../../shared/platform/ports'
import { createBlobURLForBundle } from '../../shared/platform/worker'
import { initSentry } from '../../shared/sentry'
import { ConditionalTelemetryRecorderProvider } from '../../shared/telemetry'
import { EventLogger } from '../../shared/tracking/eventLogger'
import { getExtensionVersion, getPlatformName, observeSourcegraphURL } from '../../shared/util/context'
import { type BrowserActionIconState, setBrowserActionIconState } from '../browser-action-icon'
import { assertEnvironment } from '../environmentAssertion'
import { checkUrlPermissions, IsProductionVersion } from '../util'
import { fromBrowserEvent } from '../web-extension-api/fromBrowserEvent'
import { observeStorageKey, storage } from '../web-extension-api/storage'
import type { BackgroundPageApi, BackgroundPageApiHandlers } from '../web-extension-api/types'

const IS_EXTENSION = true

// Interval to check if the Sourcegraph URL is valid
// This polling allows to detect if Sourcegraph instance is invalid or needs authentication.
const INTERVAL_FOR_SOURCEGRPAH_URL_CHECK = 5 /* minutes */ * 60 * 1000

assertEnvironment('BACKGROUND')

initSentry('background')

/**
 * For each tab, we store a flag if we know that we are on:
 * - a private repo not synced with Sourcegraph Cloud when the latter is the active Sourcegraph URL
 * - a repo not added to the other than Cloud Sourcegraph instance (+ the extension * points to this instance).
 * The content script notifies the background page if it has experienced this kind of an error
 * by sending `notifyRepoSyncError` message.
 */
const tabRepoSyncErrorCache = (() => {
    const cache = new Map<number, Map<string, boolean>>()
    const subject = new Subject<ReadonlyMap<number, Map<string, boolean>>>()
    return {
        observable: subject.asObservable(),
        /**
         * Update the background page's cache of which tabs have experienced either a
         * private code on Cloud or not synced repo on other than Cloud Sourcegrpah instance error.
         */
        setTabHasRepoSyncError(tabId: number, hasRepoSyncError: boolean, sourcegraphURL?: string): void {
            if (sourcegraphURL) {
                let record = cache.get(tabId)

                if (!record) {
                    record = new Map()
                    cache.set(tabId, record)
                }

                if (hasRepoSyncError) {
                    record.set(sourcegraphURL, true)
                } else if (record.size === 0) {
                    cache.delete(tabId)
                } else {
                    record.delete(sourcegraphURL)
                }
            } else {
                cache.delete(tabId)
            }

            // Emit the updated repository cache when it changes, so that consumers can
            // observe the value.
            subject.next(cache)
        },

        /**
         * Check whether the background page's cache contains data about repo sync error for
         * the given parameters.
         */
        getTabHasRepoSyncError(tabId: number, sourcegraphURL: string): boolean {
            return !!cache.get(tabId)?.get(sourcegraphURL)
        },
    }
})()

const configureOmnibox = (serverUrl: string): void => {
    browser.omnibox.setDefaultSuggestion({
        description: `Search code on ${serverUrl}`,
    })
}

const requestGraphQL = <T, V = object>({
    request,
    variables,
    sourcegraphURL,
}: {
    request: string
    variables: V
    sourcegraphURL?: string
}): Observable<GraphQLResult<T>> =>
    (sourcegraphURL ? of(sourcegraphURL) : observeSourcegraphURL(IS_EXTENSION)).pipe(
        take(1),
        switchMap(sourcegraphURL =>
            requestGraphQLCommon<T, V>({
                request,
                variables,
                baseUrl: sourcegraphURL,
                headers: getHeaders(),
                credentials: 'include',
            })
        )
    )

async function main(): Promise<void> {
    const subscriptions = new Subscription()

    // Open installation page after the extension was installed
    browser.runtime.onInstalled.addListener(event => {
        if (event.reason !== 'install') {
            return
        }

        if (IsProductionVersion) {
            subscriptions.add(
                observeSourcegraphURL(IS_EXTENSION)
                    .pipe(take(1))
                    .subscribe(sourcegraphURL => {
                        new EventLogger(requestGraphQL, sourcegraphURL)
                            .log('BrowserExtensionInstalled')
                            .then(() => console.log(`Triggered "BrowserExtensionInstalled" using ${sourcegraphURL}`))
                            .catch(error => console.error('Error triggering "BrowserExtensionInstalled" event:', error))
                        new ConditionalTelemetryRecorderProvider(of(true), requestGraphQL)
                            .getRecorder()
                            .recordEvent('browserExtension', 'install')
                    })
            )
        }

        browser.tabs.create({ url: browser.extension.getURL('after_install.html') }).catch(error => {
            console.error('Error opening after-install page:', error)
        })
    })

    // Mirror the managed sourcegraphURL to sync storage
    subscriptions.add(
        observeStorageKey('managed', 'sourcegraphURL')
            .pipe(
                filter(isDefined),
                concatMap(sourcegraphURL => storage.sync.set({ sourcegraphURL }))
            )
            .subscribe()
    )

    if (browser.omnibox) {
        initializeOmniboxInterface()

        // Configure the omnibox when the sourcegraphURL changes.
        subscriptions.add(
            observeSourcegraphURL(IS_EXTENSION).subscribe(sourcegraphURL => {
                configureOmnibox(sourcegraphURL)
            })
        )
    }

    // Update the browserAction icon based on the state of the extension
    subscriptions.add(
        observeBrowserActionState().subscribe(state => {
            setBrowserActionIconState(state)
        })
    )

    const permissions = await browser.permissions.getAll()
    if (!permissions.origins) {
        return
    }

    browser.tabs.onUpdated.addListener((tabId, changeInfo, tab) => {
        if (changeInfo.status === 'loading') {
            // A new URL is loading in the tab, so clear the cached private cloud error flag.
            tabRepoSyncErrorCache.setTabHasRepoSyncError(tabId, false)
            return
        }

        if (tab.url && changeInfo.status === 'complete') {
            checkUrlPermissions(tab.url)
                .then(async hasPermissions => {
                    if (hasPermissions) {
                        /**
                         * Loading content script dynamically
                         * See https://developer.mozilla.org/en-US/docs/Mozilla/Add-ons/WebExtensions/Content_scripts#loading_content_scripts
                         */
                        await browser.tabs.executeScript(tabId, {
                            file: 'js/contentPage.main.bundle.js',
                            runAt: 'document_end',
                        })
                    }
                })
                .catch(console.warn)
        }
    })

    browser.tabs.onRemoved.addListener(tabId => {
        tabRepoSyncErrorCache.setTabHasRepoSyncError(tabId, false)
    })

    const handlers: BackgroundPageApiHandlers = {
        async openOptionsPage(): Promise<void> {
            await browser.runtime.openOptionsPage()
        },

        async createBlobURL(bundleUrl: string): Promise<string> {
            return createBlobURLForBundle(bundleUrl)
        },

        async requestGraphQL<T, V = object>({
            request,
            variables,
            sourcegraphURL,
        }: {
            request: string
            variables: V
            sourcegraphURL?: string
        }): Promise<GraphQLResult<T>> {
            return lastValueFrom(requestGraphQL<T, V>({ request, variables, sourcegraphURL }))
        },

        async notifyRepoSyncError({ sourcegraphURL, hasRepoSyncError }, sender: browser.runtime.MessageSender) {
            const tabId = sender.tab?.id
            if (tabId !== undefined) {
                tabRepoSyncErrorCache.setTabHasRepoSyncError(tabId, hasRepoSyncError, sourcegraphURL)
            }
            return Promise.resolve()
        },

        async checkRepoSyncError({ tabId, sourcegraphURL }) {
            return Promise.resolve(!!tabRepoSyncErrorCache.getTabHasRepoSyncError(tabId, sourcegraphURL))
        },

        fetchCache,
    }

    // Handle calls from other scripts
    browser.runtime.onMessage.addListener(async (message: { type: keyof BackgroundPageApi; payload: any }, sender) => {
        const method = message.type

        if (!handlers[method]) {
            throw new Error(`Invalid RPC call for "${method}"`)
        }

        // https://stackoverflow.com/questions/55572797/why-does-typescript-expect-never-as-function-argument-when-retrieving-the-func
        return (
            handlers[method] as (
                payload: any,
                sender?: browser.runtime.MessageSender
            ) => ReturnType<BackgroundPageApi[typeof method]>
        )(message.payload, sender)
    })

    await browser.runtime.setUninstallURL(
        createURLWithUTM(
            new URL('https://sourcegraph.com/uninstall'),
            IsProductionVersion
                ? {
                      utm_source: getPlatformName(),
                      utm_campaign: 'browser-extension-uninstall',
                      utm_content: getExtensionVersion(),
                  }
                : {}
        ).href
    )

    // The `popup=true` param is used by the options page to determine if it's
    // loaded in the popup or in th standalone options page.
    browser.browserAction.setPopup({ popup: 'options.html?popup=true' })

    // Add "Enable Sourcegraph on this domain" context menu item
    addDomainPermissionToggle()

    const ENDPOINT_KIND_REGEX = /^(proxy|expose)-/

    const portKind = (port: browser.runtime.Port): string | undefined => {
        const match = port.name.match(ENDPOINT_KIND_REGEX)
        return match?.[1]
    }

    /**
     * A stream of EndpointPair created from Port objects emitted by browser.runtime.onConnect.
     *
     * On initialization, the content script creates a pair of browser.runtime.Port objects
     * using browser.runtime.connect(). The two ports are named 'proxy-{uuid}' and 'expose-{uuid}',
     * and wrapped using {@link endpointFromPort} to behave like comlink endpoints on the content script side.
     *
     * This listens to events on browser.runtime.onConnect, pairs emitted ports using their naming pattern,
     * and emits pairs. Each pair of ports represents a connection with an instance of the content script.
     */
    const browserPortPairs: Observable<Record<keyof EndpointPair, browser.runtime.PortWithSender>> = fromBrowserEvent(
        browser.runtime.onConnect
    ).pipe(
        map(([port]) => port),
        groupBy(
            port => (port.name || 'other').replace(ENDPOINT_KIND_REGEX, ''),
            port => port,
            group => group.pipe(bufferCount(2))
        ),
        filter(group => group.key !== 'other'),
        mergeMap(group =>
            group.pipe(
                bufferCount(2),
                map(ports => {
                    const proxyPort = ports.find(port => portKind(port) === 'proxy')
                    if (!proxyPort) {
                        throw new Error('No proxy port')
                    }
                    const exposePort = ports.find(port => portKind(port) === 'expose')
                    if (!exposePort) {
                        throw new Error('No expose port')
                    }
                    return {
                        proxy: proxyPort,
                        expose: exposePort,
                    }
                })
            )
        )
    )

    // Extension Host Connection
    // When an Port pair is emitted, create an extension host worker.
    // Messages from the ports are forwarded to the endpoints returned by {@link createExtensionHostWorker}, and vice-versa.
    // The lifetime of the extension host worker is tied to that of the content script instance:
    // when a port disconnects, the worker is terminated. This means there should always be exactly one
    // extension host worker per active instance of the content script.
    subscriptions.add(
        browserPortPairs.subscribe({
            next: browserPortPair => {
                subscriptions.add(handleBrowserPortPair(browserPortPair))
            },
            error: error => {
                console.error('Error handling extension host client connection', error)
            },
        })
    )

    console.log('Sourcegraph background page initialized')
}

const workerBundleURL = browser.runtime.getURL('js/extensionHostWorker.bundle.js')

/**
 * Handle an incoming browser port pair coming from a content script.
 */
function handleBrowserPortPair(
    browserPortPair: Record<keyof EndpointPair, browser.runtime.PortWithSender>
): Subscription {
    /** Subscriptions for this browser port pair */
    const subscriptions = new Subscription()

    console.log('Extension host client connected')
    const { worker, clientEndpoints } = createExtensionHostWorker(workerBundleURL)
    subscriptions.add(() => worker.terminate())

    /** Forwards all messages between two endpoints (in one direction) */
    const forwardEndpoint = (from: Endpoint, to: Endpoint): void => {
        const messageListener = (event: Event): void => {
            const { data } = event as MessageEvent
            to.postMessage(data, [...findMessagePorts(data)])
        }
        from.addEventListener('message', messageListener)
        subscriptions.add(() => from.removeEventListener('message', messageListener))

        // False positive https://github.com/eslint/eslint/issues/12822
        from.start?.()
    }

    const linkPortAndEndpoint = (role: keyof EndpointPair): void => {
        const browserPort = browserPortPair[role]
        const endpoint = clientEndpoints[role]
        const tabId = browserPort.sender.tab?.id
        if (!tabId) {
            throw new Error('Expected Port to come from tab')
        }
        const link = browserPortToMessagePort(browserPort, `comlink-${role}-`, name =>
            browser.tabs.connect(tabId, { name })
        )
        subscriptions.add(link.subscription)

        forwardEndpoint(link.messagePort, endpoint)
        forwardEndpoint(endpoint, link.messagePort)

        // Clean up when the port disconnects
        const disconnectListener = subscriptions.unsubscribe.bind(subscriptions)
        browserPort.onDisconnect.addListener(disconnectListener)
        subscriptions.add(() => browserPort.onDisconnect.removeListener(disconnectListener))
    }

    // Connect proxy client endpoint
    linkPortAndEndpoint('proxy')
    // Connect expose client endpoint
    linkPortAndEndpoint('expose')

    return subscriptions
}

// Browsers log this unhandled Promise automatically (and with a better stack trace through console.error)
// eslint-disable-next-line @typescript-eslint/no-floating-promises
main()

function validateSite(): Observable<boolean> {
    return fetchSite(requestGraphQL).pipe(
        map(() => true),
        catchError(() => [false])
    )
}

/**
 * Return an observable of the currently active tab id, which changes every time
 * the user opens or switches to a different tab.
 */
function observeCurrentTabId(): Observable<number> {
    return fromBrowserEvent(browser.tabs.onActivated).pipe(map(([event]) => event.tabId))
}

/**
 * Returns an observable that indicates whether the current tab has experienced
 * a private code on Cloud error.
 */
function observeCurrentTabRepoSyncError(): Observable<boolean> {
    return combineLatest([
        observeCurrentTabId(),
        observeStorageKey('sync', 'sourcegraphURL'),
        tabRepoSyncErrorCache.observable,
    ]).pipe(
        map(
            ([tabId, sourcegraphURL, repoSyncErrorCache]) =>
                !!(sourcegraphURL && repoSyncErrorCache.get(tabId)?.get(sourcegraphURL))
        ),
        distinctUntilChanged()
    )
}

function observeSourcegraphUrlValidation(): Observable<boolean> {
    return merge(
        // Whenever the URL was persisted to storage, we can assume it was validated before-hand
        observeStorageKey('sync', 'sourcegraphURL').pipe(map(() => true)),
        timer(0, INTERVAL_FOR_SOURCEGRPAH_URL_CHECK).pipe(mergeMap(() => validateSite()))
    )
}

function observeBrowserActionState(): Observable<BrowserActionIconState> {
    return combineLatest([
        observeStorageKey('sync', 'disableExtension'),
        observeSourcegraphUrlValidation(),
        observeCurrentTabRepoSyncError(),
    ]).pipe(
        map(([isDisabled, isSourcegraphUrlValid, hasRepoSyncError]) => {
            if (isDisabled) {
                return 'inactive'
            }

            if (!isSourcegraphUrlValid || hasRepoSyncError) {
                return 'active-with-alert'
            }

            return 'active'
        }),
        distinctUntilChanged()
    )
}
