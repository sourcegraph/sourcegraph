import { from, type Observable, of, lastValueFrom, firstValueFrom } from 'rxjs'
import { first } from 'rxjs/operators'
import { TestScheduler } from 'rxjs/testing'
import * as sinon from 'sinon'
import type * as sourcegraph from 'sourcegraph'
import { beforeEach, describe, expect, it } from 'vitest'

import type { TextDocumentPositionParameters } from '@sourcegraph/client-api'
import { type HoveredToken, LOADER_DELAY, type MaybeLoadingResult } from '@sourcegraph/codeintellify'
import { resetAllMemoizationCaches } from '@sourcegraph/common'
import { Position, Range } from '@sourcegraph/extension-api-classes'
import type { Location } from '@sourcegraph/extension-api-types'
import type { GraphQLResult, SuccessGraphQLResult } from '@sourcegraph/http-client'

import type { WorkspaceRootWithMetadata } from '../api/extension/extensionHostApi'
import type { PlatformContext, URLToFileContext } from '../platform/context'
import {
    type FileSpec,
    type UIPositionSpec,
    type RawRepoSpec,
    type RepoSpec,
    type RevisionSpec,
    type ViewStateSpec,
    toPrettyBlobURL,
} from '../util/url'

import { getDefinitionURL, getHoverActionsContext, type HoverActionsContext } from './actions'
import type { HoverContext } from './HoverOverlay'

const FIXTURE_PARAMS: TextDocumentPositionParameters & URLToFileContext = {
    textDocument: { uri: 'git://r?c#f' },
    position: { line: 1, character: 1 },
    part: undefined,
}

const FIXTURE_LOCATION: sourcegraph.Location = {
    uri: new URL('git://r2?c2#f2'),
    range: new Range(new Position(2, 2), new Position(3, 3)),
}

const FIXTURE_LOCATION_CLIENT: Location = {
    uri: 'git://r2?c2#f2',
    range: {
        start: { line: 2, character: 2 },
        end: { line: 3, character: 3 },
    },
}

const FIXTURE_HOVER_CONTEXT: HoveredToken & HoverContext = {
    repoName: 'r',
    commitID: 'c',
    revision: 'v',
    filePath: 'f',
    line: 2,
    character: 2,
}

const FIXTURE_WORKSPACE: WorkspaceRootWithMetadata = { uri: 'git://r3?c3', inputRevision: 'v3' }

// Use toPrettyBlobURL as the urlToFile passed to these functions because it results in the most readable/familiar
// expected test output.
// Some tests may override this with .callsFake()
let urlToFile!: sinon.SinonStub<Parameters<PlatformContext['urlToFile']>, string>
beforeEach(() => {
    urlToFile = sinon.stub<Parameters<PlatformContext['urlToFile']>, string>().callsFake(toPrettyBlobURL)
})

const requestGraphQL: PlatformContext['requestGraphQL'] = <R>({ variables }: { variables: { [key: string]: any } }) =>
    // eslint-disable-next-line @typescript-eslint/consistent-type-assertions
    of({
        data: {
            repository: {
                uri: variables.repoName,
                mirrorInfo: {
                    cloned: true,
                },
            },
        },
    } as SuccessGraphQLResult<any>)

const scheduler = (): TestScheduler => new TestScheduler((actual, expected) => expect(actual).toStrictEqual(expected))

describe('getHoverActionsContext', () => {
    beforeEach(() => resetAllMemoizationCaches())
    it('shows a loader for the definition if slow', () => {
        scheduler().run(({ cold, expectObservable }) =>
            expectObservable(
                from(
                    getHoverActionsContext(
                        {
                            getDefinition: () =>
                                cold<MaybeLoadingResult<Location[]>>(`l ${LOADER_DELAY + 100}ms r`, {
                                    l: { isLoading: true, result: [] },
                                    r: { isLoading: false, result: [FIXTURE_LOCATION_CLIENT] },
                                }),
                            getWorkspaceRoots: () => cold<WorkspaceRootWithMetadata[]>('w', { w: [FIXTURE_WORKSPACE] }),
                            hasReferenceProvidersForDocument: () => cold<boolean>('a', { a: true }),
                            platformContext: { urlToFile, requestGraphQL },
                        },
                        FIXTURE_HOVER_CONTEXT
                    )
                )
            ).toBe(
                `n ${LOADER_DELAY - 1}ms (lf) 97ms g`,
                ((): {
                    [key: string]: HoverActionsContext
                } => ({
                    // Show nothing
                    n: {
                        'goToDefinition.showLoading': false,
                        'goToDefinition.url': null,
                        'goToDefinition.notFound': false,
                        'goToDefinition.error': false,
                        'findReferences.url': null,
                        'panel.url': '/r@v/-/blob/f?L2:2#tab=panelID',
                        hoverPosition: FIXTURE_PARAMS,
                        hoveredOnDefinition: false,
                    },
                    // Show loader
                    l: {
                        'goToDefinition.showLoading': true,
                        'goToDefinition.url': null,
                        'goToDefinition.notFound': false,
                        'goToDefinition.error': false,
                        'findReferences.url': null,
                        'panel.url': '/r@v/-/blob/f?L2:2#tab=panelID',
                        hoverPosition: FIXTURE_PARAMS,
                        hoveredOnDefinition: false,
                    },
                    // Show find references button (same tick)
                    f: {
                        'goToDefinition.showLoading': true,
                        'goToDefinition.url': null,
                        'goToDefinition.notFound': false,
                        'goToDefinition.error': false,
                        'findReferences.url': '/r@v/-/blob/f?L2:2#tab=references',
                        'panel.url': '/r@v/-/blob/f?L2:2#tab=panelID',
                        hoverPosition: FIXTURE_PARAMS,
                        hoveredOnDefinition: false,
                    },
                    // Show go to definition button, hide loader, show find references button
                    g: {
                        'goToDefinition.showLoading': false,
                        'goToDefinition.url': '/r2@c2/-/blob/f2?L3:3',
                        'goToDefinition.notFound': false,
                        'goToDefinition.error': false,
                        'findReferences.url': '/r@v/-/blob/f?L2:2#tab=references',
                        'panel.url': '/r@v/-/blob/f?L2:2#tab=panelID',
                        hoverPosition: FIXTURE_PARAMS,
                        hoveredOnDefinition: false,
                    },
                }))()
            )
        )
    })

    it('shows a loader when definition providers are registered after invocation and a find-references button after the result returned', () => {
        scheduler().run(({ cold, expectObservable }) =>
            expectObservable(
                from(
                    getHoverActionsContext(
                        {
                            getDefinition: () =>
                                cold<MaybeLoadingResult<Location[]>>(`l e 50ms l ${LOADER_DELAY}ms r`, {
                                    l: { isLoading: true, result: [] },
                                    e: { isLoading: false, result: [] },
                                    r: { isLoading: false, result: [FIXTURE_LOCATION_CLIENT] },
                                }),
                            getWorkspaceRoots: () => cold<WorkspaceRootWithMetadata[]>('w', { w: [FIXTURE_WORKSPACE] }),
                            hasReferenceProvidersForDocument: () => cold<boolean>('a', { a: true }),
                            platformContext: { urlToFile, requestGraphQL },
                        },
                        FIXTURE_HOVER_CONTEXT
                    )
                )
            ).toBe(
                `e n ${LOADER_DELAY - 2}ms (lf) 49ms g`,
                ((): { [key: string]: HoverActionsContext } => ({
                    // Show nothing
                    e: {
                        'goToDefinition.showLoading': false,
                        'goToDefinition.url': null,
                        'goToDefinition.notFound': false,
                        'goToDefinition.error': false,
                        'findReferences.url': null,
                        'panel.url': '/r@v/-/blob/f?L2:2#tab=panelID',
                        hoverPosition: FIXTURE_PARAMS,
                        hoveredOnDefinition: false,
                    },
                    // Not found
                    n: {
                        'goToDefinition.showLoading': false,
                        'goToDefinition.url': null,
                        'goToDefinition.notFound': true,
                        'goToDefinition.error': false,
                        'findReferences.url': null,
                        'panel.url': '/r@v/-/blob/f?L2:2#tab=panelID',
                        hoverPosition: FIXTURE_PARAMS,
                        hoveredOnDefinition: false,
                    },
                    // Show loader
                    l: {
                        'goToDefinition.showLoading': true,
                        'goToDefinition.url': null,
                        'goToDefinition.notFound': false,
                        'goToDefinition.error': false,
                        'findReferences.url': null,
                        'panel.url': '/r@v/-/blob/f?L2:2#tab=panelID',
                        hoverPosition: FIXTURE_PARAMS,
                        hoveredOnDefinition: false,
                    },
                    // Show find references button (same tick)
                    f: {
                        'goToDefinition.showLoading': true,
                        'goToDefinition.url': null,
                        'goToDefinition.notFound': false,
                        'goToDefinition.error': false,
                        'findReferences.url': '/r@v/-/blob/f?L2:2#tab=references',
                        'panel.url': '/r@v/-/blob/f?L2:2#tab=panelID',
                        hoverPosition: FIXTURE_PARAMS,
                        hoveredOnDefinition: false,
                    },
                    // Show go to definition button, hide loader, show find references button
                    g: {
                        'goToDefinition.showLoading': false,
                        'goToDefinition.url': '/r2@c2/-/blob/f2?L3:3',
                        'goToDefinition.notFound': false,
                        'goToDefinition.error': false,
                        'findReferences.url': '/r@v/-/blob/f?L2:2#tab=references',
                        'panel.url': '/r@v/-/blob/f?L2:2#tab=panelID',
                        hoverPosition: FIXTURE_PARAMS,
                        hoveredOnDefinition: false,
                    },
                }))()
            )
        )
    })

    it('shows the find references button when reference providers are registered later', () => {
        scheduler().run(({ cold, expectObservable }) =>
            expectObservable(
                from(
                    getHoverActionsContext(
                        {
                            getDefinition: () =>
                                cold<MaybeLoadingResult<Location[]>>('-b', {
                                    b: { isLoading: false, result: [FIXTURE_LOCATION_CLIENT] },
                                }),
                            getWorkspaceRoots: () => cold<WorkspaceRootWithMetadata[]>('w', { w: [FIXTURE_WORKSPACE] }),
                            hasReferenceProvidersForDocument: () => cold<boolean>('a 10ms b', { a: false, b: true }),
                            platformContext: { urlToFile, requestGraphQL },
                        },
                        FIXTURE_HOVER_CONTEXT
                    )
                )
                // eslint-disable-next-line @typescript-eslint/consistent-type-assertions
            ).toBe('ng 9ms f', {
                // Nothing
                n: {
                    'goToDefinition.showLoading': false,
                    'goToDefinition.url': null,
                    'goToDefinition.notFound': false,
                    'goToDefinition.error': false,
                    'findReferences.url': null,
                    'panel.url': '/r@v/-/blob/f?L2:2#tab=panelID',
                    hoverPosition: FIXTURE_PARAMS,
                    hoveredOnDefinition: false,
                },
                // Go to definition
                g: {
                    'goToDefinition.showLoading': false,
                    'goToDefinition.url': '/r2@c2/-/blob/f2?L3:3',
                    'goToDefinition.notFound': false,
                    'goToDefinition.error': false,
                    'findReferences.url': null,
                    'panel.url': '/r@v/-/blob/f?L2:2#tab=panelID',
                    hoverPosition: FIXTURE_PARAMS,
                    hoveredOnDefinition: false,
                },
                // Find references
                f: {
                    'goToDefinition.showLoading': false,
                    'goToDefinition.url': '/r2@c2/-/blob/f2?L3:3',
                    'goToDefinition.notFound': false,
                    'goToDefinition.error': false,
                    'findReferences.url': '/r@v/-/blob/f?L2:2#tab=references',
                    'panel.url': '/r@v/-/blob/f?L2:2#tab=panelID',
                    hoverPosition: FIXTURE_PARAMS,
                    hoveredOnDefinition: false,
                },
            } as {
                [key: string]: HoverActionsContext
            })
        )
    })

    it('shows no loader for the definition if fast', () => {
        scheduler().run(({ cold, expectObservable }) =>
            expectObservable(
                from(
                    getHoverActionsContext(
                        {
                            getDefinition: () =>
                                cold<MaybeLoadingResult<Location[]>>('-b', {
                                    b: { isLoading: false, result: [FIXTURE_LOCATION_CLIENT] },
                                }),
                            getWorkspaceRoots: () => cold<WorkspaceRootWithMetadata[]>('w', { w: [FIXTURE_WORKSPACE] }),
                            hasReferenceProvidersForDocument: () => cold<boolean>('a', { a: true }),
                            platformContext: { urlToFile, requestGraphQL },
                        },
                        FIXTURE_HOVER_CONTEXT
                    )
                )
                // eslint-disable-next-line @typescript-eslint/consistent-type-assertions
            ).toBe('n(gf)', {
                // Nothing
                n: {
                    'goToDefinition.showLoading': false,
                    'goToDefinition.url': null,
                    'goToDefinition.notFound': false,
                    'goToDefinition.error': false,
                    'findReferences.url': null,
                    'panel.url': '/r@v/-/blob/f?L2:2#tab=panelID',
                    hoverPosition: FIXTURE_PARAMS,
                    hoveredOnDefinition: false,
                },
                // Go to definition
                g: {
                    'goToDefinition.showLoading': false,
                    'goToDefinition.url': '/r2@c2/-/blob/f2?L3:3',
                    'goToDefinition.notFound': false,
                    'goToDefinition.error': false,
                    'findReferences.url': null,
                    'panel.url': '/r@v/-/blob/f?L2:2#tab=panelID',
                    hoverPosition: FIXTURE_PARAMS,
                    hoveredOnDefinition: false,
                },
                // Find references button
                f: {
                    'goToDefinition.showLoading': false,
                    'goToDefinition.url': '/r2@c2/-/blob/f2?L3:3',
                    'goToDefinition.notFound': false,
                    'goToDefinition.error': false,
                    'findReferences.url': '/r@v/-/blob/f?L2:2#tab=references',
                    'panel.url': '/r@v/-/blob/f?L2:2#tab=panelID',
                    hoverPosition: FIXTURE_PARAMS,
                    hoveredOnDefinition: false,
                },
            } as {
                [key: string]: HoverActionsContext
            })
        )
    })
})

describe('getDefinitionURL', () => {
    beforeEach(() => resetAllMemoizationCaches())

    it('emits null if the locations result is empty', () =>
        expect(
            lastValueFrom(
                of({ isLoading: false, result: [] }).pipe(
                    getDefinitionURL(
                        { urlToFile, requestGraphQL },
                        {
                            getWorkspaceRoots: () => of([FIXTURE_WORKSPACE]),
                        },
                        FIXTURE_PARAMS
                    ),
                    first(({ isLoading }) => !isLoading)
                )
            )
        ).resolves.toStrictEqual({ isLoading: false, result: null }))

    describe('if there is exactly 1 location result', () => {
        it('resolves the raw repo name and passes it to urlToFile()', async () => {
            const requestGraphQL = <R>({ variables }: { variables: any }): Observable<GraphQLResult<R>> =>
                // eslint-disable-next-line @typescript-eslint/consistent-type-assertions
                of({
                    data: {
                        repository: {
                            uri: `github.com/${variables.repoName as string}`,
                            mirrorInfo: {
                                cloned: true,
                            },
                        },
                    },
                } as SuccessGraphQLResult<any>)
            const urlToFile = sinon.spy(
                (
                    _location: RepoSpec &
                        Partial<RawRepoSpec> &
                        RevisionSpec &
                        FileSpec &
                        Partial<UIPositionSpec> &
                        Partial<ViewStateSpec>
                ) => ''
            )
            await lastValueFrom(
                of({
                    isLoading: false,
                    result: [{ uri: 'git://r3?c3#f' }],
                }).pipe(
                    getDefinitionURL(
                        { urlToFile, requestGraphQL },
                        {
                            getWorkspaceRoots: () => of([FIXTURE_WORKSPACE]),
                        },
                        FIXTURE_PARAMS
                    ),
                    first(({ isLoading }) => !isLoading)
                )
            )
            sinon.assert.calledOnce(urlToFile)
            expect(urlToFile.getCalls()[0].args[0]).toMatchObject({
                filePath: 'f',
                position: undefined,
                rawRepoName: 'github.com/r3',
                repoName: 'r3',
                revision: 'v3',
            })
        })

        describe('when the result is inside the current root', () => {
            it('emits the definition URL the user input revision (not commit SHA) of the root', () =>
                expect(
                    lastValueFrom(
                        of({
                            isLoading: false,
                            result: [{ uri: 'git://r3?c3#f' }],
                        }).pipe(
                            getDefinitionURL(
                                { urlToFile, requestGraphQL },
                                {
                                    getWorkspaceRoots: () => of([FIXTURE_WORKSPACE]),
                                },
                                FIXTURE_PARAMS
                            ),
                            first(({ isLoading }) => !isLoading)
                        )
                    )
                ).resolves.toEqual({ isLoading: false, result: { url: '/r3@v3/-/blob/f', multiple: false } }))
        })

        describe('when the result is not inside the current root (different repo and/or commit)', () => {
            it('emits the definition URL with range', () =>
                expect(
                    lastValueFrom(
                        of({
                            isLoading: false,
                            result: [FIXTURE_LOCATION_CLIENT],
                        }).pipe(
                            getDefinitionURL(
                                { urlToFile, requestGraphQL },
                                {
                                    getWorkspaceRoots: () => of([FIXTURE_WORKSPACE]),
                                },
                                FIXTURE_PARAMS
                            ),
                            first(({ isLoading }) => !isLoading)
                        )
                    )
                ).resolves.toEqual({ isLoading: false, result: { url: '/r2@c2/-/blob/f2?L3:3', multiple: false } }))

            it('emits the definition URL without range', () =>
                expect(
                    lastValueFrom(
                        of({
                            isLoading: false,
                            result: [{ ...FIXTURE_LOCATION_CLIENT, range: undefined }],
                        }).pipe(
                            getDefinitionURL(
                                { urlToFile, requestGraphQL },
                                {
                                    getWorkspaceRoots: () => of([FIXTURE_WORKSPACE]),
                                },
                                FIXTURE_PARAMS
                            ),
                            first(({ isLoading }) => !isLoading)
                        )
                    )
                ).resolves.toEqual({ isLoading: false, result: { url: '/r2@c2/-/blob/f2', multiple: false } }))
        })
    })

    it('emits the definition panel URL if there is more than 1 location result', () =>
        expect(
            firstValueFrom(
                of({
                    isLoading: false,
                    result: [FIXTURE_LOCATION_CLIENT, { ...FIXTURE_LOCATION, uri: 'other' }],
                }).pipe(
                    getDefinitionURL(
                        { urlToFile, requestGraphQL },
                        {
                            getWorkspaceRoots: () => of([{ uri: 'git://r?c', inputRevision: 'v' }]),
                        },
                        FIXTURE_PARAMS
                    )
                )
            )
        ).resolves.toEqual({ isLoading: false, result: { url: '/r@v/-/blob/f?L2:2#tab=def', multiple: true } }))
})
