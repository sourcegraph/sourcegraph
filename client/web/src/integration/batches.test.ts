import assert from 'assert'

import { addDays, subDays } from 'date-fns'
import { afterEach, beforeEach, describe, it } from 'mocha'

import {
    BatchSpecSource,
    ChangesetSpecOperation,
    ChangesetState,
    ExternalServiceKind,
    type SharedGraphQlOperations,
} from '@sourcegraph/shared/src/graphql-operations'
import { accessibilityAudit } from '@sourcegraph/shared/src/testing/accessibility'
import { createDriverForTest, type Driver } from '@sourcegraph/shared/src/testing/driver'
import { afterEachSaveScreenshotIfFailed } from '@sourcegraph/shared/src/testing/screenshotReporter'

import {
    type BatchChangeBatchSpecsResult,
    type BatchChangeBatchSpecsVariables,
    type BatchChangeByNamespaceResult,
    type BatchChangeChangesetsResult,
    type BatchChangeChangesetsVariables,
    BatchChangeState,
    BatchSpecState,
    ChangesetCheckState,
    type ChangesetCountsOverTimeResult,
    type ChangesetCountsOverTimeVariables,
    ChangesetReviewState,
    ChangesetSpecType,
    DiffHunkLineType,
    type ExternalChangesetFileDiffsFields,
    type ExternalChangesetFileDiffsResult,
    type ExternalChangesetFileDiffsVariables,
    type ListBatchChange,
    type WebGraphQlOperations,
} from '../graphql-operations'

import { createWebIntegrationTestContext, type WebIntegrationTestContext } from './context'
import { commonWebGraphQlResults } from './graphQlResults'
import { percySnapshotWithVariants } from './utils'

const now = new Date()

const batchChangeListNode: ListBatchChange & { __typename: 'BatchChange' } = {
    __typename: 'BatchChange',
    id: 'batch123',
    url: '/users/alice/batch-changes/test-batch-change',
    name: 'test-batch-change',
    createdAt: subDays(now, 5).toISOString(),
    changesetsStats: { closed: 4, merged: 10, open: 5 },
    closedAt: null,
    description: null,
    state: BatchChangeState.OPEN,

    namespace: {
        namespaceName: 'alice',
        url: '/users/alice',
    },

    currentSpec: {
        __typename: 'BatchSpec',
        id: 'test-spec',
        state: BatchSpecState.COMPLETED,
        applyURL: '/fake-apply-url',
    },
    batchSpecs: {
        nodes: [
            {
                __typename: 'BatchSpec',
                id: 'test-spec',
                state: BatchSpecState.COMPLETED,
                applyURL: '/fake-apply-url',
            },
        ],
    },
}

const mockDiff: NonNullable<ExternalChangesetFileDiffsFields['diff']> = {
    __typename: 'RepositoryComparison',
    fileDiffs: {
        nodes: [
            {
                internalID: 'intid123',
                oldPath: '/somefile.md',
                newPath: '/somefile.md',
                oldFile: {
                    __typename: 'GitBlob',
                    binary: false,
                    byteSize: 0,
                },
                newFile: {
                    __typename: 'GitBlob',
                    binary: false,
                    byteSize: 0,
                },
                mostRelevantFile: {
                    __typename: 'GitBlob',
                    url: 'http://test.test/fileurl',
                    changelistURL: '',
                },
                hunks: [
                    {
                        section: "@@ -70,33 +81,154 @@ super('awesome', () => {",
                        oldRange: {
                            startLine: 70,
                            lines: 33,
                        },
                        newRange: {
                            startLine: 81,
                            lines: 154,
                        },
                        oldNoNewlineAt: false,
                        highlight: {
                            aborted: false,
                            lines: [
                                {
                                    html: 'some fiel content',
                                    kind: DiffHunkLineType.DELETED,
                                },
                                {
                                    html: 'some file content',
                                    kind: DiffHunkLineType.ADDED,
                                },
                            ],
                        },
                    },
                ],
                stat: {
                    added: 13,
                    deleted: 11,
                },
            },
        ],
        pageInfo: {
            endCursor: null,
            hasNextPage: false,
        },
        totalCount: 1,
    },
    range: {
        base: {
            __typename: 'GitRef',
            target: {
                oid: 'abc123base',
            },
        },
        head: {
            __typename: 'GitRef',
            target: {
                oid: 'abc123head',
            },
        },
    },
}

const mockChangesetSpecFileDiffs: NonNullable<ExternalChangesetFileDiffsFields['diff']> = {
    __typename: 'PreviewRepositoryComparison',
    fileDiffs: mockDiff.fileDiffs,
}

const ChangesetCountsOverTime: (variables: ChangesetCountsOverTimeVariables) => ChangesetCountsOverTimeResult = () => ({
    node: {
        __typename: 'BatchChange',
        changesetCountsOverTime: [
            {
                __typename: 'ChangesetCounts',
                closed: 12,
                date: subDays(now, 2).toISOString(),
                merged: 10,
                openApproved: 3,
                openChangesRequested: 1,
                openPending: 81,
                total: 130,
                draft: 10,
            },
            {
                __typename: 'ChangesetCounts',
                closed: 12,
                date: subDays(now, 1).toISOString(),
                merged: 10,
                openApproved: 23,
                openChangesRequested: 1,
                openPending: 66,
                total: 130,
                draft: 5,
            },
        ],
    },
})

const ExternalChangesetFileDiffs: (
    variables: ExternalChangesetFileDiffsVariables
) => ExternalChangesetFileDiffsResult = () => ({
    node: {
        __typename: 'ExternalChangeset',
        diff: mockDiff,
    },
})

const BatchChangeChangesets: (variables: BatchChangeChangesetsVariables) => BatchChangeChangesetsResult = () => ({
    node: {
        __typename: 'BatchChange',
        changesets: {
            __typename: 'ChangesetConnection',
            totalCount: 1,
            pageInfo: {
                endCursor: null,
                hasNextPage: false,
            },
            nodes: [
                {
                    __typename: 'ExternalChangeset',
                    body: 'body123',
                    checkState: ChangesetCheckState.PASSED,
                    createdAt: subDays(now, 5).toISOString(),
                    updatedAt: subDays(now, 5).toISOString(),
                    diffStat: {
                        __typename: 'DiffStat',
                        added: 100,
                        deleted: 23,
                    },
                    error: null,
                    syncerError: null,
                    externalID: '123',
                    state: ChangesetState.OPEN,
                    externalURL: {
                        url: 'http://test.test/123',
                    },
                    forkNamespace: null,
                    id: 'changeset123',
                    labels: [
                        {
                            __typename: 'ChangesetLabel',
                            color: '93ba13',
                            description: null,
                            text: 'Abc label',
                        },
                    ],
                    nextSyncAt: null,
                    commitVerification: null,
                    repository: {
                        id: 'repo123',
                        name: 'github.com/sourcegraph/repo',
                        url: 'http://test.test/repo',
                    },
                    reviewState: ChangesetReviewState.APPROVED,
                    title: 'The changeset title',
                    currentSpec: {
                        id: 'spec-rand-id-1',
                        type: ChangesetSpecType.BRANCH,
                        description: {
                            __typename: 'GitBranchChangesetDescription',
                            baseRef: 'my-branch',
                            headRef: 'my-branch',
                        },
                        pageInfo: { hasNextPage: false },

                        forkTarget: null,
                    },
                },
            ],
        },
    },
})

const BatchChangeBatchSpecs: (variables: BatchChangeBatchSpecsVariables) => BatchChangeBatchSpecsResult = () => ({
    node: {
        __typename: 'BatchChange',
        batchSpecs: {
            __typename: 'BatchSpecConnection',
            totalCount: 1,
            pageInfo: { endCursor: null, hasNextPage: false },
            nodes: [
                {
                    __typename: 'BatchSpec',
                    id: 'Execution',
                    state: BatchSpecState.COMPLETED,
                    finishedAt: '2022-07-06T23:21:45Z',
                    createdAt: '2022-07-06T23:21:45Z',
                    description: {
                        __typename: 'BatchChangeDescription',
                        name: 'test-batch-change',
                    },
                    source: BatchSpecSource.REMOTE,
                    startedAt: '2022-07-06T23:21:45Z',
                    namespace: {
                        namespaceName: 'alice',
                        url: '/users/alice',
                    },
                    creator: {
                        username: 'alice',
                    },
                    originalInput: 'name: awesome-batch-change\ndescription: somesttring',
                    files: null,
                },
            ],
        },
    },
})

function mockCommonGraphQLResponses(
    entityType: 'user' | 'org',
    batchesOverrides?: Partial<NonNullable<BatchChangeByNamespaceResult['batchChange']>>
): Partial<WebGraphQlOperations & SharedGraphQlOperations> {
    const namespaceURL = entityType === 'user' ? '/users/alice' : '/organizations/test-org'
    return {
        Organization: () => ({
            organization: {
                __typename: 'Org',
                createdAt: '2020-08-07T00:00',
                displayName: 'test-org',
                settingsURL: `${namespaceURL}/settings`,
                id: 'TestOrg',
                name: 'test-org',
                url: namespaceURL,
                viewerCanAdminister: true,
                viewerIsMember: false,
                viewerPendingInvitation: null,
            },
        }),
        UserAreaUserProfile: () => ({
            user: {
                __typename: 'User',
                id: 'user123',
                username: 'alice',
                displayName: 'alice',
                url: namespaceURL,
                settingsURL: `${namespaceURL}/settings`,
                avatarURL: '',
                viewerCanAdminister: true,
                builtinAuth: true,
                createdAt: '2020-04-10T21:11:42Z',
                roles: {
                    __typename: 'RoleConnection',
                    nodes: [],
                },
            },
        }),
        UserSettingsAreaUserProfile: () => ({
            node: {
                __typename: 'User',
                id: 'user123',
                username: 'alice',
                displayName: 'alice',
                url: namespaceURL,
                settingsURL: `${namespaceURL}/settings`,
                avatarURL: '',
                viewerCanAdminister: true,
                viewerCanChangeUsername: true,
                siteAdmin: true,
                builtinAuth: true,
                createdAt: '2020-04-10T21:11:42Z',
                emails: [{ email: 'alice@example.com', verified: true, isPrimary: true }],
                organizations: { nodes: [] },
                permissionsInfo: null,
                scimControlled: false,
                roles: {
                    __typename: 'RoleConnection',
                    nodes: [],
                },
            },
        }),
        BatchChangeByNamespace: () => ({
            batchChange: {
                __typename: 'BatchChange',
                id: 'change123',
                changesetsStats: {
                    __typename: 'ChangesetsStats',
                    closed: 2,
                    deleted: 1,
                    merged: 3,
                    open: 8,
                    total: 37,
                    archived: 18,
                    unpublished: 3,
                    draft: 2,
                    isCompleted: false,
                    percentComplete: 27,
                    failed: 0,
                    scheduled: 0,
                    retrying: 0,
                    processing: 0,
                },
                state: BatchChangeState.OPEN,
                closedAt: null,
                createdAt: subDays(now, 5).toISOString(),
                updatedAt: subDays(now, 5).toISOString(),
                // To fix accessibility rule: "heading-order"
                description: '## Very cool batch change',
                creator: {
                    url: '/users/alice',
                    username: 'alice',
                },
                name: 'test-batch-change',
                namespace:
                    entityType === 'user'
                        ? {
                              __typename: 'User',
                              id: '1234',
                              displayName: null,
                              namespaceName: 'alice',
                              url: namespaceURL,
                              username: 'alice',
                          }
                        : {
                              __typename: 'Org',
                              id: '1234',
                              displayName: null,
                              namespaceName: 'test-org',
                              url: namespaceURL,
                              name: 'test-org',
                          },
                diffStat: { added: 1000, changed: 2000, deleted: 1000, __typename: 'DiffStat' },
                url: `${namespaceURL}/batch-changes/test-batch-change`,
                viewerCanAdminister: true,
                lastAppliedAt: subDays(now, 5).toISOString(),
                lastApplier: {
                    url: '/users/bob',
                    username: 'bob',
                },
                currentSpec: {
                    id: 'specID1',
                    originalInput: 'name: awesome-batch-change\ndescription: somesttring',
                    supersedingBatchSpec: null,
                    source: BatchSpecSource.REMOTE,
                    codeHostsWithoutWebhooks: {
                        nodes: [],
                        pageInfo: { hasNextPage: false },
                        totalCount: 0,
                    },
                    viewerBatchChangesCodeHosts: {
                        __typename: 'BatchChangesCodeHostConnection',
                        nodes: [],
                        totalCount: 0,
                    },
                    files: null,
                    description: {
                        __typename: 'BatchChangeDescription',
                        name: 'awesome batch spec',
                    },
                },
                batchSpecs: {
                    nodes: [{ state: BatchSpecState.COMPLETED }],
                    pageInfo: { hasNextPage: false },
                },
                bulkOperations: { __typename: 'BulkOperationConnection', totalCount: 0 },
                activeBulkOperations: { __typename: 'BulkOperationConnection', totalCount: 0, nodes: [] },
                ...batchesOverrides,
            },
        }),
        BatchChangesByNamespace: () =>
            entityType === 'user'
                ? {
                      node: {
                          __typename: 'User',
                          batchChanges: {
                              __typename: 'BatchChangeConnection',
                              nodes: [batchChangeListNode],
                              pageInfo: {
                                  endCursor: null,
                                  hasNextPage: false,
                              },
                              totalCount: 1,
                          },
                      },
                  }
                : {
                      node: {
                          __typename: 'Org',
                          batchChanges: {
                              __typename: 'BatchChangeConnection',
                              nodes: [
                                  {
                                      ...batchChangeListNode,
                                      url: '/organizations/test-org/batch-changes/test-batch-change',
                                      namespace: {
                                          namespaceName: 'test-org',
                                          url: '/organizations/test-org',
                                      },
                                  },
                              ],
                              pageInfo: {
                                  endCursor: null,
                                  hasNextPage: false,
                              },
                              totalCount: 1,
                          },
                      },
                  },
    }
}

describe('Batches', () => {
    let driver: Driver
    before(async () => {
        driver = await createDriverForTest()
    })
    after(() => driver?.close())
    let testContext: WebIntegrationTestContext
    beforeEach(async function () {
        testContext = await createWebIntegrationTestContext({
            driver,
            currentTest: this.currentTest!,
            directory: __dirname,
        })
    })
    afterEachSaveScreenshotIfFailed(() => driver.page)
    afterEach(() => testContext?.dispose())

    const batchChangeLicenseGraphQlResults = {
        GetLicenseAndUsageInfo: () => ({
            campaigns: true,
            batchChanges: true,
            allBatchChanges: {
                totalCount: 1,
            },
            maxUnlicensedChangesets: 10,
        }),
    }
    const batchChangesListResults = {
        BatchChanges: () => ({
            batchChanges: {
                __typename: 'BatchChangeConnection' as const,
                nodes: [batchChangeListNode],
                pageInfo: {
                    endCursor: null,
                    hasNextPage: false,
                },
                totalCount: 1,
            },
        }),
    }

    describe('Batch changes getting started', () => {
        it('displays batch changes - getting started section', async () => {
            // Mock Videos on getting started page
            const videoDomains = ['https://storage.googleapis.com', 'https://www.youtube-nocookie.com']
            for (const domain of videoDomains) {
                testContext.server.host(domain, () => {
                    testContext.server.get('/*path').intercept((request, response) => {
                        response.sendStatus(200)
                    })
                })
            }
            testContext.overrideGraphQL({
                ...commonWebGraphQlResults,
                ...batchChangeLicenseGraphQlResults,
                ...batchChangesListResults,
            })
            await driver.page.goto(driver.sourcegraphBaseUrl + '/batch-changes')
            await driver.page.waitForSelector('.test-batches-list-page')
            await driver.page.click('[data-testid="test-getting-started-btn"]')
            await driver.page.waitForSelector('[data-testid="test-getting-started"]')
            await percySnapshotWithVariants(driver.page, 'Batch changes getting started page')
            await accessibilityAudit(driver.page)
        })
    })

    describe('Batch changes list', () => {
        it('lists global batch changes', async () => {
            testContext.overrideGraphQL({
                ...commonWebGraphQlResults,
                ...batchChangeLicenseGraphQlResults,
                ...batchChangesListResults,
            })
            await driver.page.goto(driver.sourcegraphBaseUrl + '/batch-changes')
            await driver.page.waitForSelector('.test-batches-list-page')
            await driver.page.waitForSelector('.test-batches-namespace-link')
            await driver.page.waitForSelector('.test-batches-link')
            assert.strictEqual(
                await driver.page.evaluate(
                    () => document.querySelector<HTMLAnchorElement>('.test-batches-namespace-link')?.href
                ),
                driver.sourcegraphBaseUrl + '/users/alice/batch-changes'
            )
            assert.strictEqual(
                await driver.page.evaluate(() => document.querySelector<HTMLAnchorElement>('.test-batches-link')?.href),
                driver.sourcegraphBaseUrl + '/users/alice/batch-changes/test-batch-change'
            )

            await percySnapshotWithVariants(driver.page, 'Batch Changes List')
            // TODO: Disabled, we need to audit SSBC things on this list before it can pass.
            // await accessibilityAudit(driver.page)
        })

        it('lists user batch changes', async () => {
            testContext.overrideGraphQL({
                ...commonWebGraphQlResults,
                ...batchChangeLicenseGraphQlResults,
                ...batchChangesListResults,
                ...mockCommonGraphQLResponses('user'),
            })
            await driver.page.goto(driver.sourcegraphBaseUrl + '/users/alice/batch-changes')

            await driver.page.waitForSelector('.test-batches-list-page')
            await driver.page.waitForSelector('.test-batches-link')
            assert.strictEqual(
                await driver.page.evaluate(() => document.querySelector<HTMLAnchorElement>('.test-batches-link')?.href),
                driver.sourcegraphBaseUrl + '/users/alice/batch-changes/test-batch-change'
            )
            assert.strictEqual(await driver.page.$('.test-batches-namespace-link'), null)
        })

        it('lists org batch changes', async () => {
            testContext.overrideGraphQL({
                ...commonWebGraphQlResults,
                ...batchChangeLicenseGraphQlResults,
                ...batchChangesListResults,
                ...mockCommonGraphQLResponses('org'),
            })
            await driver.page.goto(driver.sourcegraphBaseUrl + '/batch-changes')

            await driver.page.goto(driver.sourcegraphBaseUrl + '/organizations/test-org/batch-changes')
            await driver.page.waitForSelector('.test-batches-list-page')
            await driver.page.waitForSelector('.test-batches-link')
            assert.strictEqual(
                await driver.page.evaluate(() => document.querySelector<HTMLAnchorElement>('.test-batches-link')?.href),
                driver.sourcegraphBaseUrl + '/organizations/test-org/batch-changes/test-batch-change'
            )
            assert.strictEqual(await driver.page.$('.test-batches-namespace-link'), null)
        })
    })

    describe('Create batch changes', () => {
        // TODO: SSBC has to go through accessibility audits before this can pass.
        it.skip('is styled correctly', async () => {
            await driver.page.goto(driver.sourcegraphBaseUrl + '/batch-changes/create')
            await driver.page.waitForSelector('[data-testid="batch-spec-yaml-file"]')
            await percySnapshotWithVariants(driver.page, 'Create batch change')
            await accessibilityAudit(driver.page)
        })
    })

    // Possibly flaky test. Removing .skip temporarily in order to figure
    // out the root cause of this as we're unable to reproduce locally.
    // See https://github.com/sourcegraph/sourcegraph/issues/37233
    describe('Batch changes details', () => {
        for (const entityType of ['user', 'org'] as const) {
            it(`displays a single batch change for ${entityType}`, async () => {
                testContext.overrideGraphQL({
                    ...commonWebGraphQlResults,
                    ...batchChangeLicenseGraphQlResults,
                    ...batchChangesListResults,
                    ...mockCommonGraphQLResponses(entityType),
                    BatchChangeChangesets,
                    ChangesetCountsOverTime,
                    ExternalChangesetFileDiffs,
                    BatchChangeBatchSpecs,
                })
                const namespaceURL = entityType === 'user' ? '/users/alice' : '/organizations/test-org'

                await driver.page.goto(driver.sourcegraphBaseUrl + namespaceURL + '/batch-changes/test-batch-change', {
                    // Wait for page to be fully loaded.
                    waitUntil: 'networkidle0',
                })
                // View overview page.
                await driver.page.waitForSelector('.test-batch-change-details-page', { visible: true })
                await percySnapshotWithVariants(driver.page, `Batch change details page ${entityType}`)
                await accessibilityAudit(driver.page)

                // we wait for the changesets to be loaded in the browser before proceeding
                await driver.page.waitForSelector('.test-batches-expand-changeset', { visible: true })

                await driver.page.click('.test-batches-expand-changeset')
                // Expect one diff to be rendered.
                await driver.page.waitForSelector('.test-file-diff-node', { visible: true })

                // Switch to view burndown chart.
                await driver.page.click('[data-testid="wildcard-tab-list"] [data-testid="wildcard-tab"]:nth-child(2)')
                await driver.page.waitForSelector('.test-batches-chart', { visible: true })

                // Switch to view executions.
                await driver.page.click('[data-testid="wildcard-tab-list"] [data-testid="wildcard-tab"]:nth-child(3)')
                await driver.page.waitForSelector('.test-batches-executions', { visible: true })

                // Go to close page via button.
                await Promise.all([driver.page.waitForNavigation(), driver.page.click('.test-batches-close-btn')])
                assert.strictEqual(
                    await driver.page.evaluate(() => window.location.href),
                    driver.sourcegraphBaseUrl + namespaceURL + '/batch-changes/test-batch-change/close'
                )
                await driver.page.waitForSelector('.test-batch-change-close-page', { visible: true })
                // Change overrides to make batch change appear closed.
                testContext.overrideGraphQL({
                    ...commonWebGraphQlResults,
                    ...batchChangeLicenseGraphQlResults,
                    ...mockCommonGraphQLResponses(entityType, { closedAt: subDays(now, 1).toISOString() }),
                    BatchChangeChangesets,
                    ChangesetCountsOverTime,
                    ExternalChangesetFileDiffs,
                    BatchChangeBatchSpecs,
                    DeleteBatchChange: () => ({
                        deleteBatchChange: {
                            alwaysNil: null,
                        },
                    }),
                })

                // Return to details page.
                await Promise.all([driver.page.waitForNavigation(), driver.page.click('.test-batches-close-abort-btn')])
                await driver.page.waitForSelector('.test-batch-change-details-page', { visible: true })
                assert.strictEqual(
                    await driver.page.evaluate(() => window.location.href),
                    // We now have 1 in the cache, so we'll have a starting number visible that gets set in the URL.
                    driver.sourcegraphBaseUrl + namespaceURL + '/batch-changes/test-batch-change?visible=1'
                )

                // Delete the closed batch change.
                await Promise.all([
                    driver.page.waitForNavigation(),
                    driver.acceptNextDialog(),
                    driver.page.click('.test-batches-delete-btn'),
                ])
                assert.strictEqual(
                    await driver.page.evaluate(() => window.location.href),
                    driver.sourcegraphBaseUrl + namespaceURL + '/batch-changes'
                )

                // Test read tab from location.
                await driver.page.goto(
                    driver.sourcegraphBaseUrl + namespaceURL + '/batch-changes/test-batch-change?tab=chart',
                    {
                        // Wait for page to be fully loaded.
                        waitUntil: 'networkidle0',
                    }
                )
                await driver.page.waitForSelector('.test-batches-chart', { visible: true })
                await driver.page.goto(
                    driver.sourcegraphBaseUrl + namespaceURL + '/batch-changes/test-batch-change/executions',
                    {
                        // Wait for page to be fully loaded.
                        waitUntil: 'networkidle0',
                    }
                )
                await driver.page.waitForSelector('.test-batches-executions', { visible: true })
            })
        }
    })

    describe('Batch spec preview', () => {
        for (const entityType of ['user', 'org'] as const) {
            it(`displays a preview of a batch spec in ${entityType} namespace`, async () => {
                const namespaceURL = entityType === 'user' ? '/users/alice' : '/organizations/test-org'
                testContext.overrideGraphQL({
                    ...commonWebGraphQlResults,
                    ...mockCommonGraphQLResponses(entityType),
                    BatchChangeChangesets,
                    BatchSpecByID: () => ({
                        node: {
                            __typename: 'BatchSpec',
                            id: 'spec123',
                            appliesToBatchChange: null,
                            createdAt: subDays(now, 2).toISOString(),
                            creator: {
                                username: 'alice',
                                url: '/users/alice',
                                avatarURL: null,
                            },
                            description: {
                                name: 'test-batch-change',
                                description: '### Very great batch change',
                            },
                            diffStat: {
                                __typename: 'DiffStat',
                                added: 1000,
                                deleted: 182,
                            },
                            expiresAt: addDays(now, 3).toISOString(),
                            namespace:
                                entityType === 'user'
                                    ? {
                                          namespaceName: 'alice',
                                          url: '/users/alice',
                                      }
                                    : {
                                          namespaceName: 'test-org',
                                          url: '/organizations/test-org',
                                      },
                            supersedingBatchSpec: null,
                            viewerCanAdminister: true,
                            originalInput: 'name: awesome-batch-change\ndescription: somestring',
                            applyPreview: {
                                __typename: 'ChangesetApplyPreviewConnection',
                                stats: {
                                    archive: 10,
                                },
                                totalCount: 10,
                            },
                            viewerBatchChangesCodeHosts: {
                                __typename: 'BatchChangesCodeHostConnection',
                                totalCount: 0,
                                nodes: [],
                            },
                        },
                    }),
                    BatchSpecApplyPreview: () => ({
                        node: {
                            __typename: 'BatchSpec',
                            applyPreview: {
                                __typename: 'ChangesetApplyPreviewConnection',
                                nodes: [
                                    {
                                        __typename: 'VisibleChangesetApplyPreview',
                                        operations: [ChangesetSpecOperation.PUSH, ChangesetSpecOperation.PUBLISH],
                                        delta: {
                                            __typename: 'ChangesetSpecDelta',
                                            titleChanged: false,
                                            baseRefChanged: false,
                                            diffChanged: false,
                                            bodyChanged: false,
                                            authorEmailChanged: false,
                                            authorNameChanged: false,
                                            commitMessageChanged: false,
                                        },
                                        targets: {
                                            __typename: 'VisibleApplyPreviewTargetsAttach',
                                            changesetSpec: {
                                                __typename: 'VisibleChangesetSpec',
                                                description: {
                                                    __typename: 'GitBranchChangesetDescription',
                                                    baseRef: 'main',
                                                    headRef: 'head-ref',
                                                    fork: false,
                                                    baseRepository: {
                                                        name: 'github.com/sourcegraph/repo',
                                                        url: 'http://test.test/repo',
                                                    },
                                                    published: true,
                                                    body: 'Body',
                                                    commits: [
                                                        {
                                                            __typename: 'GitCommitDescription',
                                                            subject: 'Commit message',
                                                            body: 'And the more explanatory body.',
                                                            author: {
                                                                __typename: 'Person',
                                                                avatarURL: null,
                                                                displayName: 'john',
                                                                email: 'john@test.not',
                                                                user: {
                                                                    __typename: 'User',
                                                                    displayName: 'lejohn',
                                                                    url: '/users/lejohn',
                                                                    username: 'john',
                                                                },
                                                            },
                                                        },
                                                    ],
                                                    diffStat: {
                                                        __typename: 'DiffStat',
                                                        added: 10,
                                                        deleted: 9,
                                                    },
                                                    title: 'Changeset title',
                                                },
                                                expiresAt: addDays(now, 3).toISOString(),
                                                id: 'changesetspec123',
                                                type: ChangesetSpecType.BRANCH,
                                                forkTarget: null,
                                            },
                                        },
                                    },
                                ],
                                pageInfo: {
                                    endCursor: null,
                                    hasNextPage: false,
                                },
                                totalCount: 1,
                            },
                        },
                    }),
                    ChangesetSpecFileDiffs: () => ({
                        node: {
                            __typename: 'VisibleChangesetSpec',
                            description: {
                                __typename: 'GitBranchChangesetDescription',
                                diff: mockChangesetSpecFileDiffs,
                            },
                        },
                    }),
                    CreateBatchChange: () => ({
                        createBatchChange: {
                            __typename: 'BatchChange',
                            id: 'change123',
                            url: namespaceURL + '/batch-changes/test-batch-change',
                        },
                    }),
                    QueryApplyPreviewStats: () => ({
                        node: {
                            __typename: 'BatchSpec',
                            id: 'spec123',

                            applyPreview: {
                                stats: {
                                    close: 10,
                                    detach: 10,
                                    import: 10,
                                    publish: 10,
                                    publishDraft: 10,
                                    push: 10,
                                    reopen: 10,
                                    undraft: 10,
                                    update: 10,
                                    reattach: 10,
                                    archive: 18,
                                    added: 5,
                                    modified: 10,
                                    removed: 3,
                                },
                            },
                        },
                    }),
                })

                await driver.page.goto(driver.sourcegraphBaseUrl + namespaceURL + '/batch-changes/apply/spec123')
                // View overview page.
                await driver.page.waitForSelector('.test-batch-change-apply-page')
                await percySnapshotWithVariants(driver.page, `Batch change preview page ${entityType}`)
                await accessibilityAudit(driver.page)

                // Expand one changeset.
                await driver.page.click('.test-batches-expand-preview')
                // Expect one diff to be rendered.
                await driver.page.waitForSelector('.test-file-diff-node')

                // Apply batch change.
                await Promise.all([
                    driver.page.waitForNavigation(),
                    driver.acceptNextDialog(),
                    driver.page.click('.test-batches-confirm-apply-btn'),
                ])
                // Expect to be back at batch change overview page.
                assert.strictEqual(
                    await driver.page.evaluate(() => window.location.href),
                    driver.sourcegraphBaseUrl +
                        namespaceURL +
                        '/batch-changes/test-batch-change?archivedCount=10&archivedBy=spec123'
                )
            })
        }
    })

    describe('Batch change close preview', () => {
        for (const entityType of ['user', 'org'] as const) {
            it(`displays a preview for closing a batch change in ${entityType} namespace`, async () => {
                testContext.overrideGraphQL({
                    ...commonWebGraphQlResults,
                    ...mockCommonGraphQLResponses(entityType),
                    BatchChangeChangesets,
                    ExternalChangesetFileDiffs,
                    CloseBatchChange: () => ({
                        closeBatchChange: {
                            id: 'batch123',
                        },
                    }),
                })
                const namespaceURL = entityType === 'user' ? '/users/alice' : '/organizations/test-org'

                await driver.page.goto(
                    driver.sourcegraphBaseUrl + namespaceURL + '/batch-changes/test-batch-change/close'
                )
                // View overview page.
                await driver.page.waitForSelector('.test-batch-change-close-page')

                // Check close changesets box.
                assert.strictEqual(await driver.page.$('.test-batches-close-willclose-header'), null)
                await driver.page.click('.test-batches-close-changesets-checkbox')
                await driver.page.waitForSelector('.test-batches-close-willclose-header')

                // Expand one changeset.
                await driver.page.click('.test-batches-expand-changeset')
                // Expect one diff to be rendered.
                await driver.page.waitForSelector('.test-file-diff-node')

                // Close batch change.
                await Promise.all([
                    driver.page.waitForNavigation(),
                    driver.page.click('.test-batches-confirm-close-btn'),
                ])
                // Expect to be back at the batch change overview page.
                assert.strictEqual(
                    await driver.page.evaluate(() => window.location.href),
                    driver.sourcegraphBaseUrl + namespaceURL + '/batch-changes/test-batch-change'
                )
            })
        }
    })

    describe('Code host token', () => {
        it('allows to add a token for a code host', async () => {
            let isCreated = false
            testContext.overrideGraphQL({
                ...commonWebGraphQlResults,
                ...mockCommonGraphQLResponses('user'),
                UserBatchChangesCodeHosts: () => ({
                    node: {
                        __typename: 'User',
                        batchChangesCodeHosts: {
                            __typename: 'BatchChangesCodeHostConnection',
                            totalCount: 1,
                            pageInfo: {
                                endCursor: null,
                                hasNextPage: false,
                                __typename: 'PageInfo',
                            },
                            nodes: [
                                {
                                    __typename: 'BatchChangesCodeHost',
                                    externalServiceKind: ExternalServiceKind.GITHUB,
                                    externalServiceURL: 'https://github.com/',
                                    credential: isCreated
                                        ? {
                                              id: '123',
                                              isSiteCredential: false,
                                              sshPublicKey: 'ssh-rsa randorandorandorando',
                                          }
                                        : null,
                                    requiresSSH: false,
                                    requiresUsername: false,
                                    supportsCommitSigning: false,
                                    commitSigningConfiguration: null,
                                },
                            ],
                        },
                    },
                }),
                CreateBatchChangesCredential: () => {
                    isCreated = true
                    return {
                        createBatchChangesCredential: {
                            id: '123',
                            isSiteCredential: false,
                            sshPublicKey: 'ssh-rsa randorandorandorando',
                        },
                    }
                },
                DeleteBatchChangesCredential: () => {
                    isCreated = false
                    return {
                        deleteBatchChangesCredential: {
                            alwaysNil: null,
                        },
                    }
                },
            })

            await driver.page.goto(driver.sourcegraphBaseUrl + '/users/alice/settings/batch-changes')
            // View settings page.
            await driver.page.waitForSelector('.test-batches-settings-page')
            await percySnapshotWithVariants(driver.page, 'User batch changes settings page')
            await accessibilityAudit(driver.page)
            // Wait for list to load.
            await driver.page.waitForSelector('.test-code-host-connection-node')
            // Check no credential is configured.
            assert.strictEqual(await driver.page.$('.test-code-host-connection-node-enabled'), null)
            // Click "Add token".
            await driver.page.click('.test-code-host-connection-node-btn-add')
            // Wait for modal to appear.
            await driver.page.waitForSelector('.test-add-credential-modal')
            // Enter token.
            await driver.page.type('[data-testid="test-add-credential-modal-input"]', 'SUPER SECRET')
            // Click add.
            await driver.page.click('.test-add-credential-modal-submit')
            // Await list reload and expect to be enabled.
            await driver.page.waitForSelector('.test-code-host-connection-node-enabled')
            // No modal open.
            assert.strictEqual(await driver.page.$('.test-add-credential-modal'), null)
            // Click "Remove" to remove the token.
            await driver.page.click('.test-code-host-connection-node-btn-remove')
            // Wait for modal to appear.
            await driver.page.waitForSelector('.test-remove-credential-modal')
            // Click confirmation.
            await driver.page.click('.test-remove-credential-modal-submit')
            // Await list reload and expect to be disabled again.
            await driver.page.waitForSelector('.test-code-host-connection-node-disabled')
            // No modal open.
            assert.strictEqual(await driver.page.$('.test-remove-credential-modal'), null)
        })
    })
})
