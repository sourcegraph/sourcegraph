import '../../../shared/src/polyfills'

import { setLinkComponent, AnchorLink } from '../../../shared/src/components/Link'
import { injectCodeIntelligence } from '../shared/code-hosts/shared/inject'
import {
    EXTENSION_MARKER_ID,
    injectExtensionMarker,
    NATIVE_INTEGRATION_ACTIVATED,
} from '../shared/code-hosts/sourcegraph/inject'
import { getAssetsURL } from '../shared/util/context'

const IS_EXTENSION = false

setLinkComponent(AnchorLink)

function init(): void {
    console.log('Sourcegraph native integration is running')
    const sourcegraphURL = window.SOURCEGRAPH_URL
    if (!sourcegraphURL) {
        throw new Error('window.SOURCEGRAPH_URL is undefined')
    }

    const assetsURL = getAssetsURL(sourcegraphURL)

    if (document.querySelector(`#${EXTENSION_MARKER_ID}`) !== null) {
        // If the extension marker already exists, it means the browser extension is currently executing.
        // Dispatch a custom event to signal that browser extension resources should be cleaned up.
        document.dispatchEvent(new CustomEvent<{}>(NATIVE_INTEGRATION_ACTIVATED))
    } else {
        injectExtensionMarker()
    }
    const link = document.createElement('link')
    link.setAttribute('rel', 'stylesheet')
    link.setAttribute('type', 'text/css')
    link.setAttribute('href', new URL('css/style.bundle.css', assetsURL).href)
    link.id = 'sourcegraph-styles'
    document.head.append(link)
    window.localStorage.setItem('SOURCEGRAPH_URL', sourcegraphURL)
    window.SOURCEGRAPH_URL = sourcegraphURL
    // TODO handle subscription
    injectCodeIntelligence({ sourcegraphURL, assetsURL }, IS_EXTENSION).catch(error => {
        console.error('Error injecting Sourcegraph code intelligence:', error)
    })
}

init()
