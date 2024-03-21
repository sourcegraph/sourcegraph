import type * as Comlink from 'comlink'
import * as uuid from 'uuid'

import { EventSource, type Event as EventType } from '@sourcegraph/shared/src/graphql-operations'

import { version } from '../../../package.json'
import type { ExtensionCoreAPI } from '../../contract'
import { ANONYMOUS_USER_ID_KEY } from '../../settings/LocalStorageService'

import type { VsceTelemetryService } from './telemetryService'

// Event Logger for VS Code Extension
export class EventLogger implements VsceTelemetryService {
    private anonymousUserID = ''
    private evenSourceType = EventSource.BACKEND || EventSource.IDEEXTENSION
    private eventID = 0
    private listeners: Set<(eventName: string) => void> = new Set()
    private vsceAPI: Comlink.Remote<ExtensionCoreAPI>
    private newInstall = false
    private editorInfo = { editor: 'vscode', version }

    constructor(extensionAPI: Comlink.Remote<ExtensionCoreAPI>) {
        this.vsceAPI = extensionAPI
        this.initializeLogParameters()
            .then(() => {})
            .catch(() => {})
    }

    /**
     * @deprecated use logPageView instead
     *
     * Log a pageview event (by sending it to the server).
     */
    public logViewEvent(pageTitle: string, eventProperties?: any, publicArgument?: any, url?: string): void {
        if (pageTitle) {
            this.tracker(
                `View${pageTitle}`,
                { ...eventProperties, ...this.editorInfo },
                { ...publicArgument, ...this.editorInfo },
                url
            )
        }
    }

    /**
     * Log a pageview.
     * Page titles should be specific and human-readable in pascal case, e.g. "SearchResults" or "Blob" or "NewOrg"
     */
    public logPageView(eventName: string, eventProperties?: any, publicArgument?: any, url?: string): void {
        if (eventName) {
            this.tracker(
                `${eventName}Viewed`,
                { ...eventProperties, ...this.editorInfo },
                { ...publicArgument, ...this.editorInfo },
                url
            )
        }
    }

    /**
     * Log a user action or event.
     * Event labels should be specific and follow a ${noun}${verb} structure in pascal case, e.g. "ButtonClicked" or "SignInInitiated"
     *
     * @param eventLabel: the event name.
     * @param eventProperties: event properties. These get logged to our database, but do not get
     * sent to our analytics systems. This may contain private info such as repository names or search queries.
     * @param publicArgument: event properties that include only public information. Do NOT
     * include any private information, such as full URLs that may contain private repo names or
     * search queries. The contents of this parameter are sent to our analytics systems.
     */
    public log(eventLabel: string, eventProperties?: any, publicArgument?: any, uri?: string): void {
        if (!eventLabel) {
            return
        }
        switch (eventLabel) {
            case 'DynamicFilterClicked': {
                eventLabel = 'VSCESidebarDynamicFiltersClick'
                break
            }
            case 'SearchSnippetClicked': {
                eventLabel = 'VSCESidebarRepositoriesClick'
                break
            }
            case 'SearchReferenceOpened': {
                eventLabel = 'VSCESidebarSearchReferenceClick'
                break
            }
        }
        for (const listener of this.listeners) {
            listener(eventLabel)
        }
        this.tracker(
            eventLabel,
            { ...eventProperties, ...this.editorInfo },
            { ...publicArgument, ...this.editorInfo },
            uri
        )
    }

    /**
     * Gets the anonymous user ID and cohort ID of the user from VSCE storage utility.
     * If user doesn't have an anonymous user ID yet, a new one is generated
     * And a new ide install event will be logged
     */
    private async initializeLogParameters(): Promise<void> {
        let anonymousUserID = await this.vsceAPI.getLocalStorageItem(ANONYMOUS_USER_ID_KEY)
        const source = await this.vsceAPI.getEventSource
        if (!anonymousUserID) {
            anonymousUserID = uuid.v4()
            this.newInstall = true
            await this.vsceAPI.setLocalStorageItem(ANONYMOUS_USER_ID_KEY, anonymousUserID)
        }
        this.anonymousUserID = anonymousUserID
        this.evenSourceType = source
        if (this.newInstall) {
            this.log('IDEInstalled')
            this.newInstall = false
        }
    }

    /**
     * Get the anonymous identifier for this user (used to allow site admins
     * on a Sourcegraph instance to see a count of unique users on a daily,
     * weekly, and monthly basis).
     */
    public getAnonymousUserID(): string {
        return this.anonymousUserID
    }

    /**
     * Regular instance version format: 3.38.2
     * Insider version format: 134683_2022-03-02_5188fes0101
     */
    public getEventSourceType(): EventSource {
        return this.evenSourceType
    }

    /**
     * Event ID is used to deduplicate events in Amplitude.
     * This is used in the case that multiple events with the same userID and timestamp
     * are sent. https://developers.amplitude.com/docs/http-api-v2#optional-keys
     */
    public getEventID(): number {
        this.eventID++
        return this.eventID
    }

    public addEventLogListener(callback: (eventName: string) => void): () => void {
        this.listeners.add(callback)
        return () => this.listeners.delete(callback)
    }

    public tracker(eventName: string, eventProperties?: unknown, publicArgument?: unknown, uri?: string): void {
        const userEventVariables: EventType = {
            event: eventName,
            userCookieID: this.getAnonymousUserID(),
            referrer: 'VSCE',
            url: uri || '',
            source: this.getEventSourceType(),
            argument: eventProperties ? JSON.stringify(eventProperties) : null,
            publicArgument: JSON.stringify(publicArgument),
            deviceID: this.getAnonymousUserID(),
            eventID: this.getEventID(),
        }
        this.vsceAPI
            .logEvents(userEventVariables)
            .then(() => {})
            .catch(error => console.log(error))
    }
}
