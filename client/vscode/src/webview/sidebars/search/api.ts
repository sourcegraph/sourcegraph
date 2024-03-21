import { of } from 'rxjs'

import { proxySubscribable } from '@sourcegraph/shared/src/api/extension/api/common'

import type { SearchSidebarAPI } from '../../../contract'
import type { WebviewPageProps } from '../../platform/context'

import { createVSCodeExtensionsController } from './extension-host'

export function createSearchSidebarAPI(
    webviewPageProps: Pick<WebviewPageProps, 'platformContext' | 'instanceURL'>
): SearchSidebarAPI {
    return {
        ping: () => {
            console.log('ping called')
            return proxySubscribable(of('pong'))
        },
        ...createVSCodeExtensionsController(webviewPageProps),
    }
}
