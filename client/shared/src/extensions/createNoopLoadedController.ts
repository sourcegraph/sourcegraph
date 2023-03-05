import { Remote } from 'comlink'

import { ExposedToClient, initMainThreadAPI } from '../api/client/mainthread-api'
import { FlatExtensionHostAPI } from '../api/contract'
import { createExtensionHostAPI } from '../api/extension/extensionHostApi'
import { createExtensionHostState } from '../api/extension/extensionHostState'
import { pretendRemote, syncPromiseSubscription } from '../api/util'
import { PlatformContext } from '../platform/context'
import { isSettingsValid } from '../settings/settings'

import { Controller } from './controller'

export function createNoopController(platformContext: PlatformContext): Controller {
    const api: Promise<{
        remoteExtensionHostAPI: Remote<FlatExtensionHostAPI>
        exposedToClient: ExposedToClient
    }> = new Promise((resolve, reject) => {
        platformContext.settings.subscribe(settingsCascade => {
            ;(async () => {
                const [injectNewCodeintel, newSettingsGetter] = await Promise.all([
                    import('../codeintel/api').then(module => module.injectNewCodeintel),
                    import('../codeintel/legacy-extensions/api').then(module => module.newSettingsGetter),
                ])

                if (!isSettingsValid(settingsCascade)) {
                    throw new Error('Settings are not valid')
                }

                const extensionHostState = createExtensionHostState(
                    {
                        clientApplication: 'sourcegraph',
                        initialSettings: settingsCascade,
                    },
                    null,
                    null
                )
                const extensionHostAPI = injectNewCodeintel(createExtensionHostAPI(extensionHostState), {
                    requestGraphQL: platformContext.requestGraphQL,
                    telemetryService: platformContext.telemetryService,
                    settings: newSettingsGetter(settingsCascade),
                })
                const remoteExtensionHostAPI = pretendRemote(extensionHostAPI)
                const exposedToClient = initMainThreadAPI(remoteExtensionHostAPI, platformContext).exposedToClient

                // We don't have to load any extensions so we are already done
                extensionHostState.haveInitialExtensionsLoaded.next(true)

                return { remoteExtensionHostAPI, exposedToClient }
            })().then(resolve, reject)
        })
    })
    return {
        executeCommand: parameters => api.then(({ exposedToClient }) => exposedToClient.executeCommand(parameters)),
        registerCommand: entryToRegister =>
            syncPromiseSubscription(
                api.then(({ exposedToClient }) => exposedToClient.registerCommand(entryToRegister))
            ),
        extHostAPI: api.then(({ remoteExtensionHostAPI }) => remoteExtensionHostAPI),
        unsubscribe: () => {},
    }
}
