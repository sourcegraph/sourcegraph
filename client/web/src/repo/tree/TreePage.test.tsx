import { MockedProvider } from '@apollo/client/testing'
import { cleanup, screen } from '@testing-library/react'
import { EMPTY, NEVER } from 'rxjs'
import sinon from 'sinon'
import { afterEach, describe, expect, it } from 'vitest'

import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { renderWithBrandedContext } from '@sourcegraph/wildcard/src/testing'

import type { AuthenticatedUser } from '../../auth'
import { type RepositoryFields, RepositoryType } from '../../graphql-operations'

import { type Props, TreePage } from './TreePage'

// TreePage has a dependency on the `perforceChangelistMapping` experimental feature
// in order to build an appropriately-worded Commits button.
// The feature needs to be present to avoid errors.
window.context = window.context || {}
window.context.experimentalFeatures = { perforceChangelistMapping: 'disabled' }

describe('TreePage', () => {
    afterEach(cleanup)

    const repoDefaults = (): RepositoryFields => ({
        id: 'repo-id',
        name: 'repo-name',
        sourceType: RepositoryType.GIT_REPOSITORY,
        url: 'http://repo.url.example.com',
        description: 'Awesome for testing',
        viewerCanAdminister: false,
        isFork: false,
        externalURLs: [],
        externalRepository: {
            serviceType: 'REPO SERVICE TYPE',
            serviceID: 'repo-service-id',
        },
        defaultBranch: {
            displayName: 'Default Branch Display Name',
            abbrevName: 'def-branch-abbr',
        },
        metadata: [],
        topics: [],
    })

    const treePagePropsDefaults = (repositoryFields: RepositoryFields): Props => ({
        repo: repositoryFields,
        repoName: 'test-repo',
        filePath: '',
        commitID: 'asdf1234',
        revision: 'asdf1234',
        isSourcegraphDotCom: false,
        settingsCascade: {
            subjects: null,
            final: null,
        },
        platformContext: {
            settings: NEVER,
            updateSettings: () => Promise.reject(new Error('updateSettings not implemented')),
            getGraphQLClient: () => Promise.reject(new Error('getGraphQLClient not implemented')),
            requestGraphQL: () => EMPTY,
            createExtensionHost: () => Promise.reject(new Error('createExtensionHost not implemented')),
            urlToFile: () => '',
            sourcegraphURL: 'https://sourcegraph.com',
            clientApplication: 'sourcegraph',
            telemetryRecorder: noOpTelemetryRecorder,
        },
        telemetryService: NOOP_TELEMETRY_SERVICE,
        codeIntelligenceEnabled: false,
        batchChangesExecutionEnabled: false,
        batchChangesEnabled: false,
        batchChangesWebhookLogsEnabled: false,
        selectedSearchContextSpec: '',
        setBreadcrumb: sinon.spy(),
        useBreadcrumb: sinon.spy(),
        ownEnabled: false,
        authenticatedUser: null,
        context: { externalURL: '' },
    })

    describe('repo page', () => {
        it('displays a page that is not a fork', () => {
            const repo = repoDefaults()
            const props = treePagePropsDefaults(repo)
            const result = renderWithBrandedContext(
                <MockedProvider>
                    <TreePage {...props} />
                </MockedProvider>
            )
            expect(result.queryByTestId('repo-fork-badge')).not.toBeInTheDocument()
            // check for validity that repo header renders
            expect(result.queryByTestId('repo-header')).toBeInTheDocument()

            // confirm that the Commits button exists
            expect(screen.queryByText('Commits')).toBeInTheDocument()
            // and links to commits in the correct revision
            expect(result.queryByRole('link', { name: 'Commits' })).toHaveAttribute(
                'href',
                '/repo-name@asdf1234/-/commits'
            )
        })

        it('displays a Perforce repository with Perforce language in the Commits button', () => {
            // enable the feature that affects how the Commits button renders
            window.context.experimentalFeatures = { perforceChangelistMapping: 'enabled' }
            const repo = repoDefaults()
            repo.sourceType = RepositoryType.PERFORCE_DEPOT
            const props = treePagePropsDefaults(repo)
            const render = renderWithBrandedContext(
                <MockedProvider>
                    <TreePage {...props} />
                </MockedProvider>
            )
            // when `perforceChangelistMapping` is enabled,
            // Perforce depots should display the Commits button using Perforce-centric language.
            expect(render.queryByText('Changelists')).toBeInTheDocument()
            expect(render.queryByText('Commits')).not.toBeInTheDocument()
            // and link to "changelists" instead of to "commits"
            expect(render.queryByRole('link', { name: 'Changelists' })).toHaveAttribute(
                'href',
                '/repo-name@asdf1234/-/changelists'
            )
        })

        it('displays a Perforce repository with Git language in the Commits button', () => {
            // enable the feature that affects how the Commits button renders
            window.context.experimentalFeatures = { perforceChangelistMapping: 'disabled' }
            const repo = repoDefaults()
            repo.sourceType = RepositoryType.PERFORCE_DEPOT
            const props = treePagePropsDefaults(repo)
            const render = renderWithBrandedContext(
                <MockedProvider>
                    <TreePage {...props} />
                </MockedProvider>
            )
            // when `perforceChangelistMapping` is disabled,
            // Perforce depots should display the Commits button using the same langauge as Git repos.
            expect(render.queryByText('Commits')).toBeInTheDocument()
            expect(render.queryByText('Changelists')).not.toBeInTheDocument()
            // and link to "commits" like any other Git repo
            expect(render.queryByRole('link', { name: 'Commits' })).toHaveAttribute(
                'href',
                '/repo-name@asdf1234/-/commits'
            )
        })

        it('displays a page that is a fork', () => {
            const repo = repoDefaults()
            repo.isFork = true
            const props = treePagePropsDefaults(repo)
            const result = renderWithBrandedContext(
                <MockedProvider>
                    <TreePage {...props} />
                </MockedProvider>
            )
            expect(result.queryByTestId('repo-fork-badge')).toHaveTextContent('Fork')
        })

        it('Should display cody CTA', () => {
            const repo = repoDefaults()
            const props = treePagePropsDefaults(repo)
            window.context = window.context || {}
            window.context.codyEnabled = true
            window.context.codyEnabledForCurrentUser = true

            const mockUser = {
                id: 'userID',
                username: 'username',
                emails: [{ email: 'user@me.com', isPrimary: true, verified: true }],
                siteAdmin: true,
            } as AuthenticatedUser

            renderWithBrandedContext(
                <MockedProvider>
                    <TreePage {...{ ...props, isSourcegraphDotCom: true, authenticatedUser: mockUser }} />
                </MockedProvider>
            )

            expect(screen.getByText('Try Cody on this repository')).toBeVisible()
            expect(screen.getByText('Click the Ask Cody button above and to the right of this banner')).toBeVisible()
            expect(
                screen.getByText('Ask Cody a question like “Explain the structure of this repository”')
            ).toBeVisible()
        })
    })
})
