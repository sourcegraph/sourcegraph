import { action } from '@storybook/addon-actions'
import type { Meta, StoryFn } from '@storybook/react'
import { subDays } from 'date-fns'
import { EMPTY, NEVER, type Observable, of } from 'rxjs'

import { subtypeOf } from '@sourcegraph/common'
import type { SearchContextFields } from '@sourcegraph/shared/src/graphql-operations'
import { noOpTelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import {
    mockFetchSearchContexts,
    mockGetUserSearchContextNamespaces,
} from '@sourcegraph/shared/src/testing/searchContexts/testHelpers'
import { NOOP_SETTINGS_CASCADE } from '@sourcegraph/shared/src/testing/searchTestHelpers'

import type { AuthenticatedUser } from '../auth'
import { WebStory } from '../components/WebStory'
import { SearchPatternType } from '../graphql-operations'

import { cncf } from './cncf'
import { CommunitySearchContextPage, type CommunitySearchContextPageProps } from './CommunitySearchContextPage'
import { temporal } from './Temporal'

const config: Meta = {
    title: 'web/CommunitySearchContextPage',
    parameters: {
        design: {
            type: 'figma',
            url: 'https://www.figma.com/file/Xc4M24VTQq8itU0Lgb1Wwm/RFC-159-Visual-Design?node-id=66%3A611',
        },
        chromatic: { viewports: [769, 1200] },
    },
}

export default config

const PLATFORM_CONTEXT: CommunitySearchContextPageProps['platformContext'] = {
    settings: NEVER,
    sourcegraphURL: '',
    requestGraphQL: () => EMPTY,
    telemetryRecorder: noOpTelemetryRecorder,
}

const authUser: AuthenticatedUser = {
    __typename: 'User',
    id: '0',
    username: 'alice',
    avatarURL: null,
    session: { canSignOut: true },
    displayName: null,
    url: '',
    settingsURL: '#',
    siteAdmin: true,
    organizations: {
        nodes: [
            { id: '0', settingsURL: '#', displayName: 'Acme Corp' },
            { id: '1', settingsURL: '#', displayName: 'Beta Inc' },
        ] as AuthenticatedUser['organizations']['nodes'],
    },
    viewerCanAdminister: true,
    hasVerifiedEmail: true,
    completedPostSignup: true,
    databaseID: 0,
    tosAccepted: true,
    emails: [{ email: 'alice@sourcegraph.com', isPrimary: true, verified: true }],
    latestSettings: null,
    permissions: { nodes: [] },
}

const repositories: SearchContextFields['repositories'] = [
    {
        __typename: 'SearchContextRepositoryRevisions',
        repository: {
            __typename: 'Repository',
            name: 'github.com/example/example2',
        },
        revisions: ['main'],
    },
    {
        __typename: 'SearchContextRepositoryRevisions',
        repository: {
            __typename: 'Repository',
            name: 'github.com/example/example1',
        },
        revisions: ['main'],
    },
]

const fetchCommunitySearchContext = (): Observable<SearchContextFields> =>
    of({
        __typename: 'SearchContext',
        id: '1',
        spec: 'public-ctx',
        name: 'public-ctx',
        namespace: null,
        public: true,
        autoDefined: false,
        description: 'Repositories on Sourcegraph',
        query: '',
        repositories,
        updatedAt: subDays(new Date(), 1).toISOString(),
        viewerCanManage: true,
        viewerHasAsDefault: false,
        viewerHasStarred: false,
    })

const commonProps = () =>
    subtypeOf<Partial<CommunitySearchContextPageProps>>()({
        settingsCascade: NOOP_SETTINGS_CASCADE,
        onThemePreferenceChange: action('onThemePreferenceChange'),
        parsedSearchQuery: 'r:golang/oauth2 test f:travis',
        patternType: SearchPatternType.standard,
        setPatternType: action('setPatternType'),
        caseSensitive: false,
        platformContext: PLATFORM_CONTEXT,
        setCaseSensitivity: action('setCaseSensitivity'),
        activation: undefined,
        isSourcegraphDotCom: true,
        searchContextsEnabled: true,
        selectedSearchContextSpec: '',
        setSelectedSearchContextSpec: () => {},
        authRequired: false,
        batchChangesEnabled: false,
        authenticatedUser: authUser,
        communitySearchContextMetadata: temporal,
        fetchSearchContexts: mockFetchSearchContexts,
        getUserSearchContextNamespaces: mockGetUserSearchContextNamespaces,
        fetchSearchContextBySpec: fetchCommunitySearchContext,
    })

export const Temporal: StoryFn = () => (
    <WebStory>{webProps => <CommunitySearchContextPage {...webProps} {...commonProps()} />}</WebStory>
)

export const CNCFStory: StoryFn = () => (
    <WebStory>
        {webProps => (
            <CommunitySearchContextPage {...webProps} {...commonProps()} communitySearchContextMetadata={cncf} />
        )}
    </WebStory>
)

CNCFStory.storyName = 'CNCF'
