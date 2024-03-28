import { from, type Observable, of, throwError, lastValueFrom } from 'rxjs'
import { fromFetch } from 'rxjs/fetch'
import { map, switchMap, catchError } from 'rxjs/operators'

import { memoizeObservable } from '@sourcegraph/common'
import { dataOrThrowErrors, gql, checkOk } from '@sourcegraph/http-client'
import { isRepoNotFoundErrorLike } from '@sourcegraph/shared/src/backend/errors'
import type { PlatformContext } from '@sourcegraph/shared/src/platform/context'
import type { RepoSpec, FileSpec, ResolvedRevisionSpec } from '@sourcegraph/shared/src/util/url'

import { storage } from '../../../browser-extension/web-extension-api/storage'
import type { addPhabricatorRepoResult, ResolveStagingRevResult } from '../../../graphql-operations'
import { isExtension } from '../../context'
import { resolveRepo } from '../../repo/backend'

import type { RevisionSpec, DiffSpec, BaseDiffSpec } from '.'
import { normalizeRepoName } from './util'

interface PhabEntity {
    id: string // e.g. "48"
    type: string // e.g. "RHURI"
    phid: string // e.g. "PHID-RHURI-..."
}

interface ConduitURI extends PhabEntity {
    fields: {
        uri: {
            raw: string // e.g. https://secure.phabricator.com/source/phabricator.git",
            display: string // e.g. https://secure.phabricator.com/source/phabricator.git",
            effective: string // e.g. https://secure.phabricator.com/source/phabricator.git",
            normalized: string // e.g. secure.phabricator.com/source/phabricator",
            disabled: boolean
        }
    }
}

interface ConduitRepo extends PhabEntity {
    fields: {
        name: string
        vcs: string // e.g. 'git'
        callsign: string
        shortName: string
        status: 'active' | 'inactive'
    }
    attachments: {
        uris: {
            uris: ConduitURI[]
        }
    }
}

export interface ConduitReposResponse {
    data: ConduitRepo[]
}

interface ConduitReference {
    ref: string
    type: 'base' | 'diff'
    commit: string // a SHA
    remote: {
        uri: string
    }
}

interface ConduitDiffChange {
    oldPath: string
    currentPath: string
}

interface ConduitDiffDetails {
    branch: string
    sourceControlBaseRevision: string // the merge base commit
    description: string // e.g. 'rNZAP9bee3bc2cd3068dd97dfa87068c4431c5d6093ef'
    changes: ConduitDiffChange[]
    dateCreated: string
    authorName: string
    authorEmail: string
    properties: {
        'arc.staging': {
            status: string
            refs: ConduitReference[]
        }
        'local:commits': string[]
    }
}

interface ConduitDiffDetailsResponse {
    [id: string]: ConduitDiffDetails
}

/**
 * Creates the `FormData` used to pass parameters along with Conduit API requests,
 * including the CSRF token.
 */
function createConduitRequestForm(): FormData {
    const searchForm = document.querySelector('.phabricator-search-menu form')
    if (!searchForm) {
        throw new Error('cannot create conduit request form')
    }
    const form = new FormData()
    form.set('__csrf__', searchForm.querySelector<HTMLInputElement>('input[name=__csrf__]')!.value)
    form.set('__form__', searchForm.querySelector<HTMLInputElement>('input[name=__form__]')!.value)
    return form
}

/**
 * Native installation of the Phabricator extension does not allow for us to fetch the style.bundle from a script element.
 * To get around this we fetch the bundled CSS contents and append it to the DOM.
 */
export async function getPhabricatorCSS(cssURL: string): Promise<string> {
    const bundleUID = process.env.BUNDLE_UID!
    const response = await fetch(`${cssURL}?v=${bundleUID}`, {
        method: 'GET',
        credentials: 'include',
        headers: new Headers({ Accept: 'text/html' }),
    })
    return response.text()
}

type ConduitResponse<T> =
    | { error_code: null; error_info: null; result: T }
    | { error_code: string; error_info: string; result: null }

export type QueryConduitHelper<T> = (endpoint: string, parameters: {}) => Observable<T>

/**
 * Generic helper to query the Phabricator Conduit API.
 */
export function queryConduitHelper<T>(endpoint: string, parameters: {}): Observable<T> {
    const form = createConduitRequestForm()
    for (const [key, value] of Object.entries(parameters)) {
        form.set(`params[${key}]`, JSON.stringify(value))
    }
    return fromFetch(window.location.origin + endpoint, {
        method: 'POST',
        body: form,
        credentials: 'include',
        headers: {
            Accept: 'application/json',
        },
        selector: response => checkOk(response).json(),
    }).pipe(
        map((response: ConduitResponse<T>) => {
            if (response.error_code !== null) {
                throw new Error(`error ${response.error_code}: ${response.error_info}`)
            }
            return response.result
        })
    )
}

/**
 * Queries the Phabricator Conduit API for the {@link ConduitDiffDetails} matching the given
 * revision and diff IDs. {@link ConduitDiffDetails} notably contain the staging details for the diff,
 * including the base and head commit IDs on the staging repository.
 */
const getDiffDetailsFromConduit = memoizeObservable(
    ({
        diffID,
        revisionID,
        queryConduit = queryConduitHelper,
    }: RevisionSpec & DiffSpec & { queryConduit?: typeof queryConduitHelper }) =>
        queryConduit<ConduitDiffDetailsResponse>('/api/differential.querydiffs', {
            ids: [diffID],
            revisionIDs: [revisionID],
        }).pipe(map(diffDetails => diffDetails[String(diffID)])),
    ({ diffID, revisionID }) => `${diffID}-${revisionID}`
)

const getRawDiffFromConduit = memoizeObservable(
    ({ diffID, queryConduit = queryConduitHelper }: { diffID: number; queryConduit?: typeof queryConduitHelper }) =>
        queryConduit<string>('/api/differential.getrawdiff', { diffID }),
    ({ diffID }) => diffID.toString()
)

interface ConduitDifferentialQueryResponse {
    [index: string]: {
        repositoryPHID: string | null
    }
}

/**
 * Queries the Phabricator Conduit API for the PHID (Phabricator's opaque unique ID)
 * of the repository matching the given revisionID.
 */
const getRepoPHIDForRevisionID = memoizeObservable(
    ({
        revisionID,
        queryConduit = queryConduitHelper,
    }: {
        revisionID: number
        queryConduit?: typeof queryConduitHelper
    }) =>
        queryConduit<ConduitDifferentialQueryResponse>('/api/differential.query', { ids: [revisionID] }).pipe(
            map(result => {
                const phid = result['0'].repositoryPHID
                if (!phid) {
                    // This happens for revisions that were created without an associated repository
                    throw new Error(`no repositoryPHID for revision ${revisionID}`)
                }
                return phid
            })
        ),
    ({ revisionID }) => revisionID.toString()
)

interface CreatePhabricatorRepoOptions extends Pick<PlatformContext, 'requestGraphQL'> {
    callsign: string
    repoName: string
    phabricatorURL: string
}

const createPhabricatorRepo = memoizeObservable(
    ({ requestGraphQL, ...variables }: CreatePhabricatorRepoOptions): Observable<void> =>
        requestGraphQL<addPhabricatorRepoResult>({
            request: gql`
                mutation addPhabricatorRepo($callsign: String!, $repoName: String!, $phabricatorURL: String!) {
                    addPhabricatorRepo(callsign: $callsign, uri: $repoName, url: $phabricatorURL) {
                        alwaysNil
                    }
                }
            `,
            variables,
            mightContainPrivateInfo: true,
        }).pipe(map(() => undefined)),
    ({ callsign }) => callsign
)

interface PhabricatorRepoDetails {
    callsign: string
    rawRepoName: string
}

/**
 * Queries the Phabricator Conduit API for a repository matching the given callsign,
 * and emits the {@link PhabricatorRepoDetails} if found.
 */
export function getRepoDetailsFromCallsign(
    callsign: string,
    requestGraphQL: PlatformContext['requestGraphQL'],
    queryConduit: QueryConduitHelper<ConduitReposResponse>
): Observable<PhabricatorRepoDetails> {
    return queryConduit('/api/diffusion.repository.search', {
        constraints: { callsigns: [callsign] },
        attachments: { uris: true },
    }).pipe(
        switchMap(({ data }) => {
            const repo = data[0]
            if (!repo) {
                throw new Error(`could not locate repo with callsign ${callsign}`)
            }
            if (!repo.attachments?.uris) {
                throw new Error(`could not locate git uri for repo with callsign ${callsign}`)
            }
            return convertConduitRepoToRepoDetails(repo)
        }),
        switchMap((details: PhabricatorRepoDetails | null) => {
            if (!details) {
                return throwError(() => new Error('could not parse repo details'))
            }
            return createPhabricatorRepo({
                callsign,
                repoName: details.rawRepoName,
                phabricatorURL: window.location.origin,
                requestGraphQL,
            }).pipe(map(() => details))
        })
    )
}

/**
 * Queries the Phabricator Conduit API sourcegraph.configuration endpoint.
 *
 * The Phabricator extension updates the window object automatically, but in the
 * case it fails we query the conduit API.
 */
export function getSourcegraphURLFromConduit(): Promise<string> {
    return lastValueFrom(
        queryConduitHelper<{ url: string }>('/api/sourcegraph.configuration', {}).pipe(map(({ url }) => url))
    )
}

const getRepoDetailsFromRepoPHID = memoizeObservable(
    ({
        phid,
        requestGraphQL,
        queryConduit = queryConduitHelper,
    }: {
        phid: string
        requestGraphQL: PlatformContext['requestGraphQL']
        queryConduit?: typeof queryConduitHelper
    }) =>
        queryConduit<ConduitReposResponse>('/api/diffusion.repository.search', {
            constraints: {
                phids: [phid],
            },
            attachments: {
                uris: true,
            },
        }).pipe(
            switchMap(({ data }) => {
                const repo = data[0]
                if (!repo) {
                    throw new Error(`could not locate repo with phid ${phid}`)
                }
                if (!repo.attachments?.uris) {
                    throw new Error(`could not locate git uri for repo with phid ${phid}`)
                }
                return from(convertConduitRepoToRepoDetails(repo)).pipe(
                    switchMap((details: PhabricatorRepoDetails | null) => {
                        if (!details) {
                            return throwError(() => new Error('could not parse repo details'))
                        }
                        if (!repo.fields?.callsign) {
                            return throwError(() => new Error('callsign not found'))
                        }
                        return createPhabricatorRepo({
                            callsign: repo.fields.callsign,
                            repoName: details.rawRepoName,
                            phabricatorURL: window.location.origin,
                            requestGraphQL,
                        }).pipe(map(() => details))
                    })
                )
            })
        ),
    ({ phid }) => phid
)

/**
 * Queries the Phabricator Conduit API for a repository matching the given revisionID,
 * and emits the {@link PhabricatorRepoDetails} for that repository if found.
 */
export function getRepoDetailsFromRevisionID(
    revisionID: number,
    requestGraphQL: PlatformContext['requestGraphQL'],
    queryConduit = queryConduitHelper
): Observable<PhabricatorRepoDetails> {
    return getRepoPHIDForRevisionID({ revisionID, queryConduit }).pipe(
        switchMap(phid => getRepoDetailsFromRepoPHID({ phid, requestGraphQL, queryConduit }))
    )
}

async function convertConduitRepoToRepoDetails(repo: ConduitRepo): Promise<PhabricatorRepoDetails | null> {
    if (isExtension) {
        const items = await storage.managed.get()
        if (items.phabricatorMappings) {
            for (const mapping of items.phabricatorMappings) {
                if (mapping.callsign === repo.fields.callsign) {
                    return {
                        callsign: repo.fields.callsign,
                        rawRepoName: mapping.path,
                    }
                }
            }
        }
        return convertToDetails(repo)
    }
    // The path to a phabricator repository on a Sourcegraph instance may differ than it's URI / name from the
    // phabricator conduit API. Since we do not currently send the PHID with the Phabricator repository this a
    // backwards work around configuration setting to ensure mappings are correct. This logic currently exists
    // in the browser extension options menu.
    type Mappings = { callsign: string; path: string }[]
    const mappingsString = window.localStorage.getItem('PHABRICATOR_CALLSIGN_MAPPINGS')
    const callsignMappings = mappingsString
        ? (JSON.parse(mappingsString) as Mappings)
        : window.PHABRICATOR_CALLSIGN_MAPPINGS || []
    const details = convertToDetails(repo)
    if (callsignMappings) {
        for (const mapping of callsignMappings) {
            if (mapping.callsign === repo.fields.callsign) {
                return {
                    callsign: repo.fields.callsign,
                    rawRepoName: mapping.path,
                }
            }
        }
    }
    return details
}

function convertToDetails(repo: ConduitRepo): PhabricatorRepoDetails | null {
    const enabledURIs = repo.attachments.uris.uris
        // Filter out disabled URIs
        .filter(({ fields }) => !fields.uri.disabled)
        .map(({ fields }) => ({
            isExternalURI: !fields.uri.normalized.replace('\\', '').startsWith(window.location.host + '/'),
            rawURI: fields.uri.raw,
        }))
    if (enabledURIs.length === 0) {
        return null
    }
    // Use the external URI if there is one, otherwise use the first enabled URI.
    const { rawURI } = enabledURIs.find(({ isExternalURI }) => isExternalURI) ?? enabledURIs[0]
    const rawRepoName = normalizeRepoName(rawURI)
    return { callsign: repo.fields.callsign, rawRepoName }
}

interface ResolveStagingOptions extends Pick<PlatformContext, 'requestGraphQL'>, RepoSpec, DiffSpec {
    baseRev: string
    patch?: string
    date?: string
    authorName?: string
    authorEmail?: string
    description?: string
}

/**
 * Returns the commit ID of the one-off commit created on the Sourcegraph instance for the given
 * repo/diffID/patch, creating that commit if needed.
 */
const resolveStagingRevision = ({
    requestGraphQL,
    ...variables
}: ResolveStagingOptions): Observable<ResolvedRevisionSpec> =>
    requestGraphQL<ResolveStagingRevResult>({
        request: gql`
            mutation ResolveStagingRev(
                $repoName: String!
                $diffID: ID!
                $baseRev: String!
                $patch: String
                $date: String
                $authorName: String
                $authorEmail: String
                $description: String
            ) {
                resolvePhabricatorDiff(
                    repoName: $repoName
                    diffID: $diffID
                    baseRev: $baseRev
                    patch: $patch
                    date: $date
                    authorName: $authorName
                    authorEmail: $authorEmail
                    description: $description
                ) {
                    oid
                }
            }
        `,
        variables,
        mightContainPrivateInfo: true,
    }).pipe(
        map(dataOrThrowErrors),
        map(({ resolvePhabricatorDiff }) => {
            if (!resolvePhabricatorDiff) {
                throw new Error('Empty resolvePhabricatorDiff')
            }
            const { oid } = resolvePhabricatorDiff
            if (!oid) {
                throw new Error('Could not resolve staging revision: empty oid')
            }
            return { commitID: oid }
        })
    )

function hasThisFileChanged(filePath: string, changes: ConduitDiffChange[]): boolean {
    for (const change of changes) {
        if (change.currentPath === filePath) {
            return true
        }
    }
    return false
}

interface ResolveDiffOptions extends RepoSpec, FileSpec, RevisionSpec, DiffSpec, BaseDiffSpec {
    isBase: boolean
    useDiffForBase: boolean // indicates whether the base should use the diff commit
    useBaseForDiff: boolean // indicates whether the diff should use the base commit
}

interface PropsWithDiffDetails extends ResolveDiffOptions {
    diffDetails: ConduitDiffDetails
}

function getPropsWithDiffDetails(
    props: ResolveDiffOptions,
    queryConduit: QueryConduitHelper<any>
): Observable<PropsWithDiffDetails> {
    return getDiffDetailsFromConduit({ ...props, queryConduit }).pipe(
        switchMap(diffDetails => {
            if (props.isBase || !props.baseDiffID || hasThisFileChanged(props.filePath, diffDetails.changes)) {
                // no need to update props
                return of({
                    ...props,
                    diffDetails,
                })
            }
            return getDiffDetailsFromConduit({ ...props, queryConduit }).pipe(
                map(
                    (diffDetails): PropsWithDiffDetails => ({
                        ...props,
                        diffDetails,
                        diffID: props.baseDiffID!,
                        useBaseForDiff: true,
                    })
                )
            )
        })
    )
}

function getStagingDetails(
    propsWithInfo: PropsWithDiffDetails
): { repoName: string; ref: ConduitReference; unconfigured: boolean } | undefined {
    const stagingInfo = propsWithInfo.diffDetails.properties['arc.staging']
    if (!stagingInfo) {
        return undefined
    }
    let key: string
    if (propsWithInfo.isBase) {
        const type = propsWithInfo.useDiffForBase ? 'diff' : 'base'
        key = `refs/tags/phabricator/${type}/${propsWithInfo.diffID}`
    } else {
        const type = propsWithInfo.useBaseForDiff ? 'base' : 'diff'
        key = `refs/tags/phabricator/${type}/${propsWithInfo.diffID}`
    }
    for (const reference of propsWithInfo.diffDetails.properties['arc.staging'].refs) {
        if (reference.ref === key) {
            const remote = reference.remote.uri
            if (remote) {
                return {
                    repoName: normalizeRepoName(remote),
                    ref: reference,
                    unconfigured: stagingInfo.status === 'repository.unconfigured',
                }
            }
        }
    }
    return undefined
}

interface ResolvedDiff extends ResolvedRevisionSpec {
    /**
     * The name of the staging repository, if it is synced to the Sourcegraph instance.
     */
    stagingRepoName?: string
}

/**
 * Emits the {@link ResolvedDiff} for the base or head commit of a Phabricator diff.
 * - If possible, the base commit from the source control repository will be used.
 * - If a staging repository is configured and is synced to the Sourcegraph instance,
 * the commit ID on the staging repository will be returned, and the {@link ResolvedDiff}
 * will include the `stagingRepoName`.
 * - If a staging repository is configured but it isn't synced to the Sourcegraph instance,
 * a one-off staging commit will be created from the raw diff on the Sourcegraph instance,
 * and its commit ID will be returned ({@see resolveStagingRev}).
 * - If no staging repository is configured, and the commit doesn't exist on the Sourcegraph instance
 * (for example in the case of a revision created through the Phabricator UI from a raw diff), a one-off
 * staging commit will be created from the raw diff on the Sourcegraph instance, and its commit ID will
 * be returned ({@see resolveStagingRev}).
 *
 */
export function resolveDiffRevision(
    props: ResolveDiffOptions,
    requestGraphQL: PlatformContext['requestGraphQL'],
    queryConduit: QueryConduitHelper<any>
): Observable<ResolvedDiff> {
    return getPropsWithDiffDetails(props, queryConduit).pipe(
        switchMap(({ diffDetails, ...props }) => {
            const stagingDetails = getStagingDetails({ diffDetails, ...props })
            const conduitProps = {
                repoName: props.repoName,
                diffID: props.diffID,
                baseRev: diffDetails.sourceControlBaseRevision,
                date: diffDetails.dateCreated,
                authorName: diffDetails.authorName,
                authorEmail: diffDetails.authorEmail,
                description: diffDetails.description,
            }

            // When resolving the base, use the commit ID from the diff details.
            if (props.isBase && !props.useDiffForBase) {
                return of({
                    commitID: diffDetails.sourceControlBaseRevision,
                })
            }
            if (!stagingDetails || stagingDetails.unconfigured) {
                // If there are no staging details, get the patch from the conduit API,
                // create a one-off commit on the Sourcegraph instance from the patch,
                // and resolve to the commit ID returned by the Sourcegraph instance.
                return getRawDiffFromConduit({ diffID: props.diffID, queryConduit }).pipe(
                    switchMap(patch => resolveStagingRevision({ ...conduitProps, patch, requestGraphQL }))
                )
            }

            // If staging details are configured, first check if the repo is present on the Sourcegraph instance.
            return resolveRepo({ rawRepoName: stagingDetails.repoName, requestGraphQL }).pipe(
                // If the repo is present on the Sourcegraph instance,
                // use the commitID and repo name from the staging details.
                map(() => ({
                    commitID: stagingDetails.ref.commit,
                    stagingRepoName: stagingDetails.repoName,
                })),
                // Otherwise, create a one-off commit containing the patch on the Sourcegraph instance,
                // and resolve to the commit ID returned by the Sourcegraph instance.
                catchError(error => {
                    if (!isRepoNotFoundErrorLike(error)) {
                        throw error
                    }
                    return getRawDiffFromConduit({ diffID: props.diffID, queryConduit }).pipe(
                        switchMap(patch => resolveStagingRevision({ ...conduitProps, patch, requestGraphQL }))
                    )
                })
            )
        })
    )
}
