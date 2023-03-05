import * as os from 'os'
import * as path from 'path'

import realPercySnapshot from '@percy/puppeteer'
import delay from 'delay'
import expect from 'expect'
import * as jsonc from 'jsonc-parser'
import { escapeRegExp } from 'lodash'
import { readFile } from 'mz/fs'
import puppeteer, {
    PageEventObject,
    Page,
    Serializable,
    LaunchOptions,
    ConsoleMessage,
    Target,
    BrowserLaunchArgumentOptions,
    BrowserConnectOptions,
} from 'puppeteer'
import { from, fromEvent, merge, Subscription } from 'rxjs'
import { filter, map, concatAll, mergeMap, mergeAll, takeUntil } from 'rxjs/operators'
import { Key } from 'ts-key-enum'

import { isDefined, logger } from '@sourcegraph/common'
import { dataOrThrowErrors, gql, GraphQLResult } from '@sourcegraph/http-client'

import {
    ExternalServiceKind,
    ExternalServicesForTestsResult,
    OverwriteSettingsForTestsResult,
    RepositoryNameForTestsResult,
    SiteForTestsResult,
    UpdateSiteConfigurationForTestsResult,
    UserSettingsForTestsResult,
} from '../graphql-operations'
import { Settings } from '../settings/settings'

import { getConfig } from './config'
import { formatPuppeteerConsoleMessage } from './console'
import { readEnvironmentBoolean, retry } from './utils'

/**
 * Returns a Promise for the next emission of the given event on the given Puppeteer page.
 */
export const oncePageEvent = <E extends keyof PageEventObject>(page: Page, eventName: E): Promise<PageEventObject[E]> =>
    new Promise(resolve => page.once(eventName, resolve))

export const extractStyles = (page: puppeteer.Page): Promise<string> =>
    page.evaluate(() =>
        [...document.styleSheets].reduce(
            (styleSheetRules, styleSheet) =>
                styleSheetRules.concat(
                    [...styleSheet.cssRules].reduce((rules, rule) => rules.concat(rule.cssText), '')
                ),
            ''
        )
    )

interface CommonSnapshotOptions {
    widths?: number[]
    minHeight?: number
    percyCSS?: string
    enableJavaScript?: boolean
    devicePixelRatio?: number
    scope?: string
}

export const percySnapshot = async (
    page: puppeteer.Page,
    name: string,
    options: CommonSnapshotOptions = {}
): Promise<void> => {
    if (!readEnvironmentBoolean({ variable: 'PERCY_ON', defaultValue: false })) {
        return Promise.resolve()
    }

    const pageStyles = await extractStyles(page)
    return realPercySnapshot(page, name, { ...options, percyCSS: pageStyles.concat(options.percyCSS || '') })
}

export const BROWSER_EXTENSION_DEV_ID = 'bmfbcejdknlknpncfpeloejonjoledha'

/**
 * Specifies how to select the content of the element. No
 * single method works in all cases:
 *
 * - Meta+A doesn't work in input boxes https://github.com/GoogleChrome/puppeteer/issues/1313
 * - selectall doesn't work in the Monaco editor
 */
type SelectTextMethod = 'selectall' | 'keyboard'

/**
 * Specifies how to enter text. Typing is preferred in cases where it's important to test
 * the process of manually typing out the text to enter. Pasting is preferred in cases
 * where typing would be too slow or we explicitly want to test paste behavior.
 */
type EnterTextMethod = 'type' | 'paste'

interface PageFuncOptions {
    timeout?: number
}

interface FindElementOptions {
    /**
     * Filter candidate elements to those with the specified CSS selector
     */
    selector?: string

    /**
     * Log the XPath quer(y|ies) used to find the element.
     */
    log?: boolean

    /**
     * Specifies how exact the search criterion is.
     */
    fuzziness?: 'exact' | 'prefix' | 'space-prefix' | 'contains'

    /**
     * Specifies whether to wait (and how long) for the element to appear.
     */
    wait?: PageFuncOptions | boolean
}

function findElementRegexpStrings(
    text: string,
    { fuzziness = 'space-prefix' }: Pick<FindElementOptions, 'fuzziness'>
): string[] {
    //  Escape regexp special chars. Copied from
    //  https://developer.mozilla.org/en-US/docs/Web/JavaScript/Guide/Regular_Expressions
    const escapedText = escapeRegExp(text)
    const regexps = [`^${escapedText}$`]
    if (fuzziness === 'exact') {
        return regexps
    }
    regexps.push(`^${escapedText}\\b`)
    if (fuzziness === 'prefix') {
        return regexps
    }
    regexps.push(`^\\s+${escapedText}$`) // Still prefer exact
    regexps.push(`^\\s+${escapedText}\\b`)
    if (fuzziness === 'space-prefix') {
        return regexps
    }
    regexps.push(escapedText)
    return regexps
}

function findElementMatchingRegexps(tag: string, regexps: string[]): HTMLElement | null {
    for (const regexpString of regexps) {
        const regexp = new RegExp(regexpString)
        for (const element of document.querySelectorAll<HTMLElement>(tag)) {
            if (!element.offsetParent) {
                // Ignore hidden elements
                continue
            }
            if (element.textContent?.match(regexp)) {
                return element
            }
        }
    }
    return null
}

function getDebugExpressionFromRegexp(tag: string, regexp: string): string {
    return `Array.from(document.querySelectorAll(${JSON.stringify(
        tag
    )})).filter(e => e.innerText && e.innerText.match(/${regexp}/))`
}

// Console logs with these keywords will be removed from the console output.
const MUTE_CONSOLE_KEYWORDS = [
    '[webpack-dev-server]',
    'Download the React DevTools',
    '[HMR]',
    '[WDS]',
    'Warning: componentWillReceiveProps has been renamed',
    'Download the Apollo DevTools',
    'Compiled in DEBUG mode',
    'Cache data may be lost',
    'Failed to decode downloaded font',
    'OTS parsing error',
]

export class Driver {
    /** The pages that were visited since the creation of the driver. */
    public visitedPages: Readonly<URL>[] = []

    public sourcegraphBaseUrl: string
    public browserType: 'chrome'
    private keepBrowser: boolean
    private subscriptions = new Subscription()

    constructor(public browser: puppeteer.Browser, public page: puppeteer.Page, options: DriverOptions) {
        this.sourcegraphBaseUrl = options.sourcegraphBaseUrl
        this.browserType = options.browser ?? 'chrome'
        this.keepBrowser = !!options.keepBrowser

        // Record visited pages
        this.subscriptions.add(
            merge(fromEvent<Target>(browser, 'targetchanged'), fromEvent<Target>(browser, 'targetcreated'))
                .pipe(filter(target => target.type() === 'page'))
                .subscribe(target => {
                    this.visitedPages.push(new URL(target.url()))
                })
        )

        // Log browser console
        if (options.logBrowserConsole) {
            this.subscriptions.add(
                merge(
                    from(browser.pages()).pipe(mergeAll()),
                    fromEvent<Target>(browser, 'targetcreated').pipe(
                        mergeMap(target => target.page()),
                        filter(isDefined)
                    )
                )
                    .pipe(
                        mergeMap(page =>
                            fromEvent<ConsoleMessage>(page, 'console').pipe(
                                filter(
                                    message =>
                                        // These requests are expected to fail, we use them to check if the browser extension is installed.
                                        message.location().url !== 'chrome-extension://invalid/' &&
                                        // Ignore React development build warnings.
                                        !message.text().startsWith('Warning: ') &&
                                        !MUTE_CONSOLE_KEYWORDS.some(keyword => message.text().includes(keyword))
                                ),
                                map(message =>
                                    // Immediately format remote handles to strings, but maintain order.
                                    formatPuppeteerConsoleMessage(page, message)
                                ),
                                concatAll(),
                                takeUntil(fromEvent(page, 'close'))
                            )
                        )
                    )
                    .subscribe(formattedLine => logger.log(formattedLine))
            )
        }
    }

    public async ensureSignedIn({
        username,
        password,
        email,
    }: {
        username: string
        password: string
        email?: string
    }): Promise<void> {
        /**
         * Waiting here for all redirects is not stable. We try to use the signin form first because
         * it's the most frequent use-case. If we cannot find its selector we fall back to the signup form.
         */
        await this.page.goto(this.sourcegraphBaseUrl)

        /**
         * In case a user is not authenticated, and site-init is NOT required, one redirect happens:
         * 1. Redirect to /sign-in?returnTo=%2F
         */
        try {
            logger.log('Trying to use the signin form...')
            await this.page.waitForSelector('.test-signin-form', { timeout: 10000 })
            await this.page.type('input', username)
            await this.page.type('input[name=password]', password)
            // TODO(uwedeportivo): see comment above, same reason
            await delay(1000)
            await this.page.click('button[type=submit]')
            await this.page.waitForNavigation({ timeout: 300000 })
        } catch (error) {
            /**
             * In case a user is not authenticated, and site-init is required, two redirects happen:
             * 1. Redirect to /sign-in?returnTo=%2F
             * 2. Redirect to /site-admin/init
             */
            if (error.message.includes('waiting for selector `.test-signin-form` failed')) {
                logger.log('Failed to use the signin form. Trying the signup form...')

                await this.page.waitForSelector('.test-signup-form')
                if (email) {
                    await this.page.type('input[name=email]', email)
                }
                await this.page.type('input[name=username]', username)
                await this.page.type('input[name=password]', password)
                await this.page.waitForSelector('button[type=submit]:not(:disabled)')
                // TODO(uwedeportivo): investigate race condition between puppeteer clicking this very fast and
                // background gql client request fetching ViewerSettings. this race condition results in the gql request
                // "winning" sometimes without proper credentials which confuses the login state machine and it navigates
                // you back to the login page
                await delay(1000)
                await this.page.click('button[type=submit]')
                await this.page.waitForNavigation({ timeout: 300000 })
            } else {
                throw error
            }
        }
    }

    /**
     * Navigates to the Sourcegraph browser extension page.
     */
    public async openBrowserExtensionPage(page: 'options' | 'after_install'): Promise<void> {
        await this.page.goto(`chrome-extension://${BROWSER_EXTENSION_DEV_ID}/${page}.html`)
    }

    /**
     * Navigates to the Sourcegraph browser extension options page and sets the sourcegraph URL.
     */
    public async setExtensionSourcegraphUrl(): Promise<void> {
        await this.openBrowserExtensionPage('options')
        await this.page.waitForSelector('.test-sourcegraph-url')
        await this.replaceText({ selector: '.test-sourcegraph-url', newText: this.sourcegraphBaseUrl })
        await this.page.keyboard.press(Key.Enter)
        await this.page.waitForSelector('.test-valid-sourcegraph-url-feedback')
    }

    /**
     * Sets 'Enable click to go to definition' option flag value.
     */
    public async setClickGoToDefOptionFlag(isEnabled: boolean): Promise<void> {
        await this.openBrowserExtensionPage('options')
        const toggleAdvancedSettingsButton = await this.page.waitForSelector('.test-toggle-advanced-settings-button')
        await toggleAdvancedSettingsButton?.click()
        const checkbox = await this.findElementWithText('Enable click to go to definition')
        if (!checkbox) {
            throw new Error("'Enable click to go to definition' checkbox not found.")
        }
        const isChecked = await checkbox.$eval('input', input => (input as HTMLInputElement).checked)
        if (isEnabled !== isChecked) {
            await checkbox.click()
        }
    }

    public async close(): Promise<void> {
        this.subscriptions.unsubscribe()
        if (!this.keepBrowser) {
            await this.browser.close()
        }
        logger.log(
            '\n  Visited routes:\n' +
                [
                    ...new Set(
                        this.visitedPages
                            .filter(url => url.href.startsWith(this.sourcegraphBaseUrl))
                            .map(url => `    ${url.pathname}`)
                    ),
                ].join('\n')
        )
    }

    public async newPage(): Promise<void> {
        this.page = await this.browser.newPage()
    }

    public async selectAll(method: SelectTextMethod = 'selectall'): Promise<void> {
        switch (method) {
            case 'selectall': {
                await this.page.evaluate(() => document.execCommand('selectall', false))
                break
            }
            case 'keyboard': {
                const modifier = os.platform() === 'darwin' ? Key.Meta : Key.Control
                await this.page.keyboard.down(modifier)
                await this.page.keyboard.press('a')
                await this.page.keyboard.up(modifier)
                break
            }
        }
    }

    public async enterText(method: EnterTextMethod = 'type', text: string): Promise<void> {
        // Pasting does not work on macOS. See:  https://github.com/GoogleChrome/puppeteer/issues/1313
        method = os.platform() === 'darwin' ? 'type' : method
        switch (method) {
            case 'type':
                await this.page.keyboard.type(text)
                break
            case 'paste':
                await this.paste(text)
                break
        }
    }

    public async replaceText({
        selector,
        newText,
        selectMethod = 'selectall',
        enterTextMethod = 'type',
    }: {
        selector: string
        newText: string
        selectMethod?: SelectTextMethod
        enterTextMethod?: EnterTextMethod
    }): Promise<void> {
        // The Monaco editor sometimes detaches nodes from the DOM, causing
        // `click()` to fail unpredictably.
        await retry(async () => {
            await this.page.waitForSelector(selector)
            await this.page.click(selector)
        })
        await this.selectAll(selectMethod)
        await this.page.keyboard.press(Key.Backspace)
        await this.enterText(enterTextMethod, newText)
    }

    public async acceptNextDialog(): Promise<void> {
        const dialog = await oncePageEvent(this.page, 'dialog')
        await dialog.accept()
    }

    public async ensureHasExternalService({
        kind,
        displayName,
        config,
        ensureRepos,
        alwaysCloning,
    }: {
        kind: ExternalServiceKind
        displayName: string
        config: string
        ensureRepos?: string[]
        alwaysCloning?: string[]
    }): Promise<void> {
        // Use the graphQL API to query external services on the instance.
        const { externalServices } = dataOrThrowErrors(
            await this.makeGraphQLRequest<ExternalServicesForTestsResult>({
                request: gql`
                    query ExternalServicesForTests {
                        externalServices(first: 1) {
                            totalCount
                        }
                    }
                `,
                variables: {},
            })
        )
        // Delete existing external services if there are any.
        if (externalServices.totalCount !== 0) {
            await this.page.goto(this.sourcegraphBaseUrl + '/site-admin/external-services')
            await this.page.waitForSelector('[data-testid="filtered-connection"]')
            await this.page.waitForSelector('[data-testid="filtered-connection-loader"]', { hidden: true })

            // Matches buttons for deleting external services named ${displayName}.
            const deleteButtonSelector = `[data-test-external-service-name="${displayName}"] .test-delete-external-service-button`
            if (await this.page.$(deleteButtonSelector)) {
                await Promise.all([this.acceptNextDialog(), this.page.click(deleteButtonSelector)])
            }
        }

        // Navigate to the add external service page.
        logger.log('Adding external service of kind', kind)
        await this.page.goto(this.sourcegraphBaseUrl + '/site-admin/external-services/new')
        await this.page.waitForSelector(`[data-test-external-service-card-link="${kind.toUpperCase()}"]`, {
            visible: true,
        })
        await this.page.evaluate((selector: string) => {
            const element = document.querySelector<HTMLElement>(selector)
            if (!element) {
                throw new Error(`Could not find element to click on for selector ${selector}`)
            }
            element.click()
        }, `[data-test-external-service-card-link="${kind.toUpperCase()}"]`)
        await this.replaceText({
            selector: '#test-external-service-form-display-name',
            newText: displayName,
        })

        await this.page.waitForSelector('.test-external-service-editor .monaco-editor')
        // Type in a new external service configuration.
        await this.replaceText({
            selector: '.test-external-service-editor .monaco-editor .view-line',
            newText: config,
            selectMethod: 'keyboard',
        })
        await Promise.all([this.page.waitForNavigation(), this.page.click('.test-add-external-service-button')])

        if (ensureRepos) {
            // Clone the repositories
            for (const slug of ensureRepos) {
                await this.page.goto(
                    this.sourcegraphBaseUrl + `/site-admin/repositories?filter=cloned&query=${encodeURIComponent(slug)}`
                )
                await this.page.waitForSelector(`.repository-node[data-test-repository='${slug}']`, {
                    visible: true,
                    timeout: 300000,
                })
                // Workaround for https://github.com/sourcegraph/sourcegraph/issues/5286
                await this.page.goto(`${this.sourcegraphBaseUrl}/${slug}`)
            }
        }

        if (alwaysCloning) {
            for (const slug of alwaysCloning) {
                await this.page.goto(
                    this.sourcegraphBaseUrl +
                        `/site-admin/repositories?filter=cloning&query=${encodeURIComponent(slug)}`
                )
                await this.page.waitForSelector(`.repository-node[data-test-repository='${slug}']`, { visible: true })
                // Workaround for https://github.com/sourcegraph/sourcegraph/issues/5286
                await this.page.goto(`${this.sourcegraphBaseUrl}/${slug}`)
            }
        }
    }

    public async paste(value: string): Promise<void> {
        await this.page.evaluate(async (value: string) => {
            await navigator.clipboard.writeText(value)
        }, value)
        const modifier = os.platform() === 'darwin' ? Key.Meta : Key.Control
        await this.page.keyboard.down(modifier)
        await this.page.keyboard.press('v')
        await this.page.keyboard.up(modifier)
    }

    public async assertWindowLocation(location: string, isAbsolute = false): Promise<any> {
        const url = isAbsolute ? location : this.sourcegraphBaseUrl + location
        await retry(async () => {
            expect(await this.page.evaluate(() => window.location.href)).toEqual(url)
        })
    }

    public async assertWindowLocationPrefix(locationPrefix: string, isAbsolute = false): Promise<any> {
        const prefix = isAbsolute ? locationPrefix : this.sourcegraphBaseUrl + locationPrefix
        await retry(async () => {
            const location: string = await this.page.evaluate(() => window.location.href)
            expect(location.startsWith(prefix)).toBeTruthy()
        })
    }

    public async assertStickyHighlightedToken(label: string): Promise<void> {
        await this.page.waitForSelector('.selection-highlight-sticky', { visible: true }) // make sure matched token is highlighted
        await retry(async () =>
            expect(
                await this.page.evaluate(() => document.querySelector('.selection-highlight-sticky')!.textContent)
            ).toEqual(label)
        )
    }

    public async assertAllHighlightedTokens(label: string): Promise<void> {
        const highlightedTokens = await this.page.evaluate(() =>
            [...document.querySelectorAll('.selection-highlight')].map(element => element.textContent || '')
        )
        expect(highlightedTokens.every(txt => txt === label)).toBeTruthy()
    }

    public async assertNonemptyLocalRefs(): Promise<void> {
        // verify active group is references
        await this.page.waitForXPath(
            "//*[contains(@class, 'panel')]//*[contains(@tabindex, '0')]//*[contains(text(), 'References')]"
        )
        // verify there are some references
        await this.page.waitForSelector('[data-testid="panel-tabs-content"] [data-testid="file-match-children-item"]', {
            visible: true,
        })
    }

    private async makeRequest<T = void>({ url, init }: { url: string; init: RequestInit & Serializable }): Promise<T> {
        const handle = await this.page.evaluateHandle(
            (url, init) => fetch(url, init).then(response => response.json()),
            url,
            init
        )
        return (await handle.jsonValue()) as T
    }

    private async makeGraphQLRequest<T, V = object>({
        request,
        variables,
    }: {
        request: string
        variables: V
    }): Promise<GraphQLResult<T>> {
        const nameMatch = request.match(/^\s*(?:query|mutation)\s+(\w+)/)
        const xhrHeaders =
            (await this.page.evaluate(
                sourcegraphBaseUrl =>
                    location.href.startsWith(sourcegraphBaseUrl) && (window as any).context.xhrHeaders,
                this.sourcegraphBaseUrl
            )) || {}
        const response = await this.makeRequest<GraphQLResult<T>>({
            url: `${this.sourcegraphBaseUrl}/.api/graphql${nameMatch ? '?' + nameMatch[1] : ''}`,
            init: {
                method: 'POST',
                body: JSON.stringify({ query: request, variables }),
                headers: {
                    ...xhrHeaders,
                    Accept: 'application/json',
                    'Content-Type': 'application/json',
                },
            },
        })
        return response
    }

    public async getRepository(name: string): Promise<RepositoryNameForTestsResult['repository']> {
        const response = await this.makeGraphQLRequest<RepositoryNameForTestsResult>({
            request: gql`
                query RepositoryNameForTests($name: String!) {
                    repository(name: $name) {
                        id
                    }
                }
            `,
            variables: { name },
        })
        const { repository } = dataOrThrowErrors(response)
        if (!repository) {
            throw new Error(`repository not found: ${name}`)
        }
        return repository
    }

    public async setConfig(
        path: jsonc.JSONPath,
        editFunction: (oldValue: jsonc.Node | undefined) => any
    ): Promise<void> {
        const currentConfigResponse = await this.makeGraphQLRequest<SiteForTestsResult>({
            request: gql`
                query SiteForTests {
                    site {
                        id
                        configuration {
                            id
                            effectiveContents
                            validationMessages
                        }
                    }
                }
            `,
            variables: {},
        })
        const { site } = dataOrThrowErrors(currentConfigResponse)
        const currentConfig = site.configuration.effectiveContents
        const newConfig = modifyJSONC(currentConfig, path, editFunction)
        const updateConfigResponse = await this.makeGraphQLRequest<UpdateSiteConfigurationForTestsResult>({
            request: gql`
                mutation UpdateSiteConfigurationForTests($lastID: Int!, $input: String!) {
                    updateSiteConfiguration(lastID: $lastID, input: $input)
                }
            `,
            variables: { lastID: site.configuration.id, input: newConfig },
        })
        dataOrThrowErrors(updateConfigResponse)
    }

    public async ensureHasCORSOrigin({ corsOriginURL }: { corsOriginURL: string }): Promise<void> {
        await this.setConfig(['corsOrigin'], oldCorsOrigin => {
            const urls = oldCorsOrigin ? (oldCorsOrigin.value as string).split(' ') : []
            return (urls.includes(corsOriginURL) ? urls : [...urls, corsOriginURL]).join(' ')
        })
    }

    public async resetUserSettings(): Promise<void> {
        return this.setUserSettings({})
    }

    public async setUserSettings<S extends Settings>(settings: S): Promise<void> {
        const currentSettingsResponse = await this.makeGraphQLRequest<UserSettingsForTestsResult>({
            request: gql`
                query UserSettingsForTests {
                    currentUser {
                        id
                        latestSettings {
                            id
                            contents
                        }
                    }
                }
            `,
            variables: {},
        })

        const { currentUser } = dataOrThrowErrors(currentSettingsResponse)
        if (!currentUser) {
            throw new Error('no currentUser')
        }

        const updateConfigResponse = await this.makeGraphQLRequest<OverwriteSettingsForTestsResult>({
            request: gql`
                mutation OverwriteSettingsForTests($subject: ID!, $lastID: Int, $contents: String!) {
                    settingsMutation(input: { subject: $subject, lastID: $lastID }) {
                        overwriteSettings(contents: $contents) {
                            empty {
                                alwaysNil
                            }
                        }
                    }
                }
            `,
            variables: {
                contents: JSON.stringify(settings),
                subject: currentUser.id,
                lastID: currentUser.latestSettings ? currentUser.latestSettings.id : null,
            },
        })
        dataOrThrowErrors(updateConfigResponse)
    }

    /**
     * Finds the first HTML element matching the text using the regular expressions returned in
     * {@link findElementRegexpStrings}.
     *
     * @param options specifies additional parameters for finding the element. If you want to wait
     * until the element appears, specify the `wait` field (which can contain additional inner
     * options for how long to wait).
     */
    public async findElementWithText(
        text: string,
        options: FindElementOptions & { action?: 'click' } = {}
    ): Promise<puppeteer.ElementHandle<Element>> {
        const { selector: tagName, fuzziness, wait } = options
        const tag = tagName || '*'
        const regexps = findElementRegexpStrings(text, { fuzziness })

        const notFoundError = (underlying?: Error): Error => {
            const debuggingExpressions = regexps.map(regexp => getDebugExpressionFromRegexp(tag, regexp))
            return new Error(
                `Could not find element with text ${JSON.stringify(text)}, options: ${JSON.stringify(options)}` +
                    (underlying ? `. Underlying error was: ${JSON.stringify(underlying.message)}.` : '') +
                    ` Debug expressions: ${debuggingExpressions.join('\n')}`
            )
        }

        return retry(
            async () => {
                const handlePromise = wait
                    ? this.page
                          .waitForFunction(
                              findElementMatchingRegexps,
                              typeof wait === 'object' ? wait : {},
                              tag,
                              regexps
                          )
                          .catch(error => {
                              throw notFoundError(error)
                          })
                    : this.page.evaluateHandle(findElementMatchingRegexps, tag, regexps)

                const element = (await handlePromise).asElement()
                if (!element) {
                    throw notFoundError()
                }

                if (options.action === 'click') {
                    await element.click()
                }
                return element
            },
            {
                retries: options.action === 'click' ? 3 : 0,
                minTimeout: 100,
                maxTimeout: 100,
                factor: 1,
                maxRetryTime: 500,
            }
        )
    }

    public async waitUntilURL(url: string, options: PageFuncOptions = {}): Promise<void> {
        await this.page.waitForFunction((url: string) => document.location.href === url, options, url)
    }
}

export function modifyJSONC(
    text: string,
    path: jsonc.JSONPath,
    editFunction: (oldValue: jsonc.Node | undefined) => any
): string | undefined {
    const tree = jsonc.parseTree(text)
    const old = tree ? jsonc.findNodeAtLocation(tree, path) : undefined
    return jsonc.applyEdits(
        text,
        jsonc.modify(text, path, editFunction(old), {
            formattingOptions: {
                eol: '\n',
                insertSpaces: true,
                tabSize: 2,
            },
        })
    )
}

interface DriverOptions extends LaunchOptions, BrowserConnectOptions, BrowserLaunchArgumentOptions {
    browser?: 'chrome'

    /** If true, load the Sourcegraph browser extension. */
    loadExtension?: boolean

    sourcegraphBaseUrl: string

    /** If not `false`, print browser console messages to stdout. */
    logBrowserConsole?: boolean

    /** If true, keep browser open when driver is closed */
    keepBrowser?: boolean
}

export async function createDriverForTest(options?: Partial<DriverOptions>): Promise<Driver> {
    const config = getConfig(
        'sourcegraphBaseUrl',
        'headless',
        'slowMo',
        'keepBrowser',
        'browser',
        'devtools',
        'windowWidth',
        'windowHeight'
    )

    // Apply defaults
    const resolvedOptions: DriverOptions = {
        ...config,
        ...options,
    }

    const { loadExtension } = resolvedOptions
    const args: string[] = ['--no-sandbox'] // https://stackoverflow.com/a/61278676
    const launchOptions: LaunchOptions & BrowserLaunchArgumentOptions & BrowserConnectOptions = {
        ignoreHTTPSErrors: true,
        ...resolvedOptions,
        args,
        defaultViewport: null,
        timeout: 300000,
    }

    // Chrome
    args.push(`--window-size=${config.windowWidth},${config.windowHeight}`)
    if (process.getuid() === 0) {
        // TODO don't run as root in CI
        logger.warn('Running as root, disabling sandbox')
        args.push('--no-sandbox', '--disable-setuid-sandbox')
    }
    if (loadExtension) {
        const chromeExtensionPath = path.resolve(__dirname, '..', '..', '..', 'browser', 'build', 'chrome')
        const manifest = JSON.parse(await readFile(path.resolve(chromeExtensionPath, 'manifest.json'), 'utf-8')) as {
            permissions: string[]
        }
        if (!manifest.permissions.includes('<all_urls>')) {
            throw new Error(
                'Browser extension was not built with permissions for all URLs.\nThis is necessary because permissions cannot be granted by e2e tests.\nTo fix, run `EXTENSION_PERMISSIONS_ALL_URLS=true pnpm run dev` inside the browser/ directory.'
            )
        }
        args.push(`--disable-extensions-except=${chromeExtensionPath}`, `--load-extension=${chromeExtensionPath}`)
    }

    const browser: puppeteer.Browser = await puppeteer.launch({ ...launchOptions })

    const page = await browser.newPage()

    return new Driver(browser, page, resolvedOptions)
}
