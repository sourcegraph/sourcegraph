import { readFileSync } from 'fs'
import path from 'path'

import type { SourcegraphContext } from '../../src/jscontext'
import { assetPathPrefix, WEB_BUILD_MANIFEST_FILENAME, type WebBuildManifest } from '../esbuild/webmanifest'

import { createJsContext, ENVIRONMENT_CONFIG, HTTPS_WEB_SERVER_URL } from '.'

const { STATIC_ASSETS_PATH } = ENVIRONMENT_CONFIG

const WEB_BUILD_MANIFEST_PATH = path.resolve(STATIC_ASSETS_PATH, WEB_BUILD_MANIFEST_FILENAME)
export const HTML_INDEX_PATH = path.resolve(STATIC_ASSETS_PATH, 'index.html')

export const getWebBuildManifest = (): WebBuildManifest =>
    JSON.parse(readFileSync(WEB_BUILD_MANIFEST_PATH, 'utf-8')) as WebBuildManifest

interface GetHTMLPageOptions {
    manifestFile: WebBuildManifest
    /**
     * Used to inject dummy `window.context` in integration tests.
     */
    jsContext?: SourcegraphContext
    /**
     * Used to inject `window.context` received from the API proxy.
     */
    jsContextScript?: string
}

/**
 * Returns an HTML page similar to `cmd/frontend/internal/app/ui/app.html` but when running
 * without the `frontend` service.
 *
 * Note: This page should be kept as close as possible to `app.html` to avoid any inconsistencies
 * between our development server and the actual production server.
 */
export function getIndexHTML(options: GetHTMLPageOptions): string {
    const { manifestFile, jsContext, jsContextScript } = options

    const { 'main.js': mainJS, 'main.css': mainCSS } = manifestFile

    return `
<!DOCTYPE html>
<html lang="en">
    <head>
        <title>Sourcegraph</title>
        <meta charset="utf-8">
        <meta name="viewport" content="width=device-width, viewport-fit=cover" />
        <meta name="referrer" content="origin-when-cross-origin"/>
        <meta name="color-scheme" content="light dark"/>
        <link rel="stylesheet" href="${assetPathPrefix}/${mainCSS}">
        ${
            ENVIRONMENT_CONFIG.SOURCEGRAPHDOTCOM_MODE
                ? '<script src="https://js.sentry-cdn.com/ae2f74442b154faf90b5ff0f7cd1c618.min.js" crossorigin="anonymous"></script>'
                : ''
        }
    </head>
    <body>
        <div id="root"></div>
        <script>
            ${
                jsContextScript ||
                `
                // Required mock of the JS context object.
                window.context = ${JSON.stringify(
                    jsContext ?? createJsContext({ sourcegraphBaseUrl: `${HTTPS_WEB_SERVER_URL}` })
                )}
            `
            }
        </script>

        <script src="${assetPathPrefix}/${mainJS}" type="module"></script>
    </body>
</html>
`
}
