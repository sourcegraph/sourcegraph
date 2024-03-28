import { type Observable, throwError } from 'rxjs'
import { map } from 'rxjs/operators'

import type { PlatformContext } from '@sourcegraph/shared/src/platform/context'

import { type ChangeState, type DifferentialState, type DiffusionState, PhabricatorMode, type RevisionState } from '.'
import { getRepoDetailsFromCallsign, getRepoDetailsFromRevisionID, type QueryConduitHelper } from './backend'

const TAG_PATTERN = /r([\dA-z]+)([\da-f]{40})/
function matchPageTag(): RegExpExecArray | null {
    const element = document.querySelectorAll('.phui-tag-core').item(0)
    if (!element) {
        throw new Error('Could not find Phabricator page tag')
    }
    return TAG_PATTERN.exec(element.children[0].getAttribute('href') as string)
}

function getCallsignFromPageTag(): string {
    const match = matchPageTag()
    if (!match) {
        throw new Error('Could not determine callsign from page tag')
    }
    return match[1]
}

function getCommitIDFromPageTag(): string {
    const match = matchPageTag()
    if (!match) {
        throw new Error('Could not determine commitID from page tag')
    }
    return match[2]
}

const DIFF_PATTERN = /Diff (\d+)/
function getDiffIdFromDifferentialPage(): number {
    const diffsContainer = document.querySelector('#differential-review-stage')
    if (!diffsContainer) {
        throw new Error('no element with id differential-review-stage found on page.')
    }
    const wrappingDiffBox = diffsContainer.parentElement
    if (!wrappingDiffBox) {
        throw new Error('parent container of diff container not found.')
    }
    const diffTitle = wrappingDiffBox.children[0].querySelectorAll('.phui-header-header').item(0)
    if (!diffTitle?.textContent) {
        throw new Error('Could not find diffTitle element, or it had no text content')
    }
    const matches = DIFF_PATTERN.exec(diffTitle.textContent)
    if (!matches) {
        throw new Error(`diffTitle element does not match pattern. Content: '${diffTitle.textContent}'`)
    }
    return parseInt(matches[1], 10)
}

// https://phabricator.sgdev.org/source/gorilla/browse/master/mux.go
const PHAB_DIFFUSION_REGEX = /^\/?(source|diffusion)\/([\w-]+)\/browse\/([\w-]+\/)?([^$;]+)(;[\da-f]{40})?(?:\$\d+)?/i
// https://phabricator.sgdev.org/D2
const PHAB_DIFFERENTIAL_REGEX = /^\/?(d\d+)(?:\?(?:(?:id=(\d+))|(vs=(?:\d+|on)&id=\d+)))?/i
// https://phabricator.sgdev.org/rMUXfb619131e25d82897c9de11789aa479941cfd415
const PHAB_REVISION_REGEX = /^\/?r([\dA-z]+)([\da-f]{40})/i
// https://phabricator.sgdev.org/source/gorilla/change/master/mux.go
const PHAB_CHANGE_REGEX = /^\/?(source|diffusion)\/([\da-z]+)\/change\/([\w-]+)\/([^;]+)(;[\da-f]{40})?/i
const PHAB_CHANGESET_REGEX = /^\/?\/differential\/changeset.*/i
const COMPARISON_REGEX = /^vs=((?:\d+|on))&id=(\d+)/i

function getBaseCommitIDFromRevisionPage(): string {
    const keyElements = document.querySelectorAll('.phui-property-list-key')
    for (const keyElement of keyElements) {
        if (keyElement.textContent === 'Parents ') {
            const parentUrl = ((keyElement.nextSibling as HTMLElement).children[0].children[0] as HTMLLinkElement).href
            const url = new URL(parentUrl)
            const revisionMatch = PHAB_REVISION_REGEX.exec(url.pathname)
            if (revisionMatch) {
                return revisionMatch[2]
            }
        }
    }
    throw new Error('Could not determine base commit ID from revision page')
}

export function getPhabricatorState(
    location: URL | Location,
    requestGraphQL: PlatformContext['requestGraphQL'],
    queryConduit: QueryConduitHelper<any>
): Observable<DiffusionState | DifferentialState | RevisionState | ChangeState> {
    try {
        const stateUrl = location.href.replace(location.origin, '')
        const diffusionMatch = PHAB_DIFFUSION_REGEX.exec(stateUrl)
        if (diffusionMatch) {
            const filePath = diffusionMatch[4]
            if (!filePath) {
                throw new Error(`Could not determine file path from diffusionMatch, stateUrl: ${stateUrl}`)
            }
            const callsign = getCallsignFromPageTag()
            return getRepoDetailsFromCallsign(callsign, requestGraphQL, queryConduit).pipe(
                map(
                    ({ rawRepoName }): DiffusionState => ({
                        mode: PhabricatorMode.Diffusion,
                        rawRepoName,
                        filePath,
                        commitID: getCommitIDFromPageTag(),
                    })
                )
            )
        }
        const differentialMatch = PHAB_DIFFERENTIAL_REGEX.exec(stateUrl)
        if (differentialMatch) {
            const differentialID = differentialMatch[1]
            const comparison = differentialMatch[3]
            const revisionID = parseInt(differentialID.split('D')[1], 10)
            let diffID = differentialMatch[2] ? parseInt(differentialMatch[2], 10) : undefined
            if (!diffID) {
                diffID = getDiffIdFromDifferentialPage()
            }

            let baseDiffID: number | undefined
            if (comparison) {
                // urls that looks like this: http://phabricator.aws.sgdev.org/D3?vs=on&id=8&whitespace=ignore-most#toc
                const comparisonMatch = COMPARISON_REGEX.exec(comparison)
                const comparisonBase = comparisonMatch?.[1]
                if (comparisonBase && comparisonBase !== 'on') {
                    baseDiffID = parseInt(comparisonBase, 10)
                    console.log(`comparison diffID ${diffID} baseDiffID ${baseDiffID}`)
                }
            }

            return getRepoDetailsFromRevisionID(revisionID, requestGraphQL, queryConduit).pipe(
                map(
                    ({ rawRepoName }): DifferentialState => ({
                        baseRawRepoName: rawRepoName,
                        headRawRepoName: rawRepoName,
                        revisionID,
                        diffID: diffID!,
                        baseDiffID,
                        mode: PhabricatorMode.Differential,
                    })
                )
            )
        }

        const revisionMatch = PHAB_REVISION_REGEX.exec(stateUrl)
        if (revisionMatch) {
            const callsign = revisionMatch[1]
            const headCommitID = revisionMatch[2]
            const baseCommitID = getBaseCommitIDFromRevisionPage()
            return getRepoDetailsFromCallsign(callsign, requestGraphQL, queryConduit).pipe(
                map(
                    ({ rawRepoName }): RevisionState => ({
                        mode: PhabricatorMode.Revision,
                        rawRepoName,
                        baseCommitID,
                        headCommitID,
                    })
                )
            )
        }

        const changeMatch = PHAB_CHANGE_REGEX.exec(stateUrl)
        if (changeMatch) {
            const filePath = changeMatch[8]
            const callsign = getCallsignFromPageTag()
            const commitID = getCommitIDFromPageTag()
            return getRepoDetailsFromCallsign(callsign, requestGraphQL, queryConduit).pipe(
                map(
                    ({ rawRepoName }): ChangeState => ({
                        mode: PhabricatorMode.Change,
                        filePath,
                        rawRepoName,
                        commitID,
                    })
                )
            )
        }

        const changesetMatch = PHAB_CHANGESET_REGEX.exec(stateUrl)
        if (changesetMatch) {
            const crumbs = document.querySelector('.phui-crumbs-view')
            if (!crumbs) {
                throw new Error('failed parsing changeset dom')
            }

            const [, differentialHref, diffHref] = crumbs.querySelectorAll('a')

            const differentialMatch = differentialHref.getAttribute('href')!.match(/D(\d+)/)
            if (!differentialMatch) {
                throw new Error('failed parsing differentialID')
            }
            const revisionID = parseInt(differentialMatch[1], 10)

            const diffMatch = diffHref.getAttribute('href')!.match(/\/differential\/diff\/(\d+)/)
            if (!diffMatch) {
                throw new Error('failed parsing diffID')
            }
            const diffID = parseInt(diffMatch[1], 10)
            return getRepoDetailsFromRevisionID(revisionID, requestGraphQL, queryConduit).pipe(
                map(
                    ({ rawRepoName }): DifferentialState => ({
                        baseRawRepoName: rawRepoName,
                        headRawRepoName: rawRepoName,
                        revisionID,
                        diffID,
                        mode: PhabricatorMode.Differential,
                    })
                )
            )
        }

        throw new Error(`Could not determine Phabricator state from stateUrl ${stateUrl}`)
    } catch (error) {
        return throwError(() => error)
    }
}

export function normalizeRepoName(origin: string): string {
    let repoName = origin
    repoName = repoName.replace('\\', '')
    if (origin.startsWith('git@')) {
        repoName = origin.slice('git@'.length)
        repoName = repoName.replace(':', '/')
    } else if (origin.startsWith('git://')) {
        repoName = origin.slice('git://'.length)
    } else if (origin.startsWith('https://')) {
        repoName = origin.slice('https://'.length)
    } else if (origin.includes('@')) {
        // Assume the origin looks like `username@host:repo/path`
        const split = origin.split('@')
        repoName = split[1]
        repoName = repoName.replace(':', '/')
    }
    return repoName.replace(/.git$/, '')
}
