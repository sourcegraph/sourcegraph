import { Subscription } from 'rxjs'

import type { ClosableEndpointPair } from '@sourcegraph/shared/src/platform/context'

import { createEndpointsForWebToWeb } from '../../../comlink/webviewEndpoint'

/* eslint-disable import/extensions, @typescript-eslint/ban-ts-comment */
// @ts-ignore
import ExtensionHostWorker from './main.worker.ts'

/* eslint-enable import/extensions, @typescript-eslint/ban-ts-comment */

export function createExtensionHost(): ClosableEndpointPair {
    const worker = new ExtensionHostWorker()
    const { webview: expose, worker: proxy } = createEndpointsForWebToWeb(worker)

    return { endpoints: { expose, proxy }, subscription: new Subscription(() => worker.terminate()) }
}
