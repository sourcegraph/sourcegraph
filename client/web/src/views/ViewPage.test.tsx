import React from 'react'
import { create, act } from 'react-test-renderer'
import { ViewPage } from './ViewPage'
import * as H from 'history'
import { SearchPatternType } from '../../../shared/src/graphql-operations'
import { noop } from 'lodash'
import { of } from 'rxjs'
import { MarkupKind } from '@sourcegraph/extension-api-classes'

jest.mock('@sourcegraph/react-loading-spinner', () => ({ LoadingSpinner: 'LoadingSpinner' }))
jest.mock('./QueryInputInViewContent', () => ({ QueryInputInViewContent: 'QueryInputInViewContent' }))

const commonProps: Omit<React.ComponentProps<typeof ViewPage>, 'viewID' | 'extraPath' | 'getViewForID'> = {
    settingsCascade: { final: {}, subjects: [] },
    caseSensitive: false,
    patternType: SearchPatternType.literal,
    setCaseSensitivity: () => undefined,
    setPatternType: () => undefined,
    history: H.createMemoryHistory(),
    location: H.createLocation('/'),
    telemetryService: { log: noop, logViewEvent: noop },
    copyQueryButton: false,
    versionContext: undefined,
    selectedSearchContextSpec: 'global',
    globbing: false,
}

describe('ViewPage', () => {
    test('view is loading', () => {
        const renderer = create(
            <ViewPage {...commonProps} viewID="v" extraPath="" getViewForID={() => of(undefined)} />
        )
        act(() => undefined)
        expect(renderer.toJSON()).toMatchSnapshot()
    })

    test('view not found', () => {
        const renderer = create(<ViewPage {...commonProps} viewID="v" extraPath="" getViewForID={() => of(null)} />)
        act(() => undefined)
        expect(renderer.toJSON()).toMatchSnapshot()
    })

    test('renders view', () => {
        const renderer = create(
            <ViewPage
                {...commonProps}
                viewID="v"
                extraPath=""
                getViewForID={() =>
                    of({
                        title: 't',
                        content: [
                            {
                                kind: MarkupKind.Markdown,
                                value: '**a**',
                            },
                            {
                                kind: MarkupKind.PlainText,
                                value: '*b*',
                            },
                            {
                                value: '*c*',
                            },
                            {
                                component: 'QueryInput',
                                props: { implicitQueryPrefix: 'x' },
                            },
                        ],
                    })
                }
            />
        )
        act(() => undefined)
        expect(renderer.toJSON()).toMatchSnapshot()
    })
})
