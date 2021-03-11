import assert from 'assert'
import { createDriverForTest, Driver } from '../../../shared/src/testing/driver'
import { commonWebGraphQlResults } from './graphQlResults'
import { createWebIntegrationTestContext, WebIntegrationTestContext } from './context'
import { afterEachSaveScreenshotIfFailed } from '../../../shared/src/testing/screenshotReporter'
import { subDays, addDays } from 'date-fns'
import {
    ChangesetCheckState,
    ChangesetReviewState,
    ChangesetCountsOverTimeVariables,
    ChangesetCountsOverTimeResult,
    ExternalChangesetFileDiffsVariables,
    ExternalChangesetFileDiffsResult,
    WebGraphQlOperations,
    ExternalChangesetFileDiffsFields,
    DiffHunkLineType,
    ChangesetSpecType,
    ListBatchChange,
    BatchChangeByNamespaceResult,
    BatchChangeChangesetsVariables,
    BatchChangeChangesetsResult,
} from '../graphql-operations'
import {
    ChangesetSpecOperation,
    ChangesetState,
    ExternalServiceKind,
    SharedGraphQlOperations,
} from '../../../shared/src/graphql-operations'

const batchChangeListNode: ListBatchChange = {
    id: 'batch123',
    url: '/users/alice/batch-changes/test-batch-change',
    name: 'test-batch-change',
    createdAt: subDays(new Date(), 5).toISOString(),
    changesetsStats: { closed: 4, merged: 10, open: 5 },
    closedAt: null,
    description: null,
    namespace: {
        namespaceName: 'alice',
        url: '/users/alice',
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
                    added: 10,
                    changed: 3,
                    deleted: 8,
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

const ChangesetCountsOverTime: (variables: ChangesetCountsOverTimeVariables) => ChangesetCountsOverTimeResult = () => ({
    node: {
        __typename: 'BatchChange',
        changesetCountsOverTime: [
            {
                closed: 12,
                date: subDays(new Date(), 2).toISOString(),
                merged: 10,
                openApproved: 3,
                openChangesRequested: 1,
                openPending: 81,
                total: 130,
                draft: 10,
            },
            {
                closed: 12,
                date: subDays(new Date(), 1).toISOString(),
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
                    createdAt: subDays(new Date(), 5).toISOString(),
                    updatedAt: subDays(new Date(), 5).toISOString(),
                    diffStat: {
                        added: 100,
                        changed: 10,
                        deleted: 23,
                    },
                    error: null,
                    syncerError: null,
                    externalID: '123',
                    state: ChangesetState.OPEN,
                    externalURL: {
                        url: 'http://test.test/123',
                    },
                    id: 'changeset123',
                    labels: [
                        {
                            color: '93ba13',
                            description: null,
                            text: 'Abc label',
                        },
                    ],
                    nextSyncAt: null,
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
                            headRef: 'my-branch',
                        },
                    },
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
        UserArea: () => ({
            user: {
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
                emails: [{ email: 'alice@example.com', verified: true }],
                organizations: { nodes: [] },
                permissionsInfo: null,
                tags: [],
            },
        }),
        BatchChangeByNamespace: () => ({
            batchChange: {
                __typename: 'BatchChange',
                id: 'change123',
                changesetsStats: { closed: 2, deleted: 1, merged: 3, open: 8, total: 19, unpublished: 3, draft: 2 },
                closedAt: null,
                createdAt: subDays(new Date(), 5).toISOString(),
                updatedAt: subDays(new Date(), 5).toISOString(),
                description: '### Very cool batch change',
                initialApplier: {
                    url: '/users/alice',
                    username: 'alice',
                },
                name: 'test-batch-change',
                namespace: {
                    namespaceName: entityType === 'user' ? 'alice' : 'test-org',
                    url: namespaceURL,
                },
                diffStat: { added: 1000, changed: 2000, deleted: 1000 },
                url: `${namespaceURL}/batch-changes/test-batch-change`,
                viewerCanAdminister: true,
                lastAppliedAt: subDays(new Date(), 5).toISOString(),
                lastApplier: {
                    url: '/users/bob',
                    username: 'bob',
                },
                currentSpec: {
                    originalInput: 'name: awesome-batch-change\ndescription: somesttring',
                    supersedingBatchSpec: null,
                },
                ...batchesOverrides,
            },
        }),
        BatchChangesByNamespace: () =>
            entityType === 'user'
                ? {
                      node: {
                          __typename: 'User',
                          batchChanges: {
                              nodes: [batchChangeListNode],
                              pageInfo: {
                                  endCursor: null,
                                  hasNextPage: false,
                              },
                              totalCount: 1,
                          },
                          allBatchChanges: {
                              totalCount: 1,
                          },
                      },
                  }
                : {
                      node: {
                          __typename: 'Org',
                          batchChanges: {
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
                          allBatchChanges: {
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
        AreBatchChangesLicensed: () => ({
            campaigns: true,
            batchChanges: true,
        }),
    }

    describe('Batch changes list', () => {
        it('lists global batch changes', async () => {
            testContext.overrideGraphQL({
                ...commonWebGraphQlResults,
                ...batchChangeLicenseGraphQlResults,
                BatchChanges: () => ({
                    batchChanges: {
                        nodes: [batchChangeListNode],
                        pageInfo: {
                            endCursor: null,
                            hasNextPage: false,
                        },
                        totalCount: 1,
                    },
                    allBatchChanges: {
                        totalCount: 1,
                    },
                }),
            })
            await driver.page.goto(driver.sourcegraphBaseUrl + '/batch-changes')
            await driver.page.waitForSelector('.test-batches-list-page')
            await driver.page.waitForSelector('.test-batches-namespace-link')
            await driver.page.waitForSelector('.test-batches-link')
            assert.strictEqual(
                await driver.page.evaluate(
                    () => document.querySelector<HTMLAnchorElement>('.test-batches-namespace-link')?.href
                ),
                testContext.driver.sourcegraphBaseUrl + '/users/alice/batch-changes'
            )
            assert.strictEqual(
                await driver.page.evaluate(() => document.querySelector<HTMLAnchorElement>('.test-batches-link')?.href),
                testContext.driver.sourcegraphBaseUrl + '/users/alice/batch-changes/test-batch-change'
            )
        })

        it('lists user batch changes', async () => {
            testContext.overrideGraphQL({
                ...commonWebGraphQlResults,
                ...batchChangeLicenseGraphQlResults,
                ...mockCommonGraphQLResponses('user'),
            })
            await driver.page.goto(driver.sourcegraphBaseUrl + '/users/alice/batch-changes')

            await driver.page.waitForSelector('.test-batches-list-page')
            await driver.page.waitForSelector('.test-batches-link')
            assert.strictEqual(
                await driver.page.evaluate(() => document.querySelector<HTMLAnchorElement>('.test-batches-link')?.href),
                testContext.driver.sourcegraphBaseUrl + '/users/alice/batch-changes/test-batch-change'
            )
            assert.strictEqual(await driver.page.$('.test-batches-namespace-link'), null)
        })

        it('lists org batch changes', async () => {
            testContext.overrideGraphQL({
                ...commonWebGraphQlResults,
                ...batchChangeLicenseGraphQlResults,
                ...mockCommonGraphQLResponses('org'),
            })
            await driver.page.goto(driver.sourcegraphBaseUrl + '/batch-changes')

            await driver.page.goto(driver.sourcegraphBaseUrl + '/organizations/test-org/batch-changes')
            await driver.page.waitForSelector('.test-batches-list-page')
            await driver.page.waitForSelector('.test-batches-link')
            assert.strictEqual(
                await driver.page.evaluate(() => document.querySelector<HTMLAnchorElement>('.test-batches-link')?.href),
                testContext.driver.sourcegraphBaseUrl + '/organizations/test-org/batch-changes/test-batch-change'
            )
            assert.strictEqual(await driver.page.$('.test-batches-namespace-link'), null)
        })
    })

    describe('Batch changes details', () => {
        for (const entityType of ['user', 'org'] as const) {
            it(`displays a single batch change for ${entityType}`, async () => {
                testContext.overrideGraphQL({
                    ...commonWebGraphQlResults,
                    ...batchChangeLicenseGraphQlResults,
                    ...mockCommonGraphQLResponses(entityType),
                    BatchChangeChangesets,
                    ChangesetCountsOverTime,
                    ExternalChangesetFileDiffs,
                })
                const namespaceURL = entityType === 'user' ? '/users/alice' : '/organizations/test-org'

                await driver.page.goto(driver.sourcegraphBaseUrl + namespaceURL + '/batch-changes/test-batch-change')
                // View overview page.
                await driver.page.waitForSelector('.test-batch-change-details-page')

                // Expand one changeset.
                await driver.page.click('.test-batches-expand-changeset')
                // Expect one diff to be rendered.
                await driver.page.waitForSelector('.test-file-diff-node')

                // Switch to view burndown chart.
                await driver.page.click('.test-batches-chart-tab')
                await driver.page.waitForSelector('.test-batches-chart')

                // Switch to view spec file.
                await driver.page.click('.test-batches-spec-tab')
                await driver.page.waitForSelector('.test-batches-spec')

                // Go to close page via button.
                await Promise.all([driver.page.waitForNavigation(), driver.page.click('.test-batches-close-btn')])
                assert.strictEqual(
                    await driver.page.evaluate(() => window.location.href),
                    testContext.driver.sourcegraphBaseUrl + namespaceURL + '/batch-changes/test-batch-change/close'
                )
                await driver.page.waitForSelector('.test-batch-change-close-page')
                // Change overrides to make batch change appear closed.
                testContext.overrideGraphQL({
                    ...commonWebGraphQlResults,
                    ...batchChangeLicenseGraphQlResults,
                    ...mockCommonGraphQLResponses(entityType, { closedAt: subDays(new Date(), 1).toISOString() }),
                    BatchChangeChangesets,
                    ChangesetCountsOverTime,
                    ExternalChangesetFileDiffs,
                    DeleteBatchChange: () => ({
                        deleteBatchChange: {
                            alwaysNil: null,
                        },
                    }),
                })

                // Return to details page.
                await Promise.all([driver.page.waitForNavigation(), driver.page.click('.test-batches-close-abort-btn')])
                await driver.page.waitForSelector('.test-batch-change-details-page')
                assert.strictEqual(
                    await driver.page.evaluate(() => window.location.href),
                    testContext.driver.sourcegraphBaseUrl + namespaceURL + '/batch-changes/test-batch-change'
                )

                // Delete the closed batch change.
                await Promise.all([
                    driver.page.waitForNavigation(),
                    driver.acceptNextDialog(),
                    driver.page.click('.test-batches-delete-btn'),
                ])
                assert.strictEqual(
                    await driver.page.evaluate(() => window.location.href),
                    testContext.driver.sourcegraphBaseUrl + namespaceURL + '/batch-changes'
                )

                // Test read tab from location.
                await driver.page.goto(
                    driver.sourcegraphBaseUrl + namespaceURL + '/batch-changes/test-batch-change?tab=chart'
                )
                await driver.page.waitForSelector('.test-batches-chart')
                await driver.page.goto(
                    driver.sourcegraphBaseUrl + namespaceURL + '/batch-changes/test-batch-change?tab=spec'
                )
                await driver.page.waitForSelector('.test-batches-spec')
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
                            createdAt: subDays(new Date(), 2).toISOString(),
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
                                added: 1000,
                                changed: 100,
                                deleted: 182,
                            },
                            expiresAt: addDays(new Date(), 3).toISOString(),
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

                                    added: 5,
                                    modified: 10,
                                    removed: 3,
                                },
                            },
                            viewerBatchChangesCodeHosts: {
                                totalCount: 0,
                                nodes: [],
                            },
                        },
                    }),
                    BatchSpecApplyPreview: () => ({
                        node: {
                            __typename: 'BatchSpec',
                            applyPreview: {
                                nodes: [
                                    {
                                        __typename: 'VisibleChangesetApplyPreview',
                                        operations: [ChangesetSpecOperation.PUSH, ChangesetSpecOperation.PUBLISH],
                                        delta: {
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
                                                    baseRepository: {
                                                        name: 'github.com/sourcegraph/repo',
                                                        url: 'http://test.test/repo',
                                                    },
                                                    published: true,
                                                    body: 'Body',
                                                    commits: [
                                                        {
                                                            subject: 'Commit message',
                                                            body: 'And the more explanatory body.',
                                                            author: {
                                                                avatarURL: null,
                                                                displayName: 'john',
                                                                email: 'john@test.not',
                                                                user: {
                                                                    displayName: 'lejohn',
                                                                    url: '/users/lejohn',
                                                                    username: 'john',
                                                                },
                                                            },
                                                        },
                                                    ],
                                                    diffStat: {
                                                        added: 10,
                                                        changed: 2,
                                                        deleted: 9,
                                                    },
                                                    title: 'Changeset title',
                                                },
                                                expiresAt: addDays(new Date(), 3).toISOString(),
                                                id: 'changesetspec123',
                                                type: ChangesetSpecType.BRANCH,
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
                                diff: mockDiff,
                            },
                        },
                    }),
                    CreateBatchChange: () => ({
                        createBatchChange: {
                            id: 'change123',
                            url: namespaceURL + '/batch-changes/test-batch-change',
                        },
                    }),
                })

                await driver.page.goto(driver.sourcegraphBaseUrl + namespaceURL + '/batch-changes/apply/spec123')
                // View overview page.
                await driver.page.waitForSelector('.test-batch-change-apply-page')

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
                    testContext.driver.sourcegraphBaseUrl + namespaceURL + '/batch-changes/test-batch-change'
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
                    testContext.driver.sourcegraphBaseUrl + namespaceURL + '/batch-changes/test-batch-change'
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
                            totalCount: 1,
                            pageInfo: {
                                endCursor: null,
                                hasNextPage: false,
                            },
                            nodes: [
                                {
                                    externalServiceKind: ExternalServiceKind.GITHUB,
                                    externalServiceURL: 'https://github.com/',
                                    credential: isCreated
                                        ? {
                                              id: '123',
                                              createdAt: new Date().toISOString(),
                                              sshPublicKey: 'ssh-rsa randorandorandorando',
                                          }
                                        : null,
                                    requiresSSH: false,
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
                            createdAt: new Date().toISOString(),
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
            // Wait for list to load.
            await driver.page.waitForSelector('.test-code-host-connection-node')
            // Check no credential is configured.
            assert.strictEqual(await driver.page.$('.test-code-host-connection-node-enabled'), null)
            // Click "Add token".
            await driver.page.click('.test-code-host-connection-node-btn-add')
            // Wait for modal to appear.
            await driver.page.waitForSelector('.test-add-credential-modal')
            // Enter token.
            await driver.page.type('.test-add-credential-modal-input', 'SUPER SECRET')
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
