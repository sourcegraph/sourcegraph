import { readFile } from 'mz/fs'
import { type Observable, throwError, of, lastValueFrom } from 'rxjs'
import { beforeEach, describe, expect, test } from 'vitest'

import { resetAllMemoizationCaches } from '@sourcegraph/common'
import type { PlatformContext } from '@sourcegraph/shared/src/platform/context'

import type { DiffOrBlobInfo } from '../shared/codeHost'
import { type GraphQLResponseMap, mockRequestGraphQL } from '../shared/testHelpers'

import type { QueryConduitHelper } from './backend'
import { resolveDiffusionFileInfo, resolveRevisionFileInfo, resolveDiffFileInfo } from './fileInfo'

interface ConduitResponseMap {
    [endpoint: string]: (parameters: any) => Observable<any>
}

const DEFAULT_CONDUIT_RESPONSES: ConduitResponseMap = {
    '/api/diffusion.repository.search': () =>
        of({
            data: [
                {
                    fields: {
                        callsign: 'MUX',
                    },
                    attachments: {
                        uris: {
                            uris: [
                                {
                                    fields: {
                                        uri: {
                                            raw: 'https://github.com/gorilla/mux',
                                            normalized: 'https://github.com/gorilla/mux',
                                        },
                                    },
                                },
                            ],
                        },
                    },
                },
            ],
        }),
    '/api/differential.query': () =>
        of({
            0: {
                repositoryPHID: '1',
            },
        }),
    '/api/differential.querydiffs': (parameters: { ids: string[]; revisionIDs: string[] }) =>
        of({
            [parameters.ids[0]]: {
                id: parameters.ids[0],
                revisionID: parameters.revisionIDs[0],
                dateCreated: '1566329300',
                dateModified: '1566329305',
                sourceControlBaseRevision: 'base-revision',
                branch: 'test',
                description: '  - test',
                changes: [
                    {
                        currentPath: 'helpers/add.go',
                    },
                    {
                        currentPath: '.arcconfig',
                    },
                ],
                properties: {
                    'arc.staging': {
                        status: 'pushed',
                        refs: [
                            {
                                ref: `refs/tags/phabricator/base/${parameters.ids[0]}`,
                                type: 'base',
                                commit: `base-${parameters.ids[0]}`,
                                remote: { uri: 'https://github.com/lguychard/testing.git' },
                            },
                            {
                                ref: `refs/tags/phabricator/diff/${parameters.ids[0]}`,
                                type: 'diff',
                                commit: `diff-${parameters.ids[0]}`,
                                remote: { uri: 'https://github.com/lguychard/testing.git' },
                            },
                        ],
                    },
                },
                authorName: 'Loïc Guychard',
                authorEmail: 'loic@sourcegraph.com',
            },
        }),
    '/api/differential.getrawdiff': () => of('diff'),
}

const DEFAULT_GRAPHQL_RESPONSES: GraphQLResponseMap = {
    addPhabricatorRepo: () =>
        of({
            data: {},
            errors: undefined,
        }),
    ResolveRepo: () =>
        of({
            data: {
                repository: null,
            },
            errors: undefined,
        }),
    ResolveStagingRev: () =>
        of({
            data: { resolvePhabricatorDiff: { oid: 'staging-revision' } },
            errors: undefined,
        }),
}

function mockQueryConduit(responseMap?: ConduitResponseMap): QueryConduitHelper<any> {
    return (endpoint, parameters) => {
        const mock = responseMap?.[endpoint] || DEFAULT_CONDUIT_RESPONSES[endpoint]
        if (!mock) {
            return throwError(() => new Error(`No mock for endpoint ${endpoint}`))
        }
        return mock(parameters)
    }
}

type Resolver = (
    codeView: HTMLElement,
    requestGraphQL: PlatformContext['requestGraphQL'],
    queryConduit: QueryConduitHelper<any>,
    windowLocation__testingOnly: Location | URL
) => Observable<DiffOrBlobInfo>

interface Fixture {
    htmlFixture: string
    url: string
    codeViewSelector: string
    graphQLResponseMap?: GraphQLResponseMap
    conduitResponseMap?: ConduitResponseMap
}

const resolveFileInfoFromFixture = async (
    { url, htmlFixture, codeViewSelector, graphQLResponseMap, conduitResponseMap }: Fixture,
    resolver: Resolver
): Promise<DiffOrBlobInfo> => {
    const fixtureContent = await readFile(`${__dirname}/__fixtures__/pages/${htmlFixture}`, 'utf-8')
    document.body.innerHTML = fixtureContent
    const codeView = document.querySelector(codeViewSelector)
    if (!codeView) {
        throw new Error(`Code view matching selector ${codeViewSelector} not found`)
    }
    return lastValueFrom(
        resolver(
            codeView as HTMLElement,
            mockRequestGraphQL({
                ...DEFAULT_GRAPHQL_RESPONSES,
                ...graphQLResponseMap,
            }),
            mockQueryConduit(conduitResponseMap),
            new URL(url)
        )
    )
}

describe('Phabricator file info', () => {
    beforeEach(() => {
        resetAllMemoizationCaches()
    })

    describe('resolveRevisionFileInfo()', () => {
        test('Commit view', async () => {
            expect(
                await resolveFileInfoFromFixture(
                    {
                        htmlFixture: 'commit-view.html',
                        url: 'https://phabricator.sgdev.org/rMUXeab9c4f3d22d907d728aa0f5918934357866249e',
                        codeViewSelector: '.differential-changeset',
                    },
                    resolveRevisionFileInfo
                )
            ).toEqual({
                base: {
                    rawRepoName: 'github.com/gorilla/mux',
                    filePath: 'mux.go',
                    commitID: '50fbc3e7fbfcdb4fb850686588071e5f0bdd4a0a',
                },
                head: {
                    rawRepoName: 'github.com/gorilla/mux',
                    filePath: 'mux.go',
                    commitID: 'eab9c4f3d22d907d728aa0f5918934357866249e',
                },
            })
        })
    })

    describe('resolveDiffusionFileInfo()', () => {
        test('Resolves file info for a Diffusion code view', async () => {
            expect(
                await resolveFileInfoFromFixture(
                    {
                        htmlFixture: 'diffusion.html',
                        url: 'https://phabricator.sgdev.org/source/gorilla/browse/master/mux.go',
                        codeViewSelector: '.diffusion-source',
                    },
                    resolveDiffusionFileInfo
                )
            ).toEqual({
                blob: {
                    commitID: 'e67b3c02c7195c052acff13261f0c9fd1ba53011',
                    filePath: 'mux.go',
                    rawRepoName: 'github.com/gorilla/mux',
                },
            })
        })

        test('Ignores disabled URIs', async () => {
            expect(
                await resolveFileInfoFromFixture(
                    {
                        htmlFixture: 'diffusion.html',
                        url: 'https://phabricator.sgdev.org/source/gorilla/browse/master/mux.go',
                        codeViewSelector: '.diffusion-source',
                        conduitResponseMap: {
                            '/api/diffusion.repository.search': () =>
                                of({
                                    data: [
                                        {
                                            fields: {
                                                callsign: 'MUX',
                                            },
                                            attachments: {
                                                uris: {
                                                    uris: [
                                                        {
                                                            fields: {
                                                                uri: {
                                                                    raw: 'ssh://git@a.b/gorilla/mux',
                                                                    normalized: 'a.b/gorilla/mux',
                                                                    disabled: true,
                                                                },
                                                            },
                                                        },
                                                        {
                                                            fields: {
                                                                uri: {
                                                                    raw: 'ssh://git@c.d/gorilla/mux',
                                                                    normalized: 'c.d/gorilla/mux',
                                                                    disabled: false,
                                                                },
                                                            },
                                                        },
                                                    ],
                                                },
                                            },
                                        },
                                    ],
                                }),
                        },
                    },
                    resolveDiffusionFileInfo
                )
            ).toEqual({
                blob: {
                    commitID: 'e67b3c02c7195c052acff13261f0c9fd1ba53011',
                    filePath: 'mux.go',
                    rawRepoName: 'c.d/gorilla/mux',
                },
            })
        })

        test('Repository hosted on phabricator instance', async () => {
            expect(
                await resolveFileInfoFromFixture(
                    {
                        htmlFixture: 'diffusion.html',
                        url: 'https://phabricator.sgdev.org/source/gorilla/browse/master/mux.go',
                        codeViewSelector: '.diffusion-source',
                        conduitResponseMap: {
                            '/api/diffusion.repository.search': () =>
                                of({
                                    data: [
                                        {
                                            fields: {
                                                callsign: 'MUX',
                                            },
                                            attachments: {
                                                uris: {
                                                    uris: [
                                                        {
                                                            fields: {
                                                                uri: {
                                                                    raw: 'https://phabricator.sgdev.org/gorilla/mux',
                                                                    normalized: 'phabricator.sgdev.org/gorilla/mux',
                                                                    disabled: false,
                                                                },
                                                            },
                                                        },
                                                    ],
                                                },
                                            },
                                        },
                                    ],
                                }),
                        },
                    },
                    resolveDiffusionFileInfo
                )
            ).toEqual({
                blob: {
                    commitID: 'e67b3c02c7195c052acff13261f0c9fd1ba53011',
                    filePath: 'mux.go',
                    rawRepoName: 'phabricator.sgdev.org/gorilla/mux',
                },
            })
        })
    })

    describe('resolveDiffFileInfo()', () => {
        test('Differential revision - no staging repo', async () => {
            expect(
                await resolveFileInfoFromFixture(
                    {
                        htmlFixture: 'differential-revision.html',
                        url: 'https://phabricator.sgdev.org/D7',
                        codeViewSelector: '.differential-changeset',
                        conduitResponseMap: {
                            // Returns diff details without staging details
                            '/api/differential.querydiffs': parameters =>
                                of({
                                    [parameters.ids[0]]: {
                                        id: parameters.ids[0],
                                        revisionID: parameters.revisionIDs[0],
                                        dateCreated: '1566329300',
                                        dateModified: '1566329305',
                                        sourceControlBaseRevision: 'base-revision',
                                        branch: 'test',
                                        description: '  - test',
                                        changes: [],
                                        properties: {},
                                        authorName: 'Loïc Guychard',
                                        authorEmail: 'loic@sourcegraph.com',
                                    },
                                }),
                        },
                    },
                    resolveDiffFileInfo
                )
            ).toEqual({
                base: { rawRepoName: 'github.com/gorilla/mux', filePath: 'helpers/add.go', commitID: 'base-revision' },
                head: {
                    rawRepoName: 'github.com/gorilla/mux',
                    filePath: 'helpers/add.go',
                    commitID: 'staging-revision',
                },
            })
        })
        test('Differential revision - staging repo not synced', async () => {
            expect(
                await resolveFileInfoFromFixture(
                    {
                        htmlFixture: 'differential-revision.html',
                        url: 'https://phabricator.sgdev.org/D7',
                        codeViewSelector: '.differential-changeset',
                    },
                    resolveDiffFileInfo
                )
            ).toEqual({
                base: { rawRepoName: 'github.com/gorilla/mux', filePath: 'helpers/add.go', commitID: 'base-revision' },
                head: {
                    rawRepoName: 'github.com/gorilla/mux',
                    filePath: 'helpers/add.go',
                    commitID: 'staging-revision',
                },
            })
        })
        test('Differential revision - staging repo synced', async () => {
            expect(
                await resolveFileInfoFromFixture(
                    {
                        htmlFixture: 'differential-revision.html',
                        url: 'https://phabricator.sgdev.org/D7',
                        codeViewSelector: '.differential-changeset',
                        graphQLResponseMap: {
                            // Echoes the raw repo name, to represent the fact that the repository
                            // exists on the Sourcegraph instance.
                            ResolveRepo: (variables: any) =>
                                of({
                                    data: {
                                        repository: {
                                            name: variables.rawRepoName,
                                        },
                                    },
                                    errors: undefined,
                                }),
                        },
                    },
                    resolveDiffFileInfo
                )
            ).toEqual({
                base: { rawRepoName: 'github.com/gorilla/mux', filePath: 'helpers/add.go', commitID: 'base-revision' },
                head: { rawRepoName: 'github.com/lguychard/testing', filePath: 'helpers/add.go', commitID: 'diff-13' },
            })
        })
        test('Differential revision - comparing diffs - staging repo not synced', async () => {
            expect(
                await resolveFileInfoFromFixture(
                    {
                        htmlFixture: 'differential-diff-comparison.html',
                        url: 'https://phabricator.sgdev.org/D1?vs=2&id=3&whitespace=ignore-most#toc',
                        codeViewSelector: '.differential-changeset',
                        graphQLResponseMap: {
                            ResolveStagingRev: (variables: any) =>
                                of({
                                    data: {
                                        resolvePhabricatorDiff: {
                                            oid: `staging-revision-${variables.patch as string}`,
                                        },
                                    },
                                    errors: undefined,
                                }),
                        },
                        conduitResponseMap: {
                            '/api/differential.getrawdiff': parameters =>
                                of(`raw-diff-for-diffid-${parameters.diffID as string}`),
                        },
                    },
                    resolveDiffFileInfo
                )
            ).toEqual({
                base: {
                    rawRepoName: 'github.com/gorilla/mux',
                    filePath: '.arcconfig',
                    commitID: 'staging-revision-raw-diff-for-diffid-2',
                },
                head: {
                    rawRepoName: 'github.com/gorilla/mux',
                    filePath: '.arcconfig',
                    commitID: 'staging-revision-raw-diff-for-diffid-3',
                },
            })
        })
        test('Differential revision - comparing diffs - staging repo synced', async () => {
            expect(
                await resolveFileInfoFromFixture(
                    {
                        htmlFixture: 'differential-diff-comparison.html',
                        url: 'https://phabricator.sgdev.org/D1?vs=2&id=3&whitespace=ignore-most#toc',
                        codeViewSelector: '.differential-changeset',
                        graphQLResponseMap: {
                            // Echoes the raw repo name, to represent the fact that the repository
                            // exists on the Sourcegraph instance.
                            ResolveRepo: (variables: any) =>
                                of({
                                    data: {
                                        repository: {
                                            name: variables.rawRepoName,
                                        },
                                    },
                                    errors: undefined,
                                }),
                        },
                    },
                    resolveDiffFileInfo
                )
            ).toEqual({
                base: { rawRepoName: 'github.com/lguychard/testing', filePath: '.arcconfig', commitID: 'diff-2' },
                head: { rawRepoName: 'github.com/lguychard/testing', filePath: '.arcconfig', commitID: 'diff-3' },
            })
        })
    })
})
