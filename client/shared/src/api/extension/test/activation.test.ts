import { BehaviorSubject, of } from 'rxjs'
import { filter, first } from 'rxjs/operators'
import sinon from 'sinon'
import sourcegraph from 'sourcegraph'

import { SettingsCascade } from '../../../settings/settings'
import { MainThreadAPI } from '../../contract'
import { pretendRemote } from '../../util'
import { activateExtensions, ExecutableExtension } from '../activation'
import { ExtensionHostState } from '../extensionHostState'

describe('Extension activation', () => {
    describe('activateExtensions()', () => {
        it('logs events for activated extensions', async () => {
            const logEvent = sinon.spy()

            const mockMain = pretendRemote<Pick<MainThreadAPI, 'logEvent'>>({
                logEvent,
            })

            const FIXTURE_EXTENSION: ExecutableExtension = {
                scriptURL: 'https://fixture.extension',
                id: 'sourcegraph/fixture-extension',
                manifest: { url: 'a', activationEvents: ['*'] },
            }

            const haveInitialExtensionsLoaded = new BehaviorSubject<boolean>(false)

            const mockState: Pick<ExtensionHostState, 'activeExtensions' | 'haveInitialExtensionsLoaded' | 'settings'> =
                {
                    activeExtensions: new BehaviorSubject([FIXTURE_EXTENSION]),
                    haveInitialExtensionsLoaded,
                    settings: new BehaviorSubject<Readonly<SettingsCascade>>({
                        subjects: [],
                        final: {},
                    }),
                }

            // Noop for activation and deactivation
            const noopPromise = () => Promise.resolve()

            activateExtensions(
                mockState,
                mockMain,
                function createExtensionAPI() {
                    return {} as typeof sourcegraph
                },
                of(true),
                noopPromise,
                noopPromise
            )

            // Wait for extensions to load to check on the spy
            await haveInitialExtensionsLoaded
                .pipe(
                    filter(haveLoaded => haveLoaded),
                    first()
                )
                .toPromise()

            sinon.assert.calledWith(logEvent, 'ExtensionActivation', { extension_id: 'sourcegraph/fixture-extension' })
        })
    })
})
