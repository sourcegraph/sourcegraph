import classNames from 'classnames'
import { fromEvent, of } from 'rxjs'
import { map, startWith } from 'rxjs/operators'
import { Omit } from 'utility-types'

import { AdjustmentDirection, PositionAdjuster } from '@sourcegraph/codeintellify'
import { LineOrPositionOrRange } from '@sourcegraph/common'
import { FileSpec, RepoSpec, ResolvedRevisionSpec, RevisionSpec } from '@sourcegraph/shared/src/util/url'

import { querySelectorOrSelf } from '../../util/dom'
import { CodeHost } from '../shared/codeHost'
import { CodeView, DOMFunctions } from '../shared/codeViews'
import { ViewResolver } from '../shared/views'

import { getContext } from './context'
import { diffDOMFunctions, newDiffDOMFunctions, singleFileDOMFunctions } from './domFunctions'
import {
    resolveCommitViewFileInfo,
    resolveCompareFileInfo,
    resolveFileInfoForSingleFileSourceView,
    resolvePullRequestFileInfo,
    resolveSingleFileDiffFileInfo,
} from './fileInfo'
import { isCommitsView, isCompareView, isPullRequestView, isSingleFileView } from './scrape'

import styles from './codeHost.module.scss'

/**
 * Gets or creates the toolbar mount for allcode views.
 */
export const getToolbarMount = (
    codeView: HTMLElement,
    fileToolbarSelector = '.file-toolbar .secondary'
): HTMLElement => {
    const existingMount = codeView.querySelector<HTMLElement>('.sg-toolbar-mount')
    if (existingMount) {
        return existingMount
    }

    const fileActions = codeView.querySelector<HTMLElement>(fileToolbarSelector)
    if (!fileActions) {
        throw new Error('Unable to find mount location')
    }

    const mount = document.createElement('div')
    mount.classList.add('btn-group')
    mount.classList.add('sg-toolbar-mount')
    mount.classList.add(styles.sgToolbarMount)

    fileActions.prepend(mount)

    return mount
}

/**
 * Sometimes tabs are converted to spaces so we need to adjust. Luckily, there
 * is an attribute `cm-text` that contains the real text.
 */
const createPositionAdjuster =
    (dom: DOMFunctions): PositionAdjuster<RepoSpec & RevisionSpec & FileSpec & ResolvedRevisionSpec> =>
    ({ direction, codeView, position }) => {
        const codeElement = dom.getCodeElementFromLineNumber(codeView, position.line, position.part)
        if (!codeElement) {
            throw new Error('(adjustPosition) could not find code element for line provided')
        }

        let delta = 0
        for (const modifiedTextElement of codeElement.querySelectorAll('[cm-text]')) {
            const actualText = modifiedTextElement.getAttribute('cm-text') || ''
            const adjustedText = modifiedTextElement.textContent || ''

            delta += actualText.length - adjustedText.length
        }

        const modifier = direction === AdjustmentDirection.ActualToCodeView ? -1 : 1

        const newPosition = {
            line: position.line,
            character: position.character + modifier * delta,
        }

        return of(newPosition)
    }

/**
 * A code view spec for single file code view in the "source" view (not diff).
 */
const singleFileSourceCodeView: Omit<CodeView, 'element'> = {
    getToolbarMount,
    dom: singleFileDOMFunctions,
    resolveFileInfo: resolveFileInfoForSingleFileSourceView,
    getPositionAdjuster: () => createPositionAdjuster(singleFileDOMFunctions),
}

const baseDiffCodeView: Omit<CodeView, 'element' | 'resolveFileInfo'> = {
    getToolbarMount,
    dom: diffDOMFunctions,
    getPositionAdjuster: () => createPositionAdjuster(diffDOMFunctions),
    // Bitbucket diff views are not tokenized.
    overrideTokenize: true,
}
/**
 * A code view spec for a single file "diff to previous" view
 */
const singleFileDiffCodeView: Omit<CodeView, 'element'> = {
    ...baseDiffCodeView,
    resolveFileInfo: resolveSingleFileDiffFileInfo,
}

/**
 * A code view spec for pull requests
 */
const pullRequestDiffCodeView: Omit<CodeView, 'element'> = {
    ...baseDiffCodeView,
    resolveFileInfo: resolvePullRequestFileInfo,
}

/**
 * A code view spec for compare pages
 */
const compareDiffCodeView: Omit<CodeView, 'element'> = {
    ...baseDiffCodeView,
    resolveFileInfo: resolveCompareFileInfo,
}

/**
 * A code view spec for commit pages
 */
const commitDiffCodeView: Omit<CodeView, 'element'> = {
    ...baseDiffCodeView,
    resolveFileInfo: resolveCommitViewFileInfo,
}

const codeViewResolver: ViewResolver<CodeView> = {
    selector: '.file-content',
    resolveView: element => {
        const contentView = element.querySelector('.content-view')
        if (!contentView) {
            return null
        }
        if (isCompareView()) {
            return { element, ...compareDiffCodeView }
        }
        if (isCommitsView(window.location)) {
            return { element, ...commitDiffCodeView }
        }
        if (isSingleFileView(element)) {
            const isDiff = contentView.classList.contains('diff-view')
            return isDiff ? { element, ...singleFileDiffCodeView } : { element, ...singleFileSourceCodeView }
        }
        if (isPullRequestView(window.location)) {
            return { element, ...pullRequestDiffCodeView }
        }
        console.error('Unknown code view', element)
        return null
    },
}

/**
 * New diff code view resolver.
 * As of Bitbucket v7.11.2, this is only used for the pull request page.
 */
const diffCodeViewResolver: ViewResolver<CodeView> = {
    selector: '.change-view',
    resolveView: element => ({ element, ...newDiffCodeView }),
}

const newDiffToolbarButtonProps = {
    listItemClass: styles.actionNavItemNewDiff,
    actionItemClass: styles.actionItemNewDiff,
}

/**
 * New diff code view element.
 * As of Bitbucket v7.11.2, this is only used for the pull request page.
 */
const newDiffCodeView: Omit<CodeView, 'element'> = {
    resolveFileInfo: resolvePullRequestFileInfo,
    getToolbarMount: codeView => getToolbarMount(codeView, '.change-header .diff-actions'),
    toolbarButtonProps: newDiffToolbarButtonProps,
    dom: newDiffDOMFunctions,
}

function getViewContextOnSourcegraphMount(container: HTMLElement): HTMLElement | null {
    const branchSelectorButtons = querySelectorOrSelf(container, '.branch-selector-toolbar .aui-buttons')
    if (!branchSelectorButtons) {
        return null
    }
    const preexisting = branchSelectorButtons.querySelector<HTMLElement>('#open-on-sourcegraph')
    if (preexisting) {
        return preexisting
    }
    const mount = document.createElement('span')
    mount.id = 'open-on-sourcegraph'
    mount.className = styles.openOnSourcegraph
    branchSelectorButtons.append(mount)
    return mount
}

export const checkIsBitbucket = (): boolean =>
    !!document.querySelector('.bitbucket-header-logo') ||
    !!document.querySelector('.aui-header-logo.aui-header-logo-bitbucket')

const iconClassName = 'aui-icon'

export const parseHash = (hash: string): LineOrPositionOrRange => {
    if (hash.startsWith('#')) {
        hash = hash.slice(1)
    }

    if (!/^\d+(-\d+)?$/.test(hash)) {
        return {}
    }

    const lpr = {} as LineOrPositionOrRange
    const [startString, endString] = hash.split('-')

    lpr.line = parseInt(startString, 10)
    if (endString) {
        lpr.endLine = parseInt(endString, 10)
    }

    return lpr
}

export const bitbucketServerCodeHost: CodeHost = {
    type: 'bitbucket-server',
    name: 'Bitbucket Server',
    check: checkIsBitbucket,
    codeViewResolvers: [codeViewResolver, diffCodeViewResolver],
    codeViewToolbarClassProps: {
        className: classNames(styles.codeViewToolbar, 'aui-buttons'),
        actionItemClass: 'aui-button',
        // actionItemPressedClass is not needed because Bitbucket applies styling to aria-pressed="true"
        actionItemIconClass: 'aui-icon',
        listItemClass: styles.actionNavItem,
    },
    hoverOverlayClassProps: {
        className: 'aui-dialog',
        actionItemClassName: classNames('aui-button', styles.hoverActionItem),
        iconClassName,
    },
    getViewContextOnSourcegraphMount,
    getContext,
    viewOnSourcegraphButtonClassProps: {
        className: 'aui-button',
        iconClassName,
    },
    codeViewsRequireTokenization: false,
    observeLineSelection: fromEvent(window, 'hashchange').pipe(
        startWith(undefined), // capture intital value
        map(() => parseHash(window.location.hash))
    ),
}
