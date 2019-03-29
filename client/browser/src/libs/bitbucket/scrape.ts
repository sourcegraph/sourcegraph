import { DiffResolvedRevSpec } from '../../shared/repo'
import { FileInfo } from '../code_intelligence'

interface PageInfo extends Pick<FileInfo, 'repoName' | 'filePath' | 'rev'> {
    project: string
    repoSlug: string
}

const LINK_SELECTORS = ['a.raw-view-link', 'a.source-view-link', 'a.mode-source']

/**
 * Attempts to parse the file info from a link element contained in the code view.
 * Depending on the configuration of the page, this can be a link to the raw file,
 * or to the original source view, so a few different selectors are tried.
 *
 * The href of these links contains:
 * - project name
 * - repo name
 * - file path
 * - rev (through the query parameter 'at')
 */
const getFileInfoFromLink = (codeView: HTMLElement): PageInfo => {
    const pageInfo = LINK_SELECTORS.map(
        (selector): PageInfo | null => {
            const linkElement = codeView.querySelector(selector) as HTMLLinkElement | null
            if (!linkElement) {
                return null
            }
            const url = new URL(linkElement.href)
            const host = window.location.hostname
            const path = url.pathname

            // Looks like /projects/<project>/repos/<repo>/(browse|raw)/<file path>?at=<rev>
            const pathMatch = path.match(/\/projects\/(.*?)\/repos\/(.*?)\/(browse|raw)\/(.*)$/)
            if (!pathMatch) {
                return null
            }

            const project = pathMatch[1]
            const repoSlug = pathMatch[2]
            const filePath = pathMatch[4]

            // Looks like 'refs/heads/<rev>'
            const at = url.searchParams.get('at')
            if (!at) {
                return null
            }

            const atMatch = at.match(/refs\/heads\/(.*?)$/)

            const rev = atMatch ? atMatch[1] : at

            return {
                repoName: [host, project, repoSlug].join('/'),
                filePath,
                rev,
                project,
                repoSlug,
            }
        }
    ).find(value => value !== null)
    if (!pageInfo) {
        throw new Error('Could not parse page info from link element')
    } else {
        return pageInfo
    }
}

const getCommitIDFromLink = (): string | null => {
    const commitLink = document.querySelector<HTMLElement>('a.commitid')
    if (!commitLink) {
        return null
    }

    return commitLink.dataset.commitid!
}

export const getFileInfoFromCodeView = (codeView: HTMLElement): PageInfo & Pick<FileInfo, 'commitID'> => {
    const { repoName, filePath, rev, project, repoSlug } = getFileInfoFromLink(codeView)
    const commitID = getCommitIDFromLink()
    if (!commitID) {
        throw new Error('Could not find commit ID from link')
    }

    return {
        repoName,
        filePath,
        rev,
        commitID,
        project,
        repoSlug,
    }
}

const getFileInfoFromFilePathLink = (codeView: HTMLElement) => {
    const rawViewLink = codeView.querySelector<HTMLAnchorElement>('.breadcrumbs a.stub')
    if (!rawViewLink) {
        throw new Error('could not find raw view link for code view (.breadcrumbs a.stub)')
    }

    const url = new URL(rawViewLink.href)

    const host = window.location.hostname

    const path = url.pathname

    const pathMatch = path.match(/\/projects\/(.*?)\/repos\/(.*?)\/pull-requests\/(\d*)\//)
    if (!pathMatch) {
        throw new Error('Unable to parse file information')
    }

    const project = pathMatch[1]
    const repoSlug = pathMatch[2]

    const commitMatch = path.match(/\/commits\/(.*?)$/)

    const commitID = commitMatch ? commitMatch[1] : undefined

    let filePath = url.hash.replace(/^#/, '')
    filePath = filePath.replace(/\?.*$/, '')

    return {
        repoName: [host, project, repoSlug].join('/'),
        filePath,
        commitID,
        project,
        repoSlug,
    }
}

export interface PRPageInfo extends PageInfo {
    prID?: number
    commitID?: string // FileInfo.commitID is required but we won't always have it from the PR page DOM.
}

export const getPRInfoFromCodeView = (codeView: HTMLElement): PRPageInfo => {
    let repoName: string
    let filePath: string
    let project: string
    let repoSlug: string
    let commitID: string | null | undefined

    try {
        const info = getFileInfoFromLink(codeView)
        repoName = info.repoName
        filePath = info.filePath
        project = info.project
        repoSlug = info.repoSlug
    } catch (e) {
        const info = getFileInfoFromFilePathLink(codeView)

        repoName = info.repoName
        filePath = info.filePath
        project = info.project
        repoSlug = info.repoSlug
        commitID = info.commitID
    }

    const prIDMatch = window.location.pathname.match(/pull-requests\/(\d*?)\/(diff|overview|commits)/)

    if (!commitID) {
        commitID = getCommitIDFromLink()
    }

    return {
        repoName: repoName!,
        filePath: filePath!,
        commitID: commitID!,
        prID: prIDMatch ? parseInt(prIDMatch[1], 10) : undefined,
        project: project!,
        repoSlug: repoSlug!,
    }
}

export function getResolvedDiffFromBranchComparePage(): DiffResolvedRevSpec {
    const headCommitElement = document.querySelector('#branch-compare .source-selector a.commitid[data-commitid]')
    const baseCommitElement = document.querySelector('#branch-compare .target-selector a.commitid[data-commitid]')
    if (!headCommitElement || !baseCommitElement) {
        throw new Error('Could not resolve Bitbucket compare diff spec')
    }
    return {
        headCommitID: headCommitElement.getAttribute('data-commitid')!,
        baseCommitID: baseCommitElement.getAttribute('data-commitid')!,
    }
}
