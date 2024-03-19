import { ManifestBuilder } from './manifestPlugin'

export interface WebBuildManifest {
    /** Main JS bundle. */
    'main.js': string

    /** Main CSS bundle. */
    'main.css': string

    /** Embed JS bundle. */
    'embed.js': string

    /** Embed CSS bundle. */
    'embed.css': string
}

export const assetPathPrefix = '/.assets'

export const WEB_BUILD_MANIFEST_FILENAME = 'web.manifest.json'

export const webManifestBuilder = new ManifestBuilder<WebBuildManifest>({
    // *.tsx are the entry files for normal builds
    // *.js are the entry files for bazel builds
    'main.js': (asset, output) => (/\/enterprise\/main\.(tsx|js)$/.test(output.entryPoint ?? '') ? asset : undefined),
    'main.css': (_asset, output) =>
        (/enterprise\/main\.(tsx|js)$/.test(output.entryPoint ?? '') && output.cssBundle) || undefined,
    'embed.js': (asset, output) => (/\/embed\/embedMain\.(tsx|js)$/.test(output.entryPoint ?? '') ? asset : undefined),
    'embed.css': (_asset, output) =>
        (/\/embed\/embedMain\.(tsx|js)$/.test(output.entryPoint ?? '') && output.cssBundle) || undefined,
})
