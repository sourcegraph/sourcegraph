import classNames from 'classnames'
import { map } from 'rxjs/operators'

import { AdjustmentDirection, PositionAdjuster } from '@sourcegraph/codeintellify'
import { Position } from '@sourcegraph/extension-api-types'
import { PlatformContext } from '@sourcegraph/shared/src/platform/context'
import { FileSpec, RepoSpec, ResolvedRevisionSpec, RevisionSpec } from '@sourcegraph/shared/src/util/url'

import { fetchBlobContentLines } from '../../repo/backend'
import { CodeHost } from '../shared/codeHost'
import { CodeView, toCodeViewResolver } from '../shared/codeViews'
import { ViewResolver } from '../shared/views'

import { diffDomFunctions, diffusionDOMFns } from './domFunctions'
import { resolveDiffFileInfo, resolveDiffusionFileInfo, resolveRevisionFileInfo } from './fileInfo'

import { convertSpacesToTabs, spacesToTabsAdjustment } from '.'

import styles from './codeHost.module.scss'

/**
 * Gets the actual text content we care about and returns the number of characters we have stripped
 * so that we can adjust accordingly.
 */
const getTextContent = (element: HTMLElement): { textContent: string; adjust: number } => {
    let textContent = element.textContent || ''
    let adjust = 0

    // For some reason, phabricator adds an invisible element to the beginning of lines containing the diff indicator
    // followed by a space (ex: '+ '). We need to adjust the position accordingly.
    if (element.firstElementChild?.classList.contains('aural-only')) {
        const pre = element.firstElementChild.textContent || ''
        // Codeintellify handles ignoring one character for diff indicators so we'll allow it to adjust for that.
        adjust = pre.replace(/^(\+|-)/, '').length

        // Get rid of the characters we have adjusted for.
        textContent = textContent.slice(pre.length - adjust)
    }

    // Phabricator adds a no-width-space to the beginning of the line in some cases.
    // We need to strip that and account for it here.
    if (textContent.charCodeAt(0) === 8203) {
        textContent = textContent.slice(1)
        adjust++
    }

    return { textContent, adjust }
}

const adjustCharacter = (position: Position, adjustment: number): Position => ({
    line: position.line,
    character: position.character + adjustment,
})

const getPositionAdjuster =
    (
        requestGraphQL: PlatformContext['requestGraphQL']
    ): PositionAdjuster<RepoSpec & RevisionSpec & FileSpec & ResolvedRevisionSpec> =>
    ({ direction, codeView, position }) =>
        fetchBlobContentLines({ ...position, requestGraphQL }).pipe(
            map(lines => {
                const codeElement = diffDomFunctions.getCodeElementFromLineNumber(
                    codeView,
                    position.line,
                    position.part
                )
                if (!codeElement) {
                    throw new Error('(adjustPosition) could not find code element for line provided')
                }

                const textContentInfo = getTextContent(codeElement)

                const documentLineContent = textContentInfo.textContent
                const actualLineContent = lines[position.line - 1]

                // See if we should adjust for whitespace changes.
                const convertSpaces = convertSpacesToTabs(actualLineContent, documentLineContent)

                // Whether the adjustment should add or subtract from the given position.
                const modifier = direction === AdjustmentDirection.CodeViewToActual ? -1 : 1

                return convertSpaces
                    ? adjustCharacter(
                          position,
                          (spacesToTabsAdjustment(documentLineContent) + textContentInfo.adjust) * modifier
                      )
                    : adjustCharacter(position, textContentInfo.adjust * modifier)
            })
        )

export const commitCodeView = {
    dom: diffDomFunctions,
    resolveFileInfo: resolveRevisionFileInfo,
    getPositionAdjuster,
    getToolbarMount: (codeView: HTMLElement): HTMLElement => {
        let mount = codeView.querySelector<HTMLElement>('.sourcegraph-phabricator-code-view-toolbar-mount')
        if (mount) {
            return mount
        }
        const actions = codeView.querySelector('.differential-changeset-buttons')
        if (!actions) {
            throw new Error('Unable to find action links for revision')
        }

        mount = document.createElement('div')
        mount.style.display = 'inline-block'
        mount.classList.add('sourcegraph-phabricator-code-view-toolbar-mount')

        actions.prepend(mount)

        return mount
    },
}

export const diffCodeView = {
    dom: diffDomFunctions,
    resolveFileInfo: resolveDiffFileInfo,
    getPositionAdjuster,
    getToolbarMount: (codeView: HTMLElement): HTMLElement => {
        const className = 'sourcegraph-phabricator-code-view-toolbar-mount'
        const existingMount = codeView.querySelector<HTMLElement>('.' + className)
        if (existingMount) {
            return existingMount
        }
        const mountLocation = codeView.querySelector('.differential-changeset-buttons')
        if (!mountLocation) {
            throw new Error('Unable to find action links for changeset')
        }
        const mount = document.createElement('div')
        mount.style.display = 'inline-block'
        mount.classList.add(className)
        mountLocation.prepend(mount, ' ')
        return mount
    },
    isDiff: true,
}

const differentialChangesetCodeViewResolver: ViewResolver<CodeView> = {
    selector: '.differential-changeset',
    resolveView: (element: HTMLElement): CodeView => {
        if (window.location.pathname.match(/^\/r/)) {
            return { element, ...commitCodeView }
        }
        return { element, ...diffCodeView }
    },
}

// TODO this code view does not include the toolbar,
// which makes it not possible to test getToolbarMount()
// Fix after https://github.com/sourcegraph/sourcegraph/issues/3271
const diffusionSourceCodeViewResolver = toCodeViewResolver('.diffusion-source', {
    dom: diffusionDOMFns,
    resolveFileInfo: resolveDiffusionFileInfo,
    getToolbarMount: () => {
        const actions = document.querySelector<HTMLElement>('.phui-two-column-content .phui-header-action-links')
        if (!actions) {
            throw new Error('unable to find file actions')
        }

        const mount = document.createElement('div')
        mount.style.display = 'inline-block'
        mount.classList.add('sourcegraph-phabricator-code-view-toolbar-mount')

        actions.prepend(mount)

        return mount
    },
})

// Matches Diffusion single file code views on recent Phabricator versions.
const phabSourceCodeViewResolver = toCodeViewResolver('.phabricator-source-code-container', {
    dom: diffusionDOMFns,
    resolveFileInfo: resolveDiffusionFileInfo,
})

export const checkIsPhabricator = (): boolean => !!document.querySelector('.phabricator-wordmark')

export const phabricatorCodeHost: CodeHost = {
    codeViewResolvers: [
        differentialChangesetCodeViewResolver,
        diffusionSourceCodeViewResolver,
        phabSourceCodeViewResolver,
    ],
    type: 'phabricator',
    name: 'Phabricator',
    check: checkIsPhabricator,

    // TODO: handle parsing selected line number from Phabricator href,
    // and find a way to listen to changes (Phabricator does not emit popstate events).
    codeViewToolbarClassProps: {
        actionItemClass: classNames('button grey', styles.actionItem),
        actionItemIconClass: styles.icon,
    },
    hoverOverlayClassProps: {
        className: classNames('aphront-dialog-view', styles.hoverOverlay),
        actionItemClassName: classNames('button grey', styles.hoverOverlayActionItem),
        closeButtonClassName: 'button grey btn-icon--phabricator',
        iconClassName: styles.hoverOverlayActionItemIcon,
    },
    codeViewsRequireTokenization: true,
}
