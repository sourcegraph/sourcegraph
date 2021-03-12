import React from 'react'
import renderer from 'react-test-renderer'
import { createMemoryHistory, createLocation } from 'history'
import { noop } from 'lodash'
import { NEVER } from 'rxjs'
import { SearchResultsInfoBar, SearchResultsInfoBarProps } from './SearchResultsInfoBar'
import { AuthenticatedUser } from '../../auth'
import { NOOP_TELEMETRY_SERVICE } from '../../../../shared/src/telemetry/telemetryService'
import { SearchPatternType } from '../../graphql-operations'
import { extensionsController } from '../../../../shared/src/util/searchTestHelpers'

const COMMON_PROPS: Omit<SearchResultsInfoBarProps, 'enableCodeMonitoring'> = {
    extensionsController,
    platformContext: { forceUpdateTooltip: noop, settings: NEVER },
    history: createMemoryHistory(),
    location: createLocation('/search'),
    authenticatedUser: {
        id: 'userID',
        username: 'username',
        email: 'user@me.com',
        siteAdmin: true,
    } as AuthenticatedUser,
    resultsFound: true,
    allExpanded: true,
    onExpandAllResultsToggle: noop,
    onSaveQueryClick: noop,
    stats: <div />,
    telemetryService: NOOP_TELEMETRY_SERVICE,
    patternType: SearchPatternType.literal,
}

describe('SearchResultsInfoBar', () => {
    test('code monitoring feature flag disabled', () => {
        expect(
            renderer
                .create(<SearchResultsInfoBar {...COMMON_PROPS} enableCodeMonitoring={false} query="foo type:diff" />)
                .toJSON()
        ).toMatchSnapshot()
    })

    test('code monitoring feature flag enabled, cannot create monitor from query', () => {
        expect(
            renderer.create(<SearchResultsInfoBar {...COMMON_PROPS} enableCodeMonitoring={true} query="foo" />).toJSON()
        ).toMatchSnapshot()
    })

    test('code monitoring feature flag enabled, can create monitor from query', () => {
        expect(
            renderer
                .create(<SearchResultsInfoBar {...COMMON_PROPS} enableCodeMonitoring={true} query="foo type:diff" />)
                .toJSON()
        ).toMatchSnapshot()
    })
})
