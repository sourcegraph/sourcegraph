import { type Cookies, defaultCookies } from './cookies'

const FIRST_SOURCE_URL_KEY = 'sourcegraphSourceUrl'
const LAST_SOURCE_URL_KEY = 'sourcegraphRecentSourceUrl'
const ORIGINAL_REFERRER_KEY = 'originalReferrer'
const SESSION_REFERRER_KEY = 'sessionReferrer'
const SESSION_FIRST_URL_KEY = 'sessionFirstUrl'

/**
 * Prefer the global sessionTracker instance.
 */
export class SessionTracker {
    /**
     * Session tracking is only done in Sourcegraph.com, where cookie values are set in Google Tag Manager
     * to ensure consistency across all public Sourcegraph-managed properties (e.g. marketing sites, blog, etc.)
     */
    private isSourcegraphDotComMode = window.context?.sourcegraphDotComMode || false

    private originalReferrer: string
    private sessionReferrer: string
    private sessionFirstURL: string
    private firstSourceURL: string
    private lastSourceURL: string

    constructor(private cookies: Cookies = defaultCookies()) {
        this.originalReferrer = this.getOriginalReferrer()
        this.sessionReferrer = this.getSessionReferrer()
        this.sessionFirstURL = this.getSessionFirstURL()
        this.firstSourceURL = this.getFirstSourceURL()
        this.lastSourceURL = this.getLastSourceURL()
    }

    public getOriginalReferrer(): string {
        if (!this.isSourcegraphDotComMode) {
            return ''
        }
        // This cookie is set in Google Tag manager.
        this.originalReferrer = this.originalReferrer || this.cookies.get(ORIGINAL_REFERRER_KEY) || document.referrer

        return this.originalReferrer
    }

    public getSessionReferrer(): string {
        if (!this.isSourcegraphDotComMode) {
            return ''
        }
        // This cookie is set in Google Tag manager.
        this.sessionReferrer = this.sessionReferrer || this.cookies.get(SESSION_REFERRER_KEY) || document.referrer

        return this.sessionReferrer
    }

    public getSessionFirstURL(): string {
        if (!this.isSourcegraphDotComMode) {
            return ''
        }
        // This cookie is set in Google Tag manager.
        this.sessionFirstURL = this.sessionFirstURL || this.cookies.get(SESSION_FIRST_URL_KEY) || location.href

        return this.sessionFirstURL
    }

    public getFirstSourceURL(): string {
        if (!this.isSourcegraphDotComMode) {
            return ''
        }
        // This cookie is set in Google Tag manager.
        this.firstSourceURL = this.firstSourceURL || this.cookies.get(FIRST_SOURCE_URL_KEY) || location.href

        return this.firstSourceURL
    }

    public getLastSourceURL(): string {
        if (!this.isSourcegraphDotComMode) {
            return ''
        }

        // This cookie is set in Google Tag manager.
        this.lastSourceURL = this.lastSourceURL || this.cookies.get(LAST_SOURCE_URL_KEY) || location.href

        return this.lastSourceURL
    }

    public getReferrer(): string {
        if (this.isSourcegraphDotComMode) {
            return document.referrer
        }
        return ''
    }
}

/**
 * Configures and loads cookie properties for session tracking purposes.
 */
export const sessionTracker = new SessionTracker()
