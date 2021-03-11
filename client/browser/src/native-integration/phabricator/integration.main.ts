import '../../../../shared/src/polyfills'

import { setLinkComponent, AnchorLink } from '../../../../shared/src/components/Link'
import { injectCodeIntelligence } from '../../shared/code-hosts/shared/inject'
import { injectExtensionMarker } from '../../shared/code-hosts/sourcegraph/inject'
import { getPhabricatorCSS, getSourcegraphURLFromConduit } from '../../shared/code-hosts/phabricator/backend'
import { metaClickOverride } from './util'
import { getAssetsURL } from '../../shared/util/context'

// Just for informational purposes (see getPlatformContext())
window.SOURCEGRAPH_PHABRICATOR_EXTENSION = true

const IS_EXTENSION = false

setLinkComponent(AnchorLink)

async function init(): Promise<void> {
    /**
     * This is the main entry point for the phabricator in-page JavaScript plugin.
     */
    if (window.localStorage && window.localStorage.getItem('SOURCEGRAPH_DISABLED') === 'true') {
        const value = window.localStorage.getItem('SOURCEGRAPH_DISABLED')
        console.log(
            `Sourcegraph on Phabricator is disabled because window.localStorage.getItem('SOURCEGRAPH_DISABLED') is set to ${String(
                value
            )}.`
        )
        return
    }

    const sourcegraphURL =
        window.localStorage.getItem('SOURCEGRAPH_URL') ||
        window.SOURCEGRAPH_URL ||
        (await getSourcegraphURLFromConduit())
    const assetsURL = getAssetsURL(sourcegraphURL)

    // Backwards compat: Support Legacy Phabricator extension. Check that the Phabricator integration
    // passed the bundle url. Legacy Phabricator extensions inject CSS via the loader.js script
    // so we do not need to do this here.
    if (!window.SOURCEGRAPH_BUNDLE_URL && !window.localStorage.getItem('SOURCEGRAPH_BUNDLE_URL')) {
        injectExtensionMarker()
        await injectCodeIntelligence({ sourcegraphURL, assetsURL }, IS_EXTENSION)
        metaClickOverride()
        return
    }

    window.SOURCEGRAPH_URL = sourcegraphURL
    const css = await getPhabricatorCSS(sourcegraphURL)
    const style = document.createElement('style')
    style.setAttribute('type', 'text/css')
    style.id = 'sourcegraph-styles'
    style.textContent = css
    document.head.append(style)
    window.localStorage.setItem('SOURCEGRAPH_URL', sourcegraphURL)
    metaClickOverride()
    injectExtensionMarker()
    await injectCodeIntelligence({ sourcegraphURL, assetsURL }, IS_EXTENSION)
}

init().catch(error => console.error('Error initializing Phabricator integration', error))
