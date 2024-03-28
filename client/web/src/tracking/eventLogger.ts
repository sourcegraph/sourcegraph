import { EMPTY, fromEvent, merge, ReplaySubject, type Observable } from 'rxjs'
import { catchError, map, share, take } from 'rxjs/operators'
import * as uuid from 'uuid'

import { isErrorLike, isFirefox, logger } from '@sourcegraph/common'
import type { SharedEventLogger } from '@sourcegraph/shared/src/api/sharedEventLogger'
import { EventClient } from '@sourcegraph/shared/src/graphql-operations'
import type { TelemetryService } from '@sourcegraph/shared/src/telemetry/telemetryService'
import type { UTMMarker } from '@sourcegraph/shared/src/tracking/utm'

import { EventName } from '../util/constants'
import { observeQuerySelector } from '../util/dom'

import { serverAdmin } from './services/serverAdminWrapper'
import { sessionTracker } from './sessionTracker'
import { userTracker } from './userTracker'
import { stripURLParameters } from './util'

export const FIRST_SOURCE_URL_KEY = 'sourcegraphSourceUrl'
export const LAST_SOURCE_URL_KEY = 'sourcegraphRecentSourceUrl'
export const ORIGINAL_REFERRER_KEY = 'originalReferrer'
export const MKTO_ORIGINAL_REFERRER_KEY = '_mkto_referrer'
export const SESSION_REFERRER_KEY = 'sessionReferrer'
export const SESSION_FIRST_URL_KEY = 'sessionFirstUrl'

const EXTENSION_MARKER_ID = '#sourcegraph-app-background'

/**
 * Indicates if the webapp ever receives a message from the user's Sourcegraph browser extension,
 * either in the form of a DOM marker element, or from a CustomEvent.
 */
const browserExtensionMessageReceived: Observable<{ platform?: string; version?: string }> = merge(
    // If the marker exists, the extension is installed
    observeQuerySelector({ selector: EXTENSION_MARKER_ID, timeout: 10000 }).pipe(
        map(extensionMarker => ({
            platform: (extensionMarker as HTMLElement)?.dataset?.platform,
            version: (extensionMarker as HTMLElement)?.dataset?.version,
        })),
        catchError(() => EMPTY)
    ),
    // If not, listen for a registration event
    fromEvent<CustomEvent<{ platform?: string; version?: string }>>(
        document,
        'sourcegraph:browser-extension-registration'
    ).pipe(
        take(1),
        map(({ detail }) => {
            try {
                return { platform: detail?.platform, version: detail?.version }
            } catch (error) {
                // Temporary to fix issues on Firefox (https://github.com/sourcegraph/sourcegraph/issues/25998)
                if (
                    isFirefox() &&
                    isErrorLike(error) &&
                    error.message.includes('Permission denied to access property "platform"')
                ) {
                    return {
                        platform: 'firefox-extension',
                        version: 'unknown due to <<Permission denied to access property "platform">>',
                    }
                }

                throw error
            }
        })
    )
).pipe(
    // Replay the same latest value for every subscriber
    share({
        connector: () => new ReplaySubject(1),
        resetOnError: false,
        resetOnComplete: false,
        resetOnRefCountZero: false,
    })
)

export class EventLogger implements TelemetryService, SharedEventLogger {
    public readonly user = userTracker
    public readonly session = sessionTracker

    private hasStrippedQueryParameters = false
    private eventID = 0
    private listeners: Set<(eventName: string) => void> = new Set()

    /**
     * @deprecated Use a TelemetryRecorder or TelemetryRecorderProvider from
     * src/telemetry instead.
     */
    constructor() {
        // EventLogger is never teared down
        // eslint-disable-next-line rxjs/no-ignored-subscription
        browserExtensionMessageReceived.subscribe(({ platform, version }) => {
            const args = { platform, version }
            this.log('BrowserExtensionConnectedToServer', args, args)

            if (debugEventLoggingEnabled()) {
                logger.debug('%cBrowser extension detected, sync completed', 'color: #aaa')
            }
        })
    }

    private logViewEventInternal(eventName: string, eventProperties?: any, logAsActiveUser = true): void {
        const props = pageViewQueryParameters(window.location.href)
        serverAdmin.trackPageView(eventName, logAsActiveUser, eventProperties)
        this.logToConsole(eventName, props)

        // Use flag to ensure URL query params are only stripped once
        if (!this.hasStrippedQueryParameters) {
            handleQueryEvents(window.location.href)
            this.hasStrippedQueryParameters = true
        }
    }

    /**
     * Log a pageview.
     * Page titles should be specific and human-readable in pascal case, e.g. "SearchResults" or "Blob" or "NewOrg"
     *
     * @deprecated Use a TelemetryRecorder or TelemetryRecorderProvider from
     * src/telemetry instead.
     */
    public logViewEvent(pageTitle: string, eventProperties?: any, logAsActiveUser = true): void {
        // call to refresh the session
        this.resetSessionCookieExpiration()

        if (window.context?.userAgentIsBot || !pageTitle) {
            return
        }
        pageTitle = `View${pageTitle}`
        this.logViewEventInternal(pageTitle, eventProperties, logAsActiveUser)
    }

    /**
     * Log a pageview, following the new event naming conventions
     *
     * @deprecated Use a TelemetryRecorder or TelemetryRecorderProvider from
     * src/telemetry instead.
     * @param eventName should be specific and human-readable in pascal case, e.g. "SearchResults" or "Blob" or "NewOrg"
     */
    public logPageView(eventName: string, eventProperties?: any, logAsActiveUser = true): void {
        // call to refresh the session
        this.resetSessionCookieExpiration()

        if (window.context?.userAgentIsBot || !eventName) {
            return
        }
        eventName = `${eventName}Viewed`
        this.logViewEventInternal(eventName, eventProperties, logAsActiveUser)
    }

    /**
     * Log a user action or event.
     * Event labels should be specific and follow a ${noun}${verb} structure in pascal case, e.g. "ButtonClicked" or "SignInInitiated"
     *
     * @deprecated Use a TelemetryRecorder or TelemetryRecorderProvider from
     * src/telemetry instead.
     * @param eventLabel the event name.
     * @param eventProperties event properties. These get logged to our database, but do not get
     * sent to our analytics systems. This may contain private info such as repository names or search queries.
     * @param publicArgument event properties that include only public information. Do NOT
     * include any private information, such as full URLs that may contain private repo names or
     * search queries. The contents of this parameter are sent to our analytics systems.
     */
    public log(eventLabel: string, eventProperties?: any, publicArgument?: any): void {
        // call to refresh the session
        this.resetSessionCookieExpiration()

        for (const listener of this.listeners) {
            listener(eventLabel)
        }
        if (window.context?.userAgentIsBot || !eventLabel) {
            return
        }

        this.logInternal(eventLabel, eventProperties, publicArgument)

        // Use flag to ensure URL query params are only stripped once
        if (!this.hasStrippedQueryParameters) {
            handleQueryEvents(window.location.href)
            this.hasStrippedQueryParameters = true
        }
    }

    public logInternal(eventLabel: string, eventProperties?: any, publicArgument?: any): void {
        serverAdmin.trackAction(eventLabel, eventProperties, publicArgument)
        this.logToConsole(eventLabel, eventProperties, publicArgument)
    }

    private logToConsole(eventLabel: string, eventProperties?: any, publicArgument?: any): void {
        if (debugEventLoggingEnabled()) {
            logger.debug('%cEVENT %s', 'color: #aaa', eventLabel, eventProperties, publicArgument)
        }
    }

    // Insert ID is used to deduplicate events in Amplitude.
    // https://developers.amplitude.com/docs/http-api-v2#optional-keys
    public getInsertID(): string {
        return uuid.v4()
    }

    // Event ID is used to deduplicate events in Amplitude.
    // This is used in the case that multiple events with the same userID and timestamp
    // are sent. https://developers.amplitude.com/docs/http-api-v2#optional-keys
    public getEventID(): number {
        this.eventID++
        return this.eventID
    }

    public getClient(): string {
        if (window.context?.sourcegraphDotComMode) {
            return EventClient.DOTCOM_WEB
        }
        return EventClient.SERVER_WEB
    }

    // Grabs and sets the deviceSessionID to renew the session expiration
    // Returns TRUE if successful, FALSE if deviceSessionID cannot be stored
    private resetSessionCookieExpiration(): boolean {
        // Function getDeviceSessionID calls cookie.set() to refresh the expiry
        const deviceSessionID = this.user.deviceSessionID
        if (!deviceSessionID || deviceSessionID === '') {
            return false
        }
        return true
    }

    public addEventLogListener(callback: (eventName: string) => void): () => void {
        this.listeners.add(callback)
        return () => this.listeners.delete(callback)
    }
}

/**
 * @deprecated Use a TelemetryRecorder or TelemetryRecorderProvider from
 * src/telemetry instead.
 */
export const eventLogger = new EventLogger()

export function debugEventLoggingEnabled(): boolean {
    return !!localStorage && localStorage.getItem('eventLogDebug') === 'true'
}

export function setDebugEventLoggingEnabled(enabled: boolean): void {
    if (localStorage) {
        localStorage.setItem('eventLogDebug', String(enabled))
    }
}

/**
 * Log events associated with URL query string parameters, and remove those parameters as necessary
 * Note that this is a destructive operation (it changes the page URL and replaces browser state) by
 * calling stripURLParameters
 */
function handleQueryEvents(url: string): void {
    const parsedUrl = new URL(url)
    if (parsedUrl.searchParams.has('signup')) {
        const args = { serviceType: parsedUrl.searchParams.get('signup') || '' }
        eventLogger.logInternal(EventName.SIGNUP_COMPLETED, args, args)
    }

    if (parsedUrl.searchParams.has('signin')) {
        const args = { serviceType: parsedUrl.searchParams.get('signin') || '' }
        eventLogger.logInternal(EventName.SINGIN_COMPLETED, args, args)
    }

    stripURLParameters(url, ['utm_campaign', 'utm_source', 'utm_medium', 'signup', 'signin'])
}

/**
 * Get pageview-specific event properties from URL query string parameters
 */
function pageViewQueryParameters(url: string): UTMMarker {
    const parsedUrl = new URL(url)

    const utmSource = parsedUrl.searchParams.get('utm_source')
    const utmCampaign = parsedUrl.searchParams.get('utm_campaign')
    const utmMedium = parsedUrl.searchParams.get('utm_medium')

    const utmProps: UTMMarker = {
        utm_campaign: utmCampaign || undefined,
        utm_source: utmSource || undefined,
        utm_medium: utmMedium || undefined,
        utm_term: parsedUrl.searchParams.get('utm_term') || undefined,
        utm_content: parsedUrl.searchParams.get('utm_content') || undefined,
    }

    if (utmSource === 'saved-search-email') {
        eventLogger.log('SavedSearchEmailClicked')
    } else if (utmSource === 'saved-search-slack') {
        eventLogger.log('SavedSearchSlackClicked')
    } else if (utmSource === 'code-monitoring-email') {
        eventLogger.log('CodeMonitorEmailLinkClicked')
    } else if (utmSource === 'hubspot' && utmCampaign?.match(/^cloud-onboarding-email(.*)$/)) {
        eventLogger.log('UTMCampaignLinkClicked', utmProps, utmProps)
    } else if (
        [
            'safari-extension',
            'firefox-extension',
            'chrome-extension',
            'phabricator-integration',
            'bitbucket-integration',
            'gitlab-integration',
        ].includes(utmSource ?? '')
    ) {
        eventLogger.log('UTMCodeHostIntegration', utmProps, utmProps)
    } else if (utmMedium === 'VSCODE' && utmCampaign === 'vsce-sign-up') {
        eventLogger.log('VSCODESignUpLinkClicked', utmProps, utmProps)
    }

    return utmProps
}
