import { storiesOf } from '@storybook/react'
import { RepogroupPage, RepogroupPageProps } from './RepogroupPage'
import React from 'react'
import { python2To3Metadata } from './Python2To3'
import * as GQL from '../../../shared/src/graphql/schema'
import { NEVER } from 'rxjs'
import { NOOP_SETTINGS_CASCADE } from '../../../shared/src/util/searchTestHelpers'
import { ThemePreference } from '../theme'
import { ActionItemComponentProps } from '../../../shared/src/actions/ActionItem'
import { Services } from '../../../shared/src/api/client/services'
import { AuthenticatedUser } from '../auth'
import { SearchPatternType } from '../graphql-operations'
import { WebStory } from '../components/WebStory'
import { subtypeOf } from '../../../shared/src/util/types'
import { action } from '@storybook/addon-actions'
import { cncf } from './cncf'

const { add } = storiesOf('web/RepogroupPage', module).addParameters({
    design: {
        type: 'figma',
        url: 'https://www.figma.com/file/Xc4M24VTQq8itU0Lgb1Wwm/RFC-159-Visual-Design?node-id=66%3A611',
    },
    chromatic: { viewports: [769, 1200] },
})

const EXTENSIONS_CONTROLLER: ActionItemComponentProps['extensionsController'] = {
    executeCommand: () => new Promise(resolve => setTimeout(resolve, 750)),
}

const PLATFORM_CONTEXT: RepogroupPageProps['platformContext'] = {
    forceUpdateTooltip: () => undefined,
    settings: NEVER,
    sourcegraphURL: '',
}

const authUser: AuthenticatedUser = {
    __typename: 'User',
    id: '0',
    email: 'alice@sourcegraph.com',
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
        ] as GQL.IOrg[],
    },
    tags: [],
    viewerCanAdminister: true,
    databaseID: 0,
}

const commonProps = () =>
    subtypeOf<Partial<RepogroupPageProps>>()({
        settingsCascade: {
            ...NOOP_SETTINGS_CASCADE,
            subjects: [],
            final: {
                'search.repositoryGroups': {
                    python: [
                        'github.com/python/test',
                        'github.com/python/test2',
                        'github.com/python/test3',
                        'github.com/python/test4',
                    ],
                },
            },
        },
        onThemePreferenceChange: action('onThemePreferenceChange'),
        parsedSearchQuery: 'r:golang/oauth2 test f:travis',
        patternType: SearchPatternType.literal,
        setPatternType: action('setPatternType'),
        caseSensitive: false,
        copyQueryButton: false,
        extensionsController: { ...EXTENSIONS_CONTROLLER, services: {} as Services },
        platformContext: PLATFORM_CONTEXT,
        keyboardShortcuts: [],
        setCaseSensitivity: action('setCaseSensitivity'),
        versionContext: undefined,
        activation: undefined,
        isSourcegraphDotCom: true,
        setVersionContext: action('setVersionContext'),
        availableVersionContexts: [],
        showSearchContext: false,
        selectedSearchContextSpec: '',
        setSelectedSearchContextSpec: () => {},
        availableSearchContexts: [],
        defaultSearchContextSpec: '',
        authRequired: false,
        showBatchChanges: false,
        authenticatedUser: authUser,
        repogroupMetadata: python2To3Metadata,
        globbing: false,
        enableSmartQuery: false,
        showOnboardingTour: false,
        showQueryBuilder: false,
    })

add('Refactor Python 2 to 3', () => (
    <WebStory>
        {webProps => (
            <RepogroupPage
                {...webProps}
                {...commonProps()}
                themePreference={webProps.isLightTheme ? ThemePreference.Light : ThemePreference.Dark}
            />
        )}
    </WebStory>
))

add('CNCF', () => (
    <WebStory>
        {webProps => (
            <RepogroupPage
                {...webProps}
                {...commonProps()}
                repogroupMetadata={cncf}
                themePreference={webProps.isLightTheme ? ThemePreference.Light : ThemePreference.Dark}
            />
        )}
    </WebStory>
))
