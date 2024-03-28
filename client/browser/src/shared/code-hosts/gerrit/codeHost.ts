import { compact, find, head } from 'lodash'
import { ReplaySubject, interval, type Observable, type Subject } from 'rxjs'
import { filter, map, share } from 'rxjs/operators'

import type { MutationRecordLike } from '../../util/dom'
import type { CodeHost } from '../shared/codeHost'
import type { CodeView, DOMFunctions } from '../shared/codeViews'
import { queryWithSelector, type ViewResolver, type CustomSelectorFunction } from '../shared/views'

import styles from './codeHost.module.scss'

const PATCHSET_LABEL_PATTERN = /patchset (\d+)/i

function checkIsGerrit(): boolean {
    const isGerrit = !!document.querySelector('gr-app#app')
    return isGerrit
}

interface GerritChangeAndPatchset {
    repoName: string
    changeId: string
    patchsetId?: string
    basePatchsetId?: string
    filePath?: string
}

function buildGerritChangeString(changeId: string, patchsetId: string): string {
    // The "change directory prefix" is a prefix composed of the *LAST* two
    // characters of the change ID, zero-padded.
    const changeDirectoryPrefix = changeId.slice(-2).padStart(2, '0')
    return `refs/changes/${changeDirectoryPrefix}/${changeId}/${patchsetId}`
}

/**
 * Get {@link querySelectorAcrossShadowRoots}
 */
function getTextFromSelector(selector: string): string | undefined {
    return querySelectorAcrossShadowRoots(document.body, selector)?.textContent ?? undefined
}

/**
 * Get the patchset ID (returned as `headPatchsetId`) and the base patchset ID,
 * if any. These are the head and base of the diff. The default and most common
 * case is that there is no base patchset ID selected (so its value is
 * undefined), in which case the base should be considered to be the parent
 * commit.
 */
function getHeadAndBasePatchsetIds(): { basePatchsetId: string | undefined; headPatchsetId: string | undefined } {
    // On the diff overview page
    const basePatchDropdownLabel =
        getTextFromSelector(
            '#app >> #app-element >> gr-change-view >> #fileListHeader >> #rangeSelect >> #basePatchDropdown >> #triggerText'
        ) ??
        getTextFromSelector(
            '#app >> #app-element >> gr-diff-view >> #rangeSelect >> #basePatchDropdown >> #triggerText'
        )
    const patchNumberDropdownLabel =
        getTextFromSelector(
            '#app >> #app-element >> gr-change-view >> #fileListHeader >> #rangeSelect >> #patchNumDropdown >> #triggerText'
        ) ??
        getTextFromSelector('#app >> #app-element >> gr-diff-view >> #rangeSelect >> #patchNumDropdown >> #triggerText')

    const basePatchsetId = basePatchDropdownLabel && getPatchsetIdFromLabel(basePatchDropdownLabel)
    const headPatchsetId = patchNumberDropdownLabel && getPatchsetIdFromLabel(patchNumberDropdownLabel)

    return { basePatchsetId, headPatchsetId }
}

/**
 * Get the patchset ID, e.g "1", from a dropdown label, e.g. "Patchset 1".
 */
function getPatchsetIdFromLabel(label: string): string | undefined {
    const patternMatch = PATCHSET_LABEL_PATTERN.exec(label)
    if (patternMatch?.[1]) {
        return patternMatch[1]
    }
    return undefined
}

/** Obtain info about the currently viewed Gerrit changed based on the URL path
 * and the patchset dropdowns. */
function parseGerritChange(): GerritChangeAndPatchset {
    const path = new URL(window.location.href).pathname
    const pathParts = path.split('/')
    const cPart = pathParts.indexOf('c')
    const plusSign = pathParts.indexOf('+')
    const repoName = pathParts.slice(cPart + 1, plusSign).join('/')
    const changeId = pathParts[plusSign + 1]

    // The URL may or may not contain the patchset ID. When viewing a single
    // file on Gerrit, there is no dropdown on the page that gives us the
    // current patchset ID, but fortunately in that case the URL should have it.
    const patchsetIdFromUrl: string | undefined = pathParts[plusSign + 2]

    const filePath = pathParts.slice(plusSign + 3).join('/')
    const repoNameWithServer = window.location.hostname + '/' + repoName
    const { basePatchsetId, headPatchsetId } = getHeadAndBasePatchsetIds()
    const parsedData = {
        repoName: repoNameWithServer,
        changeId,
        // Fall back to using the patchset ID from the URL when no dropdown
        // value was found in the page.
        patchsetId: headPatchsetId || patchsetIdFromUrl,
        filePath,
        basePatchsetId,
    }
    return parsedData
}

const resolveFileListCodeView: ViewResolver<CodeView> = {
    selector(target: HTMLElement) {
        // The Gerrit mutation observer uses this selector to emit added nodes.
        // But `trackViews` calls this selector also, to check if an added node
        // contains a code view, even though in Gerrit this is redundant. When
        // the selector is called with an existing matching element, then it
        // must return that element (and only that one element) rather than all
        // matching elements on the page.
        if (target.matches('#diffTable')) {
            return [target]
        }

        const fileListElement = querySelectorAcrossShadowRoots(
            document,
            '#app >> #app-element >> gr-change-view >> #fileList'
        )
        // Usually each `.file-row` which is `.expanded` will have a
        // corresponding `diffTable` under a sibling, with the common parent
        // being `.stickArea`.
        const fileRows = fileListElement?.shadowRoot?.querySelectorAll('.file-row.expanded')
        const fileRowElements = [...(fileRows || [])]
        const diffTables = fileRowElements.map(fileRow => {
            const stickyArea = fileRow.parentElement
            if (!stickyArea) {
                return
            }
            return querySelectorAcrossShadowRoots(stickyArea, 'gr-diff-host >> gr-diff >> #diffTable') as HTMLElement
        })
        return compact(diffTables)
    },
    resolveView(diffTableElement: HTMLElement): CodeView | null {
        // Although we already obtained code view's element, `diffTable`, from
        // the `fileRow` in the selector function above, we have to revisit the
        // `fileRow` to get the file path.
        const stickyArea = closestParentAcrossShadowRoots(diffTableElement, '.stickyArea')

        if (!stickyArea) {
            return null
        }

        const fileRow = stickyArea.querySelector('.file-row')
        const filePath = fileRow?.getAttribute('data-path')
        if (!filePath) {
            return null
        }

        const gerritChange = parseGerritChange()
        if (!gerritChange.patchsetId) {
            console.error('Sourcegraph: Cannot get Gerrit patchset ID')
            return null
        }
        const gerritChangeStringForHead = buildGerritChangeString(gerritChange.changeId, gerritChange.patchsetId)

        let parentCommit: string
        if (!gerritChange.basePatchsetId) {
            parentCommit = getParentCommit() || gerritChangeStringForHead + '^'
        } else {
            parentCommit = buildGerritChangeString(gerritChange.changeId, gerritChange.basePatchsetId)
        }

        injectCodeViewStyle(diffTableElement)

        return {
            element: diffTableElement,
            dom: diffTableDomFunctions,
            resolveFileInfo() {
                const fileInfo = {
                    base: {
                        filePath: filePath || '',
                        rawRepoName: gerritChange.repoName,
                        commitID: parentCommit,
                    },
                    head: {
                        filePath: filePath || '',
                        rawRepoName: gerritChange.repoName,
                        commitID: gerritChangeStringForHead,
                    },
                }
                console.log('resolveFileListCodeView: fileInfo', fileInfo)
                return fileInfo
            },
        }
    },
}

const getLineElementFromLineNumber: DOMFunctions['getCodeElementFromLineNumber'] = (codeView, line, part) => {
    const side = part === 'head' ? 'right' : 'left'
    // Split diff: line element is next sibling
    const lineNumberCell = codeView.querySelector(`td.lineNum.${side}[data-value="${line}"]`)
    if (!lineNumberCell) {
        return null
    }
    const contentCell = nextMatchingSibling(lineNumberCell, 'td.content')
    const codeElement = contentCell?.querySelector('.contentText')
    return codeElement as HTMLElement
}

const diffTableDomFunctions: DOMFunctions = {
    getLineElementFromLineNumber,
    getCodeElementFromLineNumber: getLineElementFromLineNumber,
    getCodeElementFromTarget: (target: HTMLElement) => {
        const codeElement = target.closest('td.content')
        // Check if we are on a line which has "File" in the line number cell.
        const fileElement = codeElement?.closest('tr')?.querySelector('[data-line-number]')
        if (fileElement) {
            const lineNumberString = fileElement.getAttribute('data-line-number')
            if (lineNumberString === 'FILE') {
                return null
            }
        }
        return codeElement as HTMLElement
    },
    getLineNumberFromCodeElement: (codeElement: HTMLElement): number => {
        const side = getSideFromCodeElement(codeElement)
        if (!side) {
            console.error(codeElement)
            throw new TypeError('Could not find line number (no side)')
        }
        const cell = codeElement.closest('td')
        const lineNumberCell = cell?.parentElement?.querySelector(`.lineNum.${side}`)
        const lineNumber = lineNumberCell?.getAttribute('data-value')
        if (!lineNumber) {
            throw new TypeError(`Could not find line number (${side})`)
        }
        return parseInt(lineNumber, 10)
    },
    getDiffCodePart: codeElement => {
        const side = getSideFromCodeElement(codeElement)
        if (side === 'left') {
            return 'base'
        }
        if (side === 'right') {
            return 'head'
        }
        console.error('Cannot tell: base or head')
        return 'base'
    },
}

const resolveFilePageCodeView: ViewResolver<CodeView> = {
    selector(target: HTMLElement) {
        // Because we expect only one code view to be present on an individual file
        // page, we don't have to consider the existing element (`target`) here in
        // the same way as in resolveFileListCodeView.
        const diffTableElement = querySelectorAcrossShadowRoots(
            document.body,
            '#app >> #app-element >> gr-diff-view >> #diffHost >> #diff >> #diffTable'
        )

        if (diffTableElement) {
            return [diffTableElement as HTMLElement]
        }
        return []
    },
    resolveView(element: HTMLElement): CodeView | null {
        const gerritChange = parseGerritChange()
        if (!gerritChange.patchsetId) {
            console.error('Sourcegraph: cannot find patchset ID')
            return null
        }
        const gerritChangeString = buildGerritChangeString(gerritChange.changeId, gerritChange.patchsetId)
        let baseCommit = gerritChangeString + '^' // Default fallback parent commit.
        if (gerritChange.basePatchsetId) {
            baseCommit = buildGerritChangeString(gerritChange.changeId, gerritChange.basePatchsetId)
        }

        injectCodeViewStyle(element)

        return {
            element,
            dom: diffTableDomFunctions,
            getToolbarMount(codeView) {
                const subheaderElement = getSubheaderFromCodeView(codeView)?.querySelector('.patchRangeLeft')
                if (!subheaderElement) {
                    throw new Error('Could not find subheader')
                }
                const existingMountElement = subheaderElement.querySelector('.sourcegraph-toolbar-mount')
                if (existingMountElement) {
                    return existingMountElement as HTMLElement
                }
                const mountElement = document.createElement('div')
                mountElement.classList.add('sourcegraph-toolbar-mount')
                subheaderElement.append(mountElement)
                subheaderElement.append(createStyleElement(toolbarStyles))
                return mountElement
            },
            resolveFileInfo() {
                const fileInfo = {
                    base: {
                        filePath: gerritChange.filePath || '',
                        rawRepoName: gerritChange.repoName,
                        commitID: baseCommit,
                    },
                    head: {
                        filePath: gerritChange.filePath || '',
                        rawRepoName: gerritChange.repoName,
                        commitID: gerritChangeString,
                    },
                }
                console.log('resolveFilePageCodeView: fileInfo', fileInfo)
                return fileInfo
            },
        }
    },
}

/** Storing  */
interface ElementWithFilePath {
    element: HTMLElement
    filePath: string
}

const POLLING_INTERVAL = 1500
export const observeMutations = (
    target: Node,
    options?: MutationObserverInit,
    paused?: Subject<boolean>
): Observable<MutationRecordLike[]> => {
    let knownElements: ElementWithFilePath[] = []
    return interval(POLLING_INTERVAL).pipe(
        map(() => {
            const { filePath = '' } = parseGerritChange()
            const selectors = [...codeViewResolvers.map(resolver => resolver.selector), toolbarSelector]
            const foundElements = selectors.map(selector => [...queryWithSelector(document.body, selector)]).flat()
            const addedNodes = foundElements.filter(
                foundElement => !find(knownElements, { element: foundElement, filePath })
            )
            const removedNodes = knownElements.filter(
                knownElement =>
                    !(
                        knownElement.filePath === filePath &&
                        foundElements.find(foundElement => foundElement === knownElement.element)
                    )
            )

            // Add to known elements
            for (const addedNode of addedNodes) {
                knownElements.push({ element: addedNode, filePath })
            }
            // Remove from known elements
            knownElements = knownElements.filter(knownElement => !find(removedNodes, knownElement))

            return { addedNodes, removedNodes: removedNodes.map(node => node.element) }
        }),
        // Filter to emit only non-empty records.
        filter(({ addedNodes, removedNodes }) => !!addedNodes.length || !!removedNodes.length),
        // Wrap in an array, because that's how mutation observers emit events.
        map(mutationRecord => [mutationRecord]),
        share({
            connector: () => new ReplaySubject(),
            resetOnError: false,
            resetOnComplete: false,
            resetOnRefCountZero: false,
        })
    )
}

const codeViewResolvers = [resolveFilePageCodeView, resolveFileListCodeView]
export const gerritCodeHost: CodeHost = {
    type: 'gerrit',
    name: 'Gerrit',
    codeViewResolvers,
    codeViewsRequireTokenization: true,
    // This overrides the default observeMutations because we need to handle shadow DOMS.
    observeMutations,
    getContext: async () => {
        const { repoName, changeId, patchsetId } = parseGerritChange()
        return Promise.resolve({
            privateRepository: true, // Gerrit is always private. Despite the fact that permissions can be set to be publicly viewable.
            rawRepoName: repoName,
            revision: patchsetId && buildGerritChangeString(changeId, patchsetId),
        })
    },
    check: checkIsGerrit,
    hoverOverlayClassProps: {
        className: styles.hoverOverlay,
    },
    codeViewToolbarClassProps: {
        className: 'code-view-toolbar--gerrit',
        actionItemIconClass: 'icon--gerrit',
    },
    viewOnSourcegraphButtonClassProps: {
        className: 'open-on-sourcegraph--gerrit',
        iconClassName: 'open-on-sourcegraph-icon--gerrit',
    },
    getViewContextOnSourcegraphMount: target => {
        const secondaryActionsElement = head(toolbarSelector(target))
        if (!secondaryActionsElement) {
            return null
        }
        const existingMountElement = secondaryActionsElement.querySelector('.open-in-sourcegraph-mount')
        if (existingMountElement) {
            return existingMountElement as HTMLElement
        }
        const mountElement = document.createElement('gr-button')
        mountElement.setAttribute('link', 'link')
        mountElement.classList.add('open-in-sourcegraph-mount')
        secondaryActionsElement.prepend(mountElement)
        secondaryActionsElement.append(createStyleElement(toolbarStyles))
        return mountElement
    },
}

const toolbarSelector: CustomSelectorFunction = () => {
    const toolbar = querySelectorAcrossShadowRoots(
        document.body,
        '#app >> #app-element >> gr-change-view >> #actions >> #secondaryActions'
    )
    return toolbar ? [toolbar as HTMLElement] : null
}

function getParentCommit(): string | null | undefined {
    const metadataPanel = querySelectorAcrossShadowRoots(
        document.body,
        '#app >> #app-element >> gr-change-view >> #metadata >> gr-commit-info'
    )
    if (!metadataPanel) {
        return null
    }
    return metadataPanel?.shadowRoot?.querySelector('.container')?.textContent?.trim()
}

function getDiffTypeFromCodeElement(codeElement: HTMLElement): 'side-by-side' | 'unified' {
    const rowElement = codeElement.closest('tr')
    if (rowElement?.matches('.unified')) {
        return 'unified'
    }
    return 'side-by-side'
}

function getSideFromCodeElement(codeElement: HTMLElement): 'left' | 'right' {
    const diffType = getDiffTypeFromCodeElement(codeElement)
    if (diffType === 'unified') {
        // If there is a line number cell for the right side, in this row, then we're on the right.
        // Otherwise we're on the left.
        const rightLineNumber = codeElement.closest('tr')?.querySelector('td.right')?.getAttribute('data-value')

        if (rightLineNumber) {
            return 'right'
        }
        return 'left'
    }

    // Side-by-side
    if (codeElement.closest('td')?.previousElementSibling?.matches('.right')) {
        return 'right'
    }
    return 'left'
}

/**
 * Return the next sibling element that matches the selector.
 */
function nextMatchingSibling(element: Element, selector: string): HTMLElement | null {
    const allSiblings = [...(element.parentElement?.childNodes || [])]
    const nextSiblings = allSiblings.slice(allSiblings.indexOf(element) + 1)
    for (const sibling of nextSiblings) {
        if (sibling instanceof HTMLElement) {
            if (sibling.matches(selector)) {
                return sibling
            }
        }
    }
    return null
}

/**
 * Returns a matching element, like `querySelector`, with support for the `>>` operator which drills into shadow roots.
 */
function querySelectorAcrossShadowRoots(element: ParentNode, selectors: string | string[]): Element | null {
    if (typeof selectors === 'string') {
        // The `>>` operator is a custom separator for shadow roots, used here to split the selector into an array.
        selectors = selectors.split('>>')
    }
    let currentElement: ParentNode | null = element
    const selectorsExceptLast = selectors.slice(0, -1)
    const lastSelector = selectors.at(-1)!
    for (const selector of selectorsExceptLast) {
        if (!currentElement) {
            return null
        }
        currentElement = currentElement.querySelector(selector)?.shadowRoot || null
    }
    return currentElement?.querySelector(lastSelector) as Element
}

/**
 * Find the closest matching parent element, like `element.closest`, with traversal up across shadow roots boundaries.
 */
function closestParentAcrossShadowRoots(element: Element, selector: string): Element | null {
    while (true) {
        const result = element.closest(selector)
        if (result) {
            return result
        }
        element = (element.getRootNode() as ShadowRoot)?.host
        if (!element || element === document.getRootNode()) {
            return null
        }
    }
}

function getSubheaderFromCodeView(codeView: HTMLElement): HTMLElement | null | undefined {
    const grDiffView = closestParentAcrossShadowRoots(codeView, 'gr-diff-view')
    const subheader = grDiffView?.shadowRoot?.querySelector('gr-fixed-panel .subHeader') as HTMLElement
    return subheader
}

function createStyleElement(styles: string): HTMLStyleElement {
    const styleElement = document.createElement('style')
    styleElement.textContent = styles
    return styleElement
}

function injectCodeViewStyle(element: HTMLElement): void {
    if (!element.querySelector('style.sourcegraph-injected-style')) {
        const styleElement = createStyleElement(codeViewStyles)
        styleElement.classList.add('sourcegraph-injected-style')
        element.append(styleElement)
    }
}

const toolbarStyles = `
.icon--gerrit {
    height: 1.3rem;
    width: 1.3rem;
    padding: 5px 4px;
}
.open-on-sourcegraph-icon--gerrit {
    height: 1rem;
    width: 1rem;
}
.open-on-sourcegraph--gerrit {
    text-decoration: none;
}
`

const codeViewStyles = `
.sourcegraph-document-highlight {
    background-color: var(--secondary);
}
.selection-highlight {
    background-color: var(--mark-bg);
}
`
