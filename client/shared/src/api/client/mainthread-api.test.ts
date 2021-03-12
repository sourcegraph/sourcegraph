import { initMainThreadAPI } from './mainthread-api'
import { PlatformContext } from '../../platform/context'
import { BehaviorSubject, EMPTY, of, Subject } from 'rxjs'
import { pretendRemote } from '../util'
import { SettingsEdit } from './services/settings'
import { SettingsCascade } from '../../settings/settings'
import { FlatExtensionHostAPI } from '../contract'
import sinon from 'sinon'
import { SuccessGraphQLResult } from '../../graphql/graphql'

describe('MainThreadAPI', () => {
    // TODO(tj): commands, notifications

    describe('graphQL', () => {
        test('PlatformContext#requestGraphQL is called with the correct arguments', async () => {
            const requestGraphQL = sinon.spy(_options => EMPTY)

            const platformContext: Pick<
                PlatformContext,
                'updateSettings' | 'settings' | 'requestGraphQL' | 'getScriptURLForExtension' | 'sideloadedExtensionURL'
            > = {
                settings: EMPTY,
                updateSettings: () => Promise.resolve(),
                requestGraphQL,
                getScriptURLForExtension: () => undefined,
                sideloadedExtensionURL: new BehaviorSubject<string | null>(null),
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
                'updateSettings' | 'settings' | 'requestGraphQL' | 'getScriptURLForExtension' | 'sideloadedExtensionURL'
            > = {
                settings: EMPTY,
                updateSettings: () => Promise.resolve(),
                requestGraphQL,
                getScriptURLForExtension: () => undefined,
                sideloadedExtensionURL: new BehaviorSubject<string | null>(null),
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
                'updateSettings' | 'settings' | 'requestGraphQL' | 'getScriptURLForExtension' | 'sideloadedExtensionURL'
            > = {
                settings: of({
                    subjects: [
                        {
                            settings: null,
                            lastID: null,
                            subject: { id: 'id1', __typename: 'DefaultSettings', viewerCanAdminister: true },
                        },
                        {
                            settings: null,
                            lastID: null,

                            subject: { id: 'id2', __typename: 'DefaultSettings', viewerCanAdminister: true },
                        },
                    ],
                    final: { a: 'value' },
                }),
                updateSettings,
                requestGraphQL: () => EMPTY,
                getScriptURLForExtension: () => undefined,
                sideloadedExtensionURL: new BehaviorSubject<string | null>(null),
            }

            const { api } = initMainThreadAPI(pretendRemote({}), platformContext)

            const edit: SettingsEdit = { path: ['a'], value: 'newVal' }
            await api.applySettingsEdit(edit)

            expect(calledWith).toEqual<Parameters<PlatformContext['updateSettings']>>(['id2', edit])
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
                'updateSettings' | 'settings' | 'requestGraphQL' | 'getScriptURLForExtension' | 'sideloadedExtensionURL'
            > = {
                settings: of(...values),
                updateSettings: () => Promise.resolve(),
                requestGraphQL: () => EMPTY,
                getScriptURLForExtension: () => undefined,
                sideloadedExtensionURL: new BehaviorSubject<string | null>(null),
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

            expect(passedToExtensionHost).toEqual<SettingsCascade<{ a: string }>[]>([values[0], values[2]])
        })

        test('changes of settings are not passed to ext host after unsub', () => {
            const values = new Subject<SettingsCascade<{ a: string }>>()
            const platformContext: Pick<
                PlatformContext,
                'updateSettings' | 'settings' | 'requestGraphQL' | 'getScriptURLForExtension' | 'sideloadedExtensionURL'
            > = {
                settings: values.asObservable(),
                updateSettings: () => Promise.resolve(),
                requestGraphQL: () => EMPTY,
                getScriptURLForExtension: () => undefined,
                sideloadedExtensionURL: new BehaviorSubject<string | null>(null),
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
