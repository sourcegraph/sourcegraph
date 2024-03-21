import '../../../platform/polyfills'

import { startExtensionHost } from '@sourcegraph/shared/src/api/extension/extensionHost'

import { createEndpointsForWebToWeb } from '../../../comlink/webviewEndpoint'

async function extensionHostMain(): Promise<void> {
    try {
        const { webview: proxy, worker: expose } = createEndpointsForWebToWeb({
            postMessage: message => self.postMessage(message),
            addEventListener: (type, listener, options) => self.addEventListener(type, listener, options),
            removeEventListener: (type, listener, options) => self.removeEventListener(type, listener, options),
        })

        const extensionHost = startExtensionHost({ proxy, expose })
        self.addEventListener('unload', () => extensionHost.unsubscribe())
    } catch (error) {
        console.error('Error starting the extension host:', error)
        self.close()
    }
    return Promise.resolve()
}

// eslint-disable-next-line @typescript-eslint/no-floating-promises
extensionHostMain()
