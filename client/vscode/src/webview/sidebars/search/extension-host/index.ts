import { from } from 'rxjs'
import { switchMap } from 'rxjs/operators'
import type { Intersection } from 'utility-types'

import { wrapRemoteObservable } from '@sourcegraph/shared/src/api/client/api/common'
import type { FlatExtensionHostAPI } from '@sourcegraph/shared/src/api/contract'
import { proxySubscribable } from '@sourcegraph/shared/src/api/extension/api/common'
import { createController as createExtensionsController } from '@sourcegraph/shared/src/extensions/createSyncLoadedController'

import type { SearchSidebarAPI } from '../../../../contract'
import type { WebviewPageProps } from '../../../platform/context'

import { createExtensionHost } from './worker'

export function createVSCodeExtensionsController({
    platformContext,
    instanceURL,
}: Pick<WebviewPageProps, 'platformContext' | 'instanceURL'>): Intersection<SearchSidebarAPI, FlatExtensionHostAPI> {
    const extensionsController = createExtensionsController({
        ...platformContext,
        sourcegraphURL: instanceURL,
        createExtensionHost: () => Promise.resolve(createExtensionHost()),
    })

    return {
        getDefinition: parameters => {
            const definitions = from(extensionsController.extHostAPI).pipe(
                switchMap(extensionHostAPI => wrapRemoteObservable(extensionHostAPI.getDefinition(parameters)))
            )

            return proxySubscribable(definitions)
        },
        getReferences: (parameters, referenceContext) => {
            const references = from(extensionsController.extHostAPI).pipe(
                switchMap(extensionHostAPI =>
                    wrapRemoteObservable(extensionHostAPI.getReferences(parameters, referenceContext))
                )
            )

            return proxySubscribable(references)
        },
        getHover: parameters => {
            const hovers = from(extensionsController.extHostAPI).pipe(
                switchMap(extensionHostAPI => wrapRemoteObservable(extensionHostAPI.getHover(parameters)))
            )

            return proxySubscribable(hovers)
        },

        addTextDocumentIfNotExists: textDocumentData =>
            extensionsController.extHostAPI.then(extensionHostAPI =>
                extensionHostAPI.addTextDocumentIfNotExists(textDocumentData)
            ),
        addViewerIfNotExists: viewer =>
            extensionsController.extHostAPI.then(extensionHostAPI => extensionHostAPI.addViewerIfNotExists(viewer)),
    }
}
