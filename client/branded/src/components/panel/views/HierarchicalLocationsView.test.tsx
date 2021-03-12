// react-visibility-sensor, used in CodeExcerpt depends on ReactDOM.findDOMNode,
// which is not supported when using react-test-renderer + jest.
// This mock makes it so that <VisibilitySensor /> simply becomes a <div> in the rendered output.
jest.mock('react-visibility-sensor', () => 'VisibilitySensor')

import { Location } from '@sourcegraph/extension-api-types'
import * as H from 'history'
import { noop } from 'lodash'
import React from 'react'
import renderer from 'react-test-renderer'
import { concat, EMPTY, NEVER, of } from 'rxjs'
import * as sinon from 'sinon'
import { Controller } from '../../../../../shared/src/extensions/controller'
import { SettingsCascadeOrError } from '../../../../../shared/src/settings/settings'
import { HierarchicalLocationsView, HierarchicalLocationsViewProps } from './HierarchicalLocationsView'
import { MaybeLoadingResult } from '@sourcegraph/codeintellify'
import { pretendProxySubscribable, pretendRemote } from '../../../../../shared/src/api/util'
import { FlatExtensionHostAPI } from '../../../../../shared/src/api/contract'
import { Contributions, Raw } from '../../../../../shared/src/api/protocol'

jest.mock('mdi-react/SourceRepositoryIcon', () => 'SourceRepositoryIcon')

describe('<HierarchicalLocationsView />', () => {
    const getProps = () => {
        const registerContributions = sinon.spy<FlatExtensionHostAPI['registerContributions']>(() =>
            pretendProxySubscribable(EMPTY).subscribe(noop as any)
        )

        const extensionsController: Pick<Controller, 'extHostAPI'> = {
            extHostAPI: Promise.resolve(
                pretendRemote<FlatExtensionHostAPI>({
                    updateContext: () => Promise.resolve(),
                    registerContributions,
                })
            ),
        }
        const settingsCascade: SettingsCascadeOrError = {
            subjects: null,
            final: null,
        }
        const location: H.Location = {
            hash: '#L36:18&tab=references',
            pathname: '/github.com/sourcegraph/sourcegraph/-/blob/browser/src/libs/phabricator/index.tsx',
            search: '',
            state: {},
        }

        const props: HierarchicalLocationsViewProps = {
            extensionsController,
            settingsCascade,
            location,
            locations: NEVER,
            defaultGroup: 'git://github.com/foo/bar',
            isLightTheme: true,
            fetchHighlightedFileLineRanges: sinon.spy(),
            versionContext: undefined,
        }
        return { props, registerContributions }
    }

    test('shows a spinner before any locations emissions', () => {
        const { props } = getProps()
        expect(renderer.create(<HierarchicalLocationsView {...props} />).toJSON()).toMatchSnapshot()
    })

    test('shows a spinner if locations emits empty and is not complete', () => {
        const { props } = getProps()
        expect(
            renderer
                .create(
                    <HierarchicalLocationsView
                        {...props}
                        locations={concat(of({ isLoading: true, result: [] }), NEVER)}
                    />
                )
                .toJSON()
        ).toMatchSnapshot()
    })

    test("registers a 'Group by file' contribution", () => {
        const { props, registerContributions } = getProps()
        renderer.create(<HierarchicalLocationsView {...props} />)
        expect(registerContributions.called).toBe(true)
        const expected: Raw<Contributions> = {
            actions: [
                {
                    id: 'panel.locations.groupByFile',
                    title: 'Group by file',
                    category: 'Locations (panel)',
                    command: 'updateConfiguration',
                    commandArguments: [
                        ['panel.locations.groupByFile'],
                        // eslint-disable-next-line no-template-curly-in-string
                        '${!config.panel.locations.groupByFile}',
                        null,
                        'json',
                    ],
                    // eslint-disable-next-line no-template-curly-in-string
                    actionItem: {
                        label: '${config.panel.locations.groupByFile && "Ungroup" || "Group"} by file',
                    },
                },
            ],
            menus: {
                'panel/toolbar': [
                    {
                        action: 'panel.locations.groupByFile',
                        when: 'panel.locations.hasResults && panel.activeView.hasLocations',
                    },
                ],
            },
        }
        expect(registerContributions.getCall(0).args[0]).toMatchObject(expected)
    })

    const SAMPLE_LOCATION: Location = {
        uri: 'git://github.com/foo/bar',
        range: {
            start: {
                line: 1,
                character: 0,
            },
            end: {
                line: 1,
                character: 10,
            },
        },
    }

    test('displays a single location when complete', () => {
        const locations = of<MaybeLoadingResult<Location[]>>({ isLoading: false, result: [SAMPLE_LOCATION] })
        const props = {
            ...getProps().props,
            locations,
        }
        expect(renderer.create(<HierarchicalLocationsView {...props} />).toJSON()).toMatchSnapshot()
    })

    test('displays partial locations before complete', () => {
        const props = {
            ...getProps().props,
            locations: concat(of({ isLoading: false, result: [SAMPLE_LOCATION] }), NEVER),
        }
        expect(renderer.create(<HierarchicalLocationsView {...props} />).toJSON()).toMatchSnapshot()
    })

    test('displays multiple locations grouped by file', () => {
        const locations: Location[] = [
            {
                uri: 'git://github.com/foo/bar#file1.txt',
                range: {
                    start: {
                        line: 1,
                        character: 0,
                    },
                    end: {
                        line: 1,
                        character: 10,
                    },
                },
            },
            {
                uri: 'git://github.com/foo/bar#file2.txt',
                range: {
                    start: {
                        line: 2,
                        character: 0,
                    },
                    end: {
                        line: 2,
                        character: 10,
                    },
                },
            },
            {
                uri: 'git://github.com/foo/bar#file1.txt',
                range: {
                    start: {
                        line: 3,
                        character: 0,
                    },
                    end: {
                        line: 3,
                        character: 10,
                    },
                },
            },
            {
                uri: 'git://github.com/foo/bar#file2.txt',
                range: {
                    start: {
                        line: 4,
                        character: 0,
                    },
                    end: {
                        line: 4,
                        character: 10,
                    },
                },
            },
            {
                uri: 'git://github.com/foo/bar#file2.txt',
                range: {
                    start: {
                        line: 5,
                        character: 0,
                    },
                    end: {
                        line: 5,
                        character: 10,
                    },
                },
            },
        ]
        const props: HierarchicalLocationsViewProps = {
            ...getProps().props,
            settingsCascade: {
                subjects: null,
                final: {
                    'panel.locations.groupByFile': true,
                },
            },
            locations: of({ isLoading: false, result: locations }),
        }
        expect(renderer.create(<HierarchicalLocationsView {...props} />).toJSON()).toMatchSnapshot()
    })
})
