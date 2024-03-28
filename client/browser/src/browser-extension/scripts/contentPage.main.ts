// Set globals first before any imports.
import '../../config/extension.entry'
import '../../config/content.entry'
// Polyfill before other imports.
import '../../shared/polyfills'

import { firstValueFrom, fromEvent, Subscription } from 'rxjs'

import { setLinkComponent, AnchorLink } from '@sourcegraph/wildcard'

import { determineCodeHost } from '../../shared/code-hosts/shared/codeHost'
import { injectCodeIntelligence } from '../../shared/code-hosts/shared/inject'
import {
    checkIsSourcegraph,
    EXTENSION_MARKER_ID,
    injectExtensionMarker,
    NATIVE_INTEGRATION_ACTIVATED,
    signalBrowserExtensionInstalled,
} from '../../shared/code-hosts/sourcegraph/inject'
import { initSentry } from '../../shared/sentry'
import { DEFAULT_SOURCEGRAPH_URL, getAssetsURL, observeSourcegraphURL } from '../../shared/util/context'
import { featureFlags } from '../../shared/util/featureFlags'
import { assertEnvironment } from '../environmentAssertion'

const subscriptions = new Subscription()
window.addEventListener('unload', () => subscriptions.unsubscribe(), { once: true })

assertEnvironment('CONTENT')

const codeHost = determineCodeHost()
initSentry('content', codeHost?.type)

setLinkComponent(AnchorLink)

const IS_EXTENSION = true

// Add style sheet and wait for it to load to avoid rendering unstyled elements (which causes an
// annoying flash/jitter when the stylesheet loads shortly thereafter).
function loadStyleSheet(options: { id: string; path: string }): HTMLLinkElement {
    const { id, path } = options

    let styleSheet = document.querySelector<HTMLLinkElement>(`#${id}`)
    // If does not exist, create
    if (!styleSheet) {
        styleSheet = document.createElement('link')
        styleSheet.id = id
        styleSheet.rel = 'stylesheet'
        styleSheet.type = 'text/css'
        styleSheet.href = browser.extension.getURL(path)
    }
    return styleSheet
}

// If stylesheet is not loaded yet, wait for it to load.
async function waitForStyleSheet(styleSheet: HTMLLinkElement): Promise<void> {
    if (!styleSheet.sheet) {
        await new Promise(resolve => {
            styleSheet.addEventListener('load', resolve, { once: true })
            // If not appended yet, append to <head>
            if (!styleSheet.parentNode) {
                document.head.append(styleSheet)
            }
        })
    }
}

/**
 * Main entry point into browser extension.
 */
async function main(): Promise<void> {
    console.log('Sourcegraph browser extension is running')

    // Make sure DOM is fully loaded
    if (document.readyState !== 'complete' && document.readyState !== 'interactive') {
        await new Promise<Event>(resolve => document.addEventListener('DOMContentLoaded', resolve, { once: true }))
    }

    // Allow users to set this via the console.
    ;(window as any).sourcegraphFeatureFlags = featureFlags

    // Check if a native integration is already running on the page,
    // and abort execution if it's the case.
    // If the native integration was activated before the content script, we can
    // synchronously check for the presence of the extension marker.
    if (document.querySelector(`#${EXTENSION_MARKER_ID}`) !== null) {
        console.log('Sourcegraph native integration is already running')
        return
    }
    // If the extension marker isn't present, inject it and listen for a custom event sent by the native
    // integration to signal its activation.
    injectExtensionMarker()
    const nativeIntegrationActivationEventReceived = firstValueFrom(fromEvent(document, NATIVE_INTEGRATION_ACTIVATED), {
        defaultValue: undefined,
    })

    let previousSubscription: Subscription
    subscriptions.add(
        // eslint-disable-next-line rxjs/no-async-subscribe
        observeSourcegraphURL(IS_EXTENSION).subscribe(async sourcegraphURL => {
            if (previousSubscription) {
                console.log('Sourcegraph detached code navigation')
                previousSubscription.unsubscribe()
            }

            const isSourcegraphServer = checkIsSourcegraph(sourcegraphURL)
            if (isSourcegraphServer) {
                signalBrowserExtensionInstalled()
                return
            }

            try {
                previousSubscription = await injectCodeIntelligence(
                    { sourcegraphURL, assetsURL: getAssetsURL(DEFAULT_SOURCEGRAPH_URL) },
                    IS_EXTENSION,
                    async function onCodeHostFound() {
                        const styleSheets = [
                            {
                                id: 'ext-style-sheet',
                                path: 'css/app.bundle.css',
                            },
                            {
                                id: 'ext-style-sheet-css-modules',
                                path: 'css/contentPage.main.bundle.css',
                            },
                        ]

                        await Promise.all(styleSheets.map(loadStyleSheet).map(waitForStyleSheet))
                    }
                )
                console.log('Sourcegraph attached code navigation')
            } catch (error) {
                console.log('Sourcegraph code host integration stopped initialization. Reason:', error)
            }
        })
    )

    // Clean up susbscription if the native integration gets activated
    // later in the lifetime of the content script.
    await nativeIntegrationActivationEventReceived
    console.log('Native integration activation event received')
    subscriptions.unsubscribe()
}

main().catch(console.error.bind(console))
