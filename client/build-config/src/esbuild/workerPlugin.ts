import type * as esbuild from 'esbuild'

import { packageResolutionPlugin } from './packageResolutionPlugin'

/**
 * Starts a new esbuild build to create a bundle for a Web Worker.
 *
 * @param esbuild The esbuild instance to use for building.
 * @param workerPath The path to the worker file.
 * @param extraConfig Additional build options to use for the worker bundle.
 */
async function buildWorker(
    esbuild: esbuild.PluginBuild['esbuild'],
    workerPath: string,
    extraConfig: Pick<esbuild.BuildOptions, 'target' | 'format' | 'minify'>
): Promise<{ workerBundle: string } & Pick<esbuild.OnResolveResult, 'watchFiles' | 'errors' | 'warnings'>> {
    const results = await esbuild.build({
        entryPoints: [workerPath],
        bundle: true,
        write: false,
        plugins: [
            packageResolutionPlugin({
                path: require.resolve('path-browserify'),
            }),
        ],
        // Use the minify option as an indicator for running in dev mode.
        // We only enable sourcmapping in dev mode, otherwise 'inline' sourcemaps
        // would more than double the size of the data URL in the worker bundle in production.
        sourcemap: extraConfig.minify ? false : 'inline',
        metafile: true,
        ...extraConfig,
    })
    return {
        workerBundle: results.outputFiles[0].text,
        watchFiles: results.metafile && Object.keys(results.metafile.inputs),
        errors: results.errors,
        warnings: results.warnings,
    }
}

/**
 * An esbuild plugin that bundles a Web Worker (classic, not a module worker) given an import of a
 * `.worker.ts` file.
 *
 * TODO(sqs): This could be improved to use the new `new Worker(new URL(..., import.meta.url))`
 * worker syntax that many bundlers are starting to prefer.
 */
export const workerPlugin: esbuild.Plugin = {
    name: 'worker',
    setup: build => {
        build.onResolve(
            // Let .js, .ts be optional to play nice with TypeScript settings
            // (which expects no file extensions for .ts and .js files)
            { filter: /\.worker(\.(js|ts))?$/, namespace: 'file' },
            async ({ path: workerPath, importer, resolveDir, pluginData }) => {
                // This plugin is only needed for classic Web Workers (which need to be bundled
                // independently), not module Web Workers (which don't need a separate bundle
                // because they can `import` shared modules just like non-worker code). Assume that
                // workers specified as entrypoints in the build are module workers, and skip them.
                const isEntrypoint = importer === ''
                if (isEntrypoint || pluginData?.skipInlineWorkerResolve) {
                    return undefined
                }
                return {
                    // Determine the real location of the worker file. The worker might be a local file
                    // or from a package, so we need to resolve it using the usual package resolution rules.
                    ...(await build.resolve(workerPath, {
                        kind: 'dynamic-import',
                        resolveDir,
                        pluginData: { skipInlineWorkerResolve: true },
                    })),
                    namespace: 'worker',
                }
            }
        )
        build.onLoad({ filter: /./, namespace: 'worker' }, async ({ path: workerPath }) => {
            // Using build.esbuild instead of importing esbuild directly from 'esbuild' ensures that
            // the same esbuild version is used throughout the build.
            // (otherwise the version imported from 'esbuild' could be different from the version used by bazel)
            const { workerBundle, watchFiles, errors, warnings } = await buildWorker(build.esbuild, workerPath, {
                target: build.initialOptions.target,
                format: build.initialOptions.format,
                minify: build.initialOptions.minify,
            })

            return {
                // It seems necessary to split this implementation into two modules.
                // Trying to use a single module caused to worker to throw an inifinite recursion error.
                contents: `
                import inlineWorker from '__inline-worker'
                     export default function Worker() {
                        return inlineWorker(${JSON.stringify(workerBundle)})
                    }
`,
                loader: 'js',
                watchFiles,
                errors,
                warnings,
            }
        })
        build.onResolve({ filter: /^__inline-worker$/ }, ({ path }) => ({ path, namespace: 'inline-worker' }))
        build.onLoad({ filter: /./, namespace: 'inline-worker' }, () => ({
            contents: `
                export default function inlineWorker(scriptText) {
                    const blob = new Blob([scriptText], { type: 'text/javascript' })
                    const url = URL.createObjectURL(blob)
                    const worker = new Worker(url)
                    URL.revokeObjectURL(url)
                    return worker
                }`,
            loader: 'js',
        }))
    },
}
