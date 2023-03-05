import classNames from 'classnames'
import { trimStart } from 'lodash'
import { createRoot } from 'react-dom/client'
import { defer, fromEvent, of } from 'rxjs'
import { distinctUntilChanged, filter, map, startWith } from 'rxjs/operators'
import { Omit } from 'utility-types'

import { AdjustmentDirection, PositionAdjuster } from '@sourcegraph/codeintellify'
import { LineOrPositionOrRange } from '@sourcegraph/common'
import { observeSystemIsLightTheme } from '@sourcegraph/shared/src/deprecated-theme-utils'
import { PlatformContext } from '@sourcegraph/shared/src/platform/context'
import { createURLWithUTM } from '@sourcegraph/shared/src/tracking/utm'
import {
    FileSpec,
    RepoSpec,
    ResolvedRevisionSpec,
    RevisionSpec,
    toAbsoluteBlobURL,
} from '@sourcegraph/shared/src/util/url'

import LogoSVG from '../../../../assets/img/sourcegraph-mark.svg'
import { background } from '../../../browser-extension/web-extension-api/runtime'
import { SourcegraphIconButton } from '../../components/SourcegraphIconButton'
import { fetchBlobContentLines } from '../../repo/backend'
import { getPlatformName } from '../../util/context'
import { querySelectorAllOrSelf, querySelectorOrSelf } from '../../util/dom'
import { CodeHost, MountGetter } from '../shared/codeHost'
import { CodeView, toCodeViewResolver } from '../shared/codeViews'
import { getSelectionsFromHash, observeSelectionsFromHash } from '../shared/util/selections'
import { ViewResolver } from '../shared/views'

import { diffDomFunctions, searchCodeSnippetDOMFunctions, singleFileDOMFunctions } from './domFunctions'
import { resolveDiffFileInfo, resolveFileInfo, resolveSnippetFileInfo } from './fileInfo'
import { getFileContainers, parseURL, getSelectorFor } from './util'

import styles from './codeHost.module.scss'

/**
 * Creates the mount element for the CodeViewToolbar on code views containing
 * a `.file-actions` element, for instance:
 * - A diff code view on a PR's files page, or a commit page
 * - An older GHE single file code view (newer GitHub.com code views use createFileLineContainerToolbarMount)
 */
export function createFileActionsToolbarMount(codeView: HTMLElement): HTMLElement {
    const className = 'github-file-actions-toolbar-mount'
    const existingMount = codeView.querySelector('.' + className) as HTMLElement
    if (existingMount) {
        return existingMount
    }

    const mountElement = document.createElement('div')
    mountElement.className = className

    const fileActions = codeView.querySelector('.file-actions')
    if (!fileActions) {
        throw new Error('Could not find GitHub file actions with selector .file-actions')
    }

    // Add a class to the .file-actions element, so that we can reliably match it in
    // stylesheets without bleeding CSS to other code hosts (GitLab also uses .file-actions elements).
    fileActions.classList.add('sg-github-file-actions')

    // Old GitHub Enterprise PR views have a "☑ show comments" text that we want to insert *after*
    const showCommentsElement = codeView.querySelector('.show-file-notes')
    if (showCommentsElement) {
        showCommentsElement.after(mountElement)
    } else {
        fileActions.prepend(mountElement)
    }

    return mountElement
}

const diffCodeView: Omit<CodeView, 'element'> = {
    dom: diffDomFunctions,
    getToolbarMount: createFileActionsToolbarMount,
    resolveFileInfo: resolveDiffFileInfo,
    getScrollBoundaries: codeView => {
        const fileHeader = codeView.querySelector<HTMLElement>('.file-header')
        if (!fileHeader) {
            throw new Error('Could not find .file-header element in GitHub PR code view')
        }
        return [fileHeader]
    },
}

const diffConversationCodeView: Omit<CodeView, 'element'> = {
    ...diffCodeView,
    getToolbarMount: undefined,
}

const singleFileCodeView: Omit<CodeView, 'element'> = {
    dom: singleFileDOMFunctions,
    getToolbarMount: createFileActionsToolbarMount,
    resolveFileInfo,
    getSelections: getSelectionsFromHash,
    observeSelections: observeSelectionsFromHash,
}

/**
 * Some code snippets get leading white space trimmed. This adjusts based on
 * this. See an example here https://github.com/sourcegraph/browser-extensions/issues/188.
 */
const getSnippetPositionAdjuster =
    (
        requestGraphQL: PlatformContext['requestGraphQL']
    ): PositionAdjuster<RepoSpec & RevisionSpec & FileSpec & ResolvedRevisionSpec> =>
    ({ direction, codeView, position }) =>
        fetchBlobContentLines({ ...position, requestGraphQL }).pipe(
            map(lines => {
                const codeElement = singleFileDOMFunctions.getCodeElementFromLineNumber(
                    codeView,
                    position.line,
                    position.part
                )
                if (!codeElement) {
                    throw new Error('(adjustPosition) could not find code element for line provided')
                }

                const actualLine = lines[position.line - 1]
                const documentLine = codeElement.textContent || ''

                const actualLeadingWhiteSpace = actualLine.length - trimStart(actualLine).length
                const documentLeadingWhiteSpace = documentLine.length - trimStart(documentLine).length

                const modifier = direction === AdjustmentDirection.ActualToCodeView ? -1 : 1
                const delta = Math.abs(actualLeadingWhiteSpace - documentLeadingWhiteSpace) * modifier

                return {
                    line: position.line,
                    character: position.character + delta,
                }
            })
        )

const searchResultCodeViewResolver = toCodeViewResolver('.code-list-item', {
    dom: searchCodeSnippetDOMFunctions,
    getPositionAdjuster: getSnippetPositionAdjuster,
    resolveFileInfo: resolveSnippetFileInfo,
})

const snippetCodeView: Omit<CodeView, 'element'> = {
    dom: singleFileDOMFunctions,
    resolveFileInfo: resolveSnippetFileInfo,
    getPositionAdjuster: getSnippetPositionAdjuster,
}

export const createFileLineContainerToolbarMount: NonNullable<CodeView['getToolbarMount']> = (
    codeViewElement: HTMLElement
): HTMLElement => {
    const className = 'sourcegraph-github-file-code-view-toolbar-mount'
    const existingMount = codeViewElement.querySelector(`.${className}`) as HTMLElement
    if (existingMount) {
        return existingMount
    }
    const mountElement = document.createElement('div')
    mountElement.style.display = 'inline-flex'
    mountElement.style.verticalAlign = 'middle'
    mountElement.style.alignItems = 'center'
    mountElement.className = className

    // new GitHub UI
    const container =
        codeViewElement.querySelector('#repos-sticky-header')?.childNodes[0]?.childNodes[0]?.childNodes[1]
            ?.childNodes[1] // we have to use this level of nesting when selecting a target container because #repos-sticky-header children don't have specific classes or ids
    if (container instanceof HTMLElement) {
        container.prepend(mountElement)
        return mountElement
    }

    // old GitHub UI (e.g., GHE)
    const rawURLLink = codeViewElement.querySelector('#raw-url')
    const buttonGroup = rawURLLink?.closest('.BtnGroup')
    if (buttonGroup?.parentNode) {
        buttonGroup.parentNode.insertBefore(mountElement, buttonGroup)
        return mountElement
    }

    throw new Error('Failed to create file toolbar mount node: container not found.')
}

/**
 * Matches the modern single-file code view, or snippets embedded in comments.
 *
 */
const fileLineContainerResolver: ViewResolver<CodeView> = {
    selector: getSelectorFor('blobContainer'),
    resolveView: (fileLineContainer: HTMLElement): CodeView | null => {
        const embeddedBlobWrapper = fileLineContainer.closest('.blob-wrapper-embedded')
        if (embeddedBlobWrapper) {
            // This is a snippet embedded in a comment.
            // Resolve to `.blob-wrapper-embedded`'s parent element,
            // the smallest element that contains both the code and
            // the HTML anchor allowing to resolve the file info.
            const element = embeddedBlobWrapper.parentElement!
            return {
                element,
                ...snippetCodeView,
            }
        }
        const { pageType } = parseURL()
        if (pageType !== 'blob') {
            // this is not a single-file code view
            return null
        }

        // selector depends on whether the page was rendered using the client-side navigation or not
        const repositoryContent = fileLineContainer.closest('.repository-content, #repo-content-turbo-frame')
        if (!repositoryContent) {
            throw new Error('Could not find repository content element')
        }
        return {
            element: repositoryContent as HTMLElement,
            ...singleFileCodeView,
            getToolbarMount: createFileLineContainerToolbarMount,
        }
    },
}

const genericCodeViewResolver: ViewResolver<CodeView> = {
    selector: target => {
        const codeViews = new Set<HTMLElement>()

        // Logic to support large diffs that are loaded asynchronously:
        // https://github.com/sourcegraph/sourcegraph/issues/18337
        // - Don't return `.file` elements that have yet to be loaded (loading is triggered by user)
        // - When the user triggers diff loading, the mutation observer will tell us about
        // .js-blob-wrapper, since the actual '.file' has been in the DOM the whole time. Return
        // the closest ancestor '.file'

        for (const file of querySelectorAllOrSelf<HTMLElement>(target, '.file')) {
            if (file.querySelectorAll('.js-diff-load-container').length === 0) {
                codeViews.add(file)
            }
        }

        for (const blobWrapper of querySelectorAllOrSelf(target, '.js-blob-wrapper')) {
            const file = blobWrapper.closest('.file')
            if (file instanceof HTMLElement) {
                codeViews.add(file)
            }
        }

        return [...codeViews]
    },
    resolveView: (element: HTMLElement): CodeView | null => {
        if (element.querySelector('article.markdown-body')) {
            // This code view is rendered markdown, we shouldn't add code navigation
            return null
        }

        // This is a suggested change on a GitHub PR
        if (element.closest('.js-suggested-changes-blob')) {
            return null
        }

        const { pageType } = parseURL()
        const isSingleCodeFile =
            pageType === 'blob' &&
            document.querySelectorAll('.file').length === 1 &&
            document.querySelectorAll('.diff-view').length === 0

        if (isSingleCodeFile) {
            return { element, ...singleFileCodeView }
        }

        if (element.closest('.discussion-item-body') || element.classList.contains('js-comment-container')) {
            // This code view is embedded on a PR conversation page.
            return { element, ...diffConversationCodeView }
        }

        return { element, ...diffCodeView }
    },
}

/**
 * Returns true if the current page is GitHub Enterprise.
 */
export function checkIsGitHubEnterprise(): boolean {
    const ogSiteName = document.head.querySelector<HTMLMetaElement>('meta[property="og:site_name"]')
    return (
        !!ogSiteName &&
        // GitHub Enterprise v2.14.11 has "GitHub" as og:site_name
        (ogSiteName.content === 'GitHub Enterprise' || ogSiteName.content === 'GitHub') &&
        document.body.classList.contains('enterprise')
    )
}

/**
 * Returns true if the current page is github.com.
 */
export const checkIsGitHubDotCom = (url = window.location.href): boolean => /^https?:\/\/(www\.)?github\.com/.test(url)

/**
 * Returns true if the current page is either github.com or GitHub Enterprise.
 */
export const checkIsGitHub = (): boolean => checkIsGitHubDotCom() || checkIsGitHubEnterprise()

const OPEN_ON_SOURCEGRAPH_ID = 'open-on-sourcegraph'

export const createOpenOnSourcegraphIfNotExists: MountGetter = (container: HTMLElement): HTMLElement | null => {
    const pageheadActions = querySelectorOrSelf(container, '.pagehead-actions')
    // If ran on page that isn't under a repository namespace.
    if (!pageheadActions || pageheadActions.children.length === 0) {
        return null
    }
    // Check for existing
    let mount = pageheadActions.querySelector<HTMLElement>('#' + OPEN_ON_SOURCEGRAPH_ID)
    if (mount) {
        return mount
    }
    // Create new
    mount = document.createElement('li')
    mount.id = OPEN_ON_SOURCEGRAPH_ID
    pageheadActions.prepend(mount)
    return mount
}

const iconClassName = classNames(styles.icon, 'v-align-text-bottom')

const searchEnhancement: GithubCodeHost['searchEnhancement'] = {
    searchViewResolver: {
        selector: '.js-site-search-form input[type="text"][aria-controls="jump-to-results"]',
        resolveView: element => ({ element }),
    },
    resultViewResolver: {
        selector: '#jump-to-suggestion-search-global',
        resolveView: element => ({ element }),
    },
    onChange: ({ value, searchURL, resultElement: ghElement }) => {
        const SEARCH_IN_SOURCEGRAPH_SELECTOR = '#jump-to-sourcegraph-search-global'

        /** Create "Search in Sourcegraph" element based on GH element */
        const createElement = (): HTMLElement => {
            /** SG Base element on top of GH "All Github" element */
            const sgElement = ghElement.cloneNode(true) as HTMLElement
            sgElement.id = SEARCH_IN_SOURCEGRAPH_SELECTOR.replace('#', '')
            sgElement.classList.remove('navigation-focus')
            sgElement.setAttribute('aria-selected', 'false')

            /** Add sourcegraph logo */
            const logo = document.createElement('img')
            logo.src = LogoSVG
            logo.setAttribute('style', 'width: 16px; height: 20px; float: left; margin-right: 2px;')
            logo.setAttribute('alt', 'Sourcegraph Logo Image')

            /** Update badge text */
            const badge = sgElement.querySelector('.js-jump-to-badge-search-text-global') as HTMLElement
            badge.textContent = 'Sourcegraph'
            badge.parentNode?.insertBefore(logo, badge)

            /** Add sourcegraph item after GH item */
            ghElement.parentNode?.insertBefore(sgElement, ghElement.nextElementSibling)

            return sgElement
        }

        /** Update link and display value */
        const updateContent = (sgElement: HTMLElement): void => {
            const displayValue = sgElement.querySelector<HTMLElement>('.jump-to-suggestion-name') as HTMLElement
            displayValue.textContent = value
            displayValue.setAttribute('aria-label', value)

            const link = sgElement.querySelector<HTMLElement>('a') as HTMLLinkElement
            const url = new URL(searchURL)
            url.searchParams.append('q', value)
            link.setAttribute('href', url.href)
            link.setAttribute('target', '_blank')
            sgElement.setAttribute('style', `display: ${value ? 'initial' : 'none !important'}`)
        }

        updateContent(document.querySelector<HTMLElement>(SEARCH_IN_SOURCEGRAPH_SELECTOR) ?? createElement())
    },
}

/**
 * Checks whether repository is private by querying its page on GitHub
 * and either parsing the HTML response or falling back to DOM element check.
 */
export const isPrivateRepository = async (
    repoName: string,
    fetchCache = background.fetchCache,
    fallbackSelector = '#repository-container-header h2 span.Label'
): Promise<boolean> => {
    if (window.location.hostname !== 'github.com') {
        return Promise.resolve(true)
    }
    try {
        const { status } = await fetchCache({
            url: `https://github.com/${repoName}`,
            credentials: 'omit',
            cacheMaxAge: 60 * 60 * 1000, // 1 hour
        })
        return status !== 200
    } catch (error) {
        // If network error
        console.warn('Failed to fetch if the repository is private.', error)
        return document.querySelector(fallbackSelector)?.textContent?.toLowerCase().trim() !== 'public'
    }
}

export interface GithubCodeHost extends CodeHost {
    /**
     * Configuration for built-in search input enhancement
     */
    searchEnhancement: {
        /** Search input element resolver */
        searchViewResolver: ViewResolver<{ element: HTMLElement }>
        /** Search result element resolver */
        resultViewResolver: ViewResolver<{ element: HTMLElement }>
        /** Callback to trigger on input element change */
        onChange: (args: { value: string; searchURL: string; resultElement: HTMLElement }) => void
    }

    enhanceSearchPage: (sourcegraphURL: string) => void
}

export const isGithubCodeHost = (codeHost: CodeHost): codeHost is GithubCodeHost => codeHost.type === 'github'

const isSimpleSearchPage = (): boolean => window.location.pathname === '/search'
const isAdvancedSearchPage = (): boolean => window.location.pathname === '/search/advanced'
const isRepoSearchPage = (): boolean => !isSimpleSearchPage() && window.location.pathname.endsWith('/search')
const isSearchResultsPage = (): boolean =>
    // TODO(#44327): Do not rely on window.location.search - it may be present not only on search pages (e.g., issues, pulls, etc.).
    Boolean(new URLSearchParams(window.location.search).get('q')) && !isAdvancedSearchPage()
const isSearchPage = (): boolean =>
    isSimpleSearchPage() || isAdvancedSearchPage() || isRepoSearchPage() || isSearchResultsPage()

type GithubResultType =
    | 'repositories'
    | 'code'
    | 'commits'
    | 'issues'
    | 'discussions'
    | 'packages'
    | 'marketplace'
    | 'topics'
    | 'wikis'
    | 'users'

const getGithubResultType = (): GithubResultType | '' => {
    const githubResultType = new URLSearchParams(window.location.search).get('type')

    return githubResultType ? (githubResultType.toLowerCase() as GithubResultType) : ''
}

type SourcegraphResultType = 'repo' | 'commit'

const getSourcegraphResultType = (): SourcegraphResultType | '' => {
    const githubResultType = getGithubResultType()

    switch (githubResultType) {
        case 'repositories':
            return 'repo'
        case 'commits':
            return 'commit'
        case 'code':
            return ''
        default:
            return isSimpleSearchPage() || isAdvancedSearchPage() ? 'repo' : ''
    }
}

const getSourcegraphResultLanguage = (): string | null => new URLSearchParams(window.location.search).get('l')

const buildSourcegraphQuery = (searchTerms: string[]): string => {
    const queryParameters = searchTerms.filter(Boolean).map(parameter => parameter.trim())
    const sourcegraphResultType = getSourcegraphResultType()
    const resultsLanguage = getSourcegraphResultLanguage()

    if (sourcegraphResultType) {
        queryParameters.push(`type:${sourcegraphResultType}`)
    }

    if (resultsLanguage) {
        queryParameters.push(`lang:${encodeURIComponent(resultsLanguage)}`)
    }

    if (isRepoSearchPage()) {
        const [user, repo] = window.location.pathname.split('/').filter(Boolean)
        queryParameters.push(`repo:${user}/${repo}$`)
    }

    for (const owner of ['org', 'user']) {
        const index = queryParameters.findIndex(parameter => parameter.startsWith(`${owner}:`))
        if (index >= 0) {
            const name = queryParameters[index].replace(`${owner}:`, '')
            if (name) {
                queryParameters[index] = `repo:${name}/*`
            }
        }
    }

    return queryParameters.join('+')
}

const queryByIdOrCreate = (id: string, className = ''): HTMLElement => {
    let element = document.querySelector<HTMLElement>(`#${id}`)

    if (element) {
        return element
    }

    element = document.createElement('div')
    element.setAttribute('id', id)
    element.classList.add(...className.split(/\s/gi))

    return element
}

export const parseHash = (hash: string): LineOrPositionOrRange => {
    const matches = hash.match(/(L\d+)/g)

    if (!matches || matches.length > 2) {
        return {}
    }

    const lpr = {} as LineOrPositionOrRange
    const [startString, endString] = matches.map(string => string.slice(1))

    lpr.line = parseInt(startString, 10)
    if (endString) {
        lpr.endLine = parseInt(endString, 10)
    }

    return lpr
}

/**
 * Adds "Search on Sourcegraph buttons" to GitHub search pages
 */
function enhanceSearchPage(sourcegraphURL: string): void {
    if (!isSearchPage()) {
        return
    }

    // TODO: cleanup button on unsubscribe
    const renderSourcegraphButton = ({
        container,
        className = '',
        getSearchQuery,
        utmCampaign,
    }: {
        container: HTMLElement
        className?: string
        utmCampaign: string
        getSearchQuery: () => string[]
    }): void => {
        const sourcegraphSearchURL = createURLWithUTM(new URL('/search', sourcegraphURL), {
            utm_source: getPlatformName(),
            utm_campaign: utmCampaign,
        })
        const root = createRoot(container)

        root.render(
            <SourcegraphIconButton
                label="Search on Sourcegraph"
                title="Search on Sourcegraph to get hover tooltips, go to definition and more"
                ariaLabel="Search on Sourcegraph to get hover tooltips, go to definition and more"
                className={classNames('btn', 'm-auto', className)}
                iconClassName={classNames('mr-1', 'v-align-middle', styles.icon)}
                href={sourcegraphSearchURL.href}
                dataTestId="search-on-sourcegraph"
                onClick={event => {
                    const searchQuery = buildSourcegraphQuery(getSearchQuery())

                    // Note: we don't use URLSearchParams.set('q', value) as it encodes the value which can't be correctly parsed by sourcegraph search page.
                    ;(event.target as HTMLAnchorElement).href = `${sourcegraphSearchURL.href}${
                        searchQuery ? `&q=${searchQuery}` : ''
                    }`
                }}
            />
        )
    }

    if (isSearchResultsPage()) {
        const githubResultType = getGithubResultType()

        if (
            ['repositories', 'commits', 'code'].includes(githubResultType) ||
            (githubResultType === '' && (isSimpleSearchPage() || isRepoSearchPage()))
        ) {
            /*
                Separate search form is visible for screen sizes xs-md, so we add a Sourcegraph button
                next to form submit button and track search query changes from the corresponding form input.
                On screen sizes smaller than md Github *Submit* and *Search on Sourcegraph* buttons are hidden while search input remains visible.
            */

            const pageSearchForm = document.querySelector<HTMLFormElement>('.application-main form.js-site-search-form')
            const pageSearchInput = pageSearchForm?.querySelector<HTMLInputElement>("input.form-control[name='q']")
            const pageSearchFormSubmitButton =
                pageSearchForm?.parentElement?.parentElement?.querySelector<HTMLButtonElement>("button[type='submit']")

            if (pageSearchInput && pageSearchFormSubmitButton) {
                const buttonContainer = queryByIdOrCreate('pageSearchFormSourcegraphButton', 'ml-2 d-none d-md-block')
                pageSearchFormSubmitButton.after(buttonContainer)

                renderSourcegraphButton({
                    container: buttonContainer,
                    utmCampaign: 'github-search-results-page',
                    getSearchQuery: () => pageSearchInput.value.split(' '),
                })
            }

            /*
                On screen sizes lg and larger separate search form is hidden, so we add a Sourcegraph button
                next to search results header container and track search query changes from search input in header.
            */

            const headerSearchInput = document.querySelector<HTMLInputElement>(
                "header form.js-site-search-form input.form-control[name='q']"
            )
            const searchResultsContainer = document.querySelector<HTMLDivElement>('.codesearch-results')
            const emptyResultsContainer = searchResultsContainer?.querySelector('.blankslate')
            const searchResultsContainerHeading =
                searchResultsContainer?.querySelector<HTMLHeadingElement>('div > div > h3')

            if (headerSearchInput && (emptyResultsContainer || searchResultsContainerHeading)) {
                const buttonContainer: HTMLElement = queryByIdOrCreate(
                    'headerSearchInputSourcegraphButton',
                    'ml-auto d-none d-lg-block'
                )

                if (emptyResultsContainer) {
                    buttonContainer.classList.add('mt-2')
                    emptyResultsContainer.append(buttonContainer)
                } else if (searchResultsContainerHeading) {
                    buttonContainer.classList.add('mr-2')
                    searchResultsContainerHeading.after(buttonContainer)
                }

                renderSourcegraphButton({
                    container: buttonContainer,
                    className: emptyResultsContainer ? '' : 'btn-sm',
                    utmCampaign: 'github-search-results-page',
                    getSearchQuery: () => headerSearchInput.value.split(' '),
                })
            }
        }

        return
    }

    /* Simple and advanced search pages */

    const searchForm = document.querySelector<HTMLFormElement>('#search_form')
    const searchInputContainer = searchForm?.querySelector('.search-form-fluid')
    const inputElement = searchForm?.querySelector<HTMLInputElement>('input')

    if (searchInputContainer && inputElement) {
        const buttonContainer = queryByIdOrCreate('searchInputSourcegraphButton', 'ml-0 ml-md-2 mt-2 mt-md-0')
        searchInputContainer?.append(buttonContainer)

        renderSourcegraphButton({
            container: buttonContainer,
            utmCampaign: `github-${isAdvancedSearchPage() ? 'advanced' : 'simple'}-search-page`,
            getSearchQuery: () => inputElement.value.split(' '),
        })
    }
}

export const githubCodeHost: GithubCodeHost = {
    type: 'github',
    name: checkIsGitHubEnterprise() ? 'GitHub Enterprise' : 'GitHub',
    searchEnhancement,
    enhanceSearchPage,
    codeViewResolvers: [genericCodeViewResolver, fileLineContainerResolver, searchResultCodeViewResolver],
    routeChange: mutations =>
        mutations.pipe(
            map(() => {
                const { pathname } = window.location

                // repository file tree navigation
                const pageType = pathname.slice(1).split('/')[2]
                if (pageType === 'blob' || pageType === 'tree') {
                    return pathname.endsWith(resolveFileInfo().blob.filePath) ? pathname : undefined
                }

                // search results page filters being applied
                // TODO(#44327): Uncomment or remove this depending on the outcome of the issue.
                // if (isSearchResultsPage()) {
                //     return document.querySelector('.codesearch-results h3')?.textContent?.trim()
                // }

                // other pages
                return pathname
            }),
            filter(Boolean),
            distinctUntilChanged()
        ),
    getContext: async () => {
        const { repoName, rawRepoName, pageType } = parseURL()

        return {
            rawRepoName,
            revision: pageType === 'blob' || pageType === 'tree' ? resolveFileInfo().blob.revision : undefined,
            privateRepository: await isPrivateRepository(repoName),
        }
    },
    isLightTheme: defer(() => {
        const mode = document.documentElement.dataset.colorMode as 'auto' | 'light' | 'dark' | undefined
        if (mode === 'auto') {
            return observeSystemIsLightTheme().observable
        }
        return of(mode !== 'dark')
    }),
    getViewContextOnSourcegraphMount: createOpenOnSourcegraphIfNotExists,
    viewOnSourcegraphButtonClassProps: {
        className: 'btn btn-sm tooltipped tooltipped-s',
        iconClassName,
    },
    check: checkIsGitHub,
    codeViewToolbarClassProps: {
        className: styles.codeViewToolbar,
        listItemClass: classNames(styles.codeViewToolbarItem, 'BtnGroup'),
        actionItemClass: classNames('btn btn-sm tooltipped tooltipped-s BtnGroup-item', styles.actionItem),
        actionItemPressedClass: 'selected',
        actionItemIconClass: classNames(styles.icon, 'v-align-text-bottom'),
    },
    hoverOverlayClassProps: {
        className: 'Box',
        actionItemClassName: 'btn btn-sm btn-secondary',
        actionItemPressedClassName: 'active',
        closeButtonClassName: 'btn-octicon p-0 hover-overlay__close-button--github',
        badgeClassName: classNames('label', styles.hoverOverlayBadge),
        iconClassName,
    },
    urlToFile: (sourcegraphURL, target, context) => {
        if (target.viewState) {
            // A view state means that a panel must be shown, and panels are currently only supported on
            // Sourcegraph (not code hosts).
            return toAbsoluteBlobURL(sourcegraphURL, target)
        }

        // Make sure the location is also on this github instance, return an absolute URL otherwise.
        const sameCodeHost = target.rawRepoName.startsWith(window.location.hostname)
        if (!sameCodeHost) {
            return toAbsoluteBlobURL(sourcegraphURL, target)
        }

        const revision = target.revision || 'HEAD'
        // If we're provided options, we can make the j2d URL more specific.
        const { rawRepoName } = parseURL()

        // Stay on same page in PR if possible.
        // TODO to be entirely correct, this would need to compare the revision of the code view with the target revision.
        const isSameRepo = rawRepoName === target.rawRepoName
        if (isSameRepo && context.part !== undefined) {
            const containers = getFileContainers()
            for (const container of containers) {
                const header = container.querySelector<HTMLElement & { dataset: { path: string; anchor: string } }>(
                    '.file-header[data-path][data-anchor]'
                )
                if (!header) {
                    // E.g. suggestion snippet
                    continue
                }
                const anchorPath = header.dataset.path
                if (anchorPath === target.filePath) {
                    const anchorUrl = header.dataset.anchor
                    const url = new URL(window.location.href)
                    url.hash = anchorUrl
                    if (target.position) {
                        // GitHub uses L for the left side, R for both right side and the unchanged/white parts
                        url.hash += `${context.part === 'base' ? 'L' : 'R'}${target.position.line}`
                    }
                    // Only use URL if it is visible
                    // TODO: Expand hidden lines to reveal
                    if (!document.querySelector(url.hash)) {
                        break
                    }
                    return url.href
                }
            }
        }

        // Go to blob URL
        const fragment = target.position
            ? `#L${target.position.line}${target.position.character ? `:${target.position.character}` : ''}`
            : ''
        return `https://${target.rawRepoName}/blob/${revision}/${target.filePath}${fragment}`
    },
    observeLineSelection: fromEvent(window, 'hashchange').pipe(
        startWith(undefined), // capture intital value
        map(() => parseHash(window.location.hash))
    ),
    codeViewsRequireTokenization: true,
}
