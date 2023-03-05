import { fromEvent } from 'rxjs'
import { map, startWith } from 'rxjs/operators'

import { LineOrPositionOrRange } from '@sourcegraph/common'

import { querySelectorOrSelf } from '../../util/dom'
import { CodeHost } from '../shared/codeHost'
import { CodeView } from '../shared/codeViews'
import { ViewResolver } from '../shared/views'

import { getContext } from './context'
import { commitDOMFunctions, pullRequestDOMFunctions, singleFileDOMFunctions } from './domFunctions'
import { getFileInfoForCommit, getFileInfoForPullRequest, getFileInfoFromSingleFileSourceCodeView } from './fileInfo'
import { isPullRequestView } from './scrape'

import styles from './codeHost.module.scss'

function checkIsBitbucketCloud(): boolean {
    return location.hostname === 'bitbucket.org'
}

function getToolbarMount(codeView: HTMLElement): HTMLElement {
    const existingMount = codeView.querySelector<HTMLElement>('.sg-toolbar-mount')
    if (existingMount) {
        return existingMount
    }

    const fileActions = codeView.querySelector<HTMLElement>('[data-testid="file-actions"')
    if (!fileActions) {
        throw new Error('Unable to find mount location')
    }

    const mount = document.createElement('div')
    mount.classList.add('sg-toolbar-mount')

    fileActions.prepend(mount)

    return mount
}

/**
 * Used for single file code views and pull requests.
 */
const codeViewResolver: ViewResolver<CodeView> = {
    selector: element => {
        // The "code view" element has no class, ID, or data attributes, so
        // look for the lowest common ancestor of file header and file content elements.
        const fileHeader = element.querySelector<HTMLElement>('[data-qa="bk-file__header"]')
        const fileContent = element.querySelector<HTMLElement>('[data-qa="bk-file__content"]')

        if (!fileHeader || !fileContent) {
            return null
        }

        let codeView: HTMLElement = fileHeader

        while (!codeView.contains(fileContent)) {
            if (!codeView.parentElement) {
                return null
            }
            codeView = codeView.parentElement
        }

        return [codeView]
    },
    resolveView: element => {
        if (isPullRequestView(window.location)) {
            return {
                element,
                getToolbarMount,
                dom: pullRequestDOMFunctions,
                resolveFileInfo: getFileInfoForPullRequest,
            }
        }

        return {
            element,
            getToolbarMount,
            dom: singleFileDOMFunctions,
            resolveFileInfo: getFileInfoFromSingleFileSourceCodeView,
        }
    },
}

function getCommitToolbarMount(codeView: HTMLElement): HTMLElement {
    const existingMount = codeView.querySelector<HTMLElement>('.sg-toolbar-mount')
    if (existingMount) {
        return existingMount
    }

    const diffActions = codeView.querySelector<HTMLElement>('.diff-actions')
    if (!diffActions) {
        throw new Error('Unable to find mount location')
    }

    diffActions.classList.add(styles.commitContainer)
    const mount = document.createElement('div')
    mount.classList.add('sg-toolbar-mount')

    diffActions.prepend(mount)

    return mount
}

/**
 * Used for commit and compare pages.
 * (Compare page is not included in the sidebar)
 */
const commitCodeViewResolver: ViewResolver<CodeView> = {
    selector: '.bb-udiff',
    resolveView: element => ({
        element,
        getToolbarMount: getCommitToolbarMount,
        dom: commitDOMFunctions,
        resolveFileInfo: getFileInfoForCommit,
    }),
}

function getViewContextOnSourcegraphMount(container: HTMLElement): HTMLElement | null {
    const OPEN_ON_SOURCEGRAPH_ID = 'open-on-sourcegraph'

    const pageHeader = querySelectorOrSelf(container, '[data-qa="page-header-wrapper"] > div > div')
    if (!pageHeader) {
        return null
    }

    let mount = pageHeader.querySelector<HTMLElement>('#' + OPEN_ON_SOURCEGRAPH_ID)
    if (mount) {
        return mount
    }
    mount = document.createElement('span')
    mount.id = OPEN_ON_SOURCEGRAPH_ID

    // At the time of development, the page header element had two children: breadcrumbs container and
    // page title + actions containers' container.
    // Try to add the view on Sourcegraph button as a child of the actions container.

    // This is brittle since it relies on DOM structure and not classes. If it fails in the future,
    // fallback to appending as last child of page header. This is still aesthetically acceptable.

    const actionsContainer = pageHeader.childNodes[1]?.childNodes[1].firstChild
    if (actionsContainer instanceof HTMLElement) {
        actionsContainer.append(mount)
    } else {
        pageHeader.append(mount)
    }

    return mount
}

export const parseHash = (hash: string): LineOrPositionOrRange => {
    if (hash.startsWith('#')) {
        hash = hash.slice(1)
    }

    if (!/^lines-\d+(:\d+)?$/.test(hash)) {
        return {}
    }

    const lpr = {} as LineOrPositionOrRange
    const [startString, endString] = hash.slice('lines-'.length).split(':')

    lpr.line = parseInt(startString, 10)
    if (endString) {
        lpr.endLine = parseInt(endString, 10)
    }

    return lpr
}

export const bitbucketCloudCodeHost: CodeHost = {
    type: 'bitbucket-cloud',
    name: 'Bitbucket Cloud',
    codeViewResolvers: [codeViewResolver, commitCodeViewResolver],
    getContext,
    getViewContextOnSourcegraphMount,
    check: checkIsBitbucketCloud,
    viewOnSourcegraphButtonClassProps: {
        className: styles.openOnSourcegraph,
        iconClassName: styles.icon,
    },
    codeViewToolbarClassProps: {
        className: styles.codeViewToolbar,
        listItemClass: styles.listItem,
        actionItemClass: styles.codeViewToolbarActionItem,
        actionItemPressedClass: styles.pressed,
        actionItemIconClass: styles.icon,
    },
    hoverOverlayClassProps: {
        className: styles.hoverOverlay,
        closeButtonClassName: styles.close,
        badgeClassName: styles.badge,
        actionItemClassName: styles.hoverOverlayActionItem,
        iconClassName: styles.icon,
        contentClassName: styles.content,
    },
    codeViewsRequireTokenization: true,
    observeLineSelection: fromEvent(window, 'hashchange').pipe(
        startWith(undefined), // capture intital value
        map(() => parseHash(window.location.hash))
    ),
}
