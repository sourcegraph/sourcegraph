import { EMPTY, of, Subject } from 'rxjs'
import sinon from 'sinon'
import { describe, expect, test } from 'vitest'

import { getGraphQLClient as getGraphQLClientBase, type SuccessGraphQLResult } from '@sourcegraph/http-client'

import { cache } from '../../backend/apolloCache'
import type { PlatformContext } from '../../platform/context'
import type { SettingsCascade } from '../../settings/settings'
import type { FlatExtensionHostAPI } from '../contract'
import { pretendRemote } from '../util'

import { initMainThreadAPI } from './mainthread-api'
import type { SettingsEdit } from './services/settings'

describe('MainThreadAPI', () => {
    // TODO(tj): commands, notifications
    const getGraphQLClient = () => getGraphQLClientBase({ cache })

    describe('graphQL', () => {
        test('PlatformContext#requestGraphQL is called with the correct arguments', async () => {
            const requestGraphQL = sinon.spy(_options => of({ data: null, errors: [] }))

            const platformContext: Pick<
                PlatformContext,
                'updateSettings' | 'settings' | 'getGraphQLClient' | 'requestGraphQL' | 'clientApplication'
            > = {
                settings: EMPTY,
                getGraphQLClient,
                updateSettings: () => Promise.resolve(),
                requestGraphQL,
                clientApplication: 'other',
            }

            const { api } = initMainThreadAPI(pretendRemote({}), platformContext)

            const gqlRequestOptions = {
                request: 'search',
                variables: {
                    query: 'test',
                },
                mightContainPrivateInfo: true,
            }

            await api.requestGraphQL(gqlRequestOptions.request, gqlRequestOptions.variables)

            sinon.assert.calledOnceWithExactly(requestGraphQL, gqlRequestOptions)
        })

        test('extension host receives the value returned by PlatformContext#requestGraphQL', async () => {
            const graphQLResult: SuccessGraphQLResult<any> = {
                data: { search: { results: { results: [] } } },
                errors: undefined,
            }
            const requestGraphQL = sinon.spy(_options => of(graphQLResult))

            const platformContext: Pick<
                PlatformContext,
                'updateSettings' | 'settings' | 'getGraphQLClient' | 'requestGraphQL' | 'clientApplication'
            > = {
                settings: EMPTY,
                getGraphQLClient,
                updateSettings: () => Promise.resolve(),
                requestGraphQL,
                clientApplication: 'other',
            }

            const { api } = initMainThreadAPI(pretendRemote({}), platformContext)

            const result = await api.requestGraphQL('search', {})

            expect(result).toStrictEqual(graphQLResult)
        })
    })

    describe('configuration', () => {
        test('changeConfiguration goes to platform with the last settings subject', async () => {
            let calledWith: Parameters<PlatformContext['updateSettings']> | undefined
            const updateSettings: PlatformContext['updateSettings'] = (...args) => {
                calledWith = args
                return Promise.resolve()
            }
            const platformContext: Pick<
                PlatformContext,
                'updateSettings' | 'settings' | 'requestGraphQL' | 'getGraphQLClient' | 'clientApplication'
            > = {
                settings: of({
                    subjects: [
                        {
                            settings: null,
                            lastID: null,
                            subject: {
                                id: 'id1',
                                __typename: 'DefaultSettings',
                                viewerCanAdminister: true,
                                settingsURL: null,
                                latestSettings: null,
                            },
                        },
                        {
                            settings: null,
                            lastID: null,

                            subject: {
                                id: 'id2',
                                __typename: 'DefaultSettings',
                                viewerCanAdminister: true,
                                settingsURL: null,
                                latestSettings: null,
                            },
                        },
                    ],
                    final: { a: 'value' },
                }),
                updateSettings,
                getGraphQLClient,
                requestGraphQL: () => EMPTY,
                clientApplication: 'other',
            }

            const { api } = initMainThreadAPI(
                pretendRemote<FlatExtensionHostAPI>({ syncSettingsData: () => {} }),
                platformContext
            )

            const edit: SettingsEdit = { path: ['a'], value: 'newVal' }
            await api.applySettingsEdit(edit)

            expect(calledWith).toEqual(['id2', edit] as Parameters<PlatformContext['updateSettings']>)
        })

        test('changes of settings from platform propagated to the ext host', () => {
            const values: SettingsCascade<{ a: string }>[] = [
                {
                    subjects: [], // this is valid actually even though it shouldn't
                    final: { a: 'one' },
                },
                {
                    subjects: null as any, // invalid and should be ignored
                    final: { a: 'invalid two' },
                },
                {
                    subjects: [], // one more valid case
                    final: { a: 'three' },
                },
            ]

            const platformContext: Pick<
                PlatformContext,
                'updateSettings' | 'settings' | 'getGraphQLClient' | 'requestGraphQL' | 'clientApplication'
            > = {
                getGraphQLClient,
                settings: of(...values),
                updateSettings: () => Promise.resolve(),
                requestGraphQL: () => EMPTY,
                clientApplication: 'other',
            }

            const passedToExtensionHost: SettingsCascade<object>[] = []
            initMainThreadAPI(
                pretendRemote<FlatExtensionHostAPI>({
                    syncSettingsData: data => {
                        passedToExtensionHost.push(data)
                    },
                }),
                platformContext
            )

            expect(passedToExtensionHost).toEqual([values[0], values[2]] as SettingsCascade<{ a: string }>[])
        })

        test('changes of settings are not passed to ext host after unsub', () => {
            const values = new Subject<SettingsCascade<{ a: string }>>()
            const platformContext: Pick<
                PlatformContext,
                'updateSettings' | 'settings' | 'getGraphQLClient' | 'requestGraphQL' | 'clientApplication'
            > = {
                settings: values.asObservable(),
                updateSettings: () => Promise.resolve(),
                getGraphQLClient,
                requestGraphQL: () => EMPTY,
                clientApplication: 'other',
            }
            const passedToExtensionHost: SettingsCascade<object>[] = []
            const { subscription } = initMainThreadAPI(
                pretendRemote<FlatExtensionHostAPI>({
                    syncSettingsData: data => {
                        passedToExtensionHost.push(data)
                    },
                }),
                platformContext
            )

            const one = {
                subjects: [],
                final: { a: 'one' },
            }

            const two = {
                subjects: [],
                final: { a: 'two' },
            }

            values.next(one)
            expect(passedToExtensionHost).toEqual([one])

            subscription.unsubscribe()
            values.next(two)
            expect(passedToExtensionHost).toEqual([one]) // nothing happened after unsub
        })
    })
})
