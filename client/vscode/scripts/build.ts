import { existsSync } from 'fs'
import path from 'path'

import * as esbuild from 'esbuild'
import { rm } from 'shelljs'

import {
    packageResolutionPlugin,
    stylePlugin,
    workerPlugin,
    buildTimerPlugin,
} from '@sourcegraph/build-config/src/esbuild/plugins'

const minify = process.env.NODE_ENV === 'production'
const outdir = path.join(__dirname, '../dist')
const isTest = !!process.env.IS_TEST

const TARGET_TYPE = process.env.TARGET_TYPE

const SHARED_CONFIG = {
    outdir,
    minify,
    sourcemap: true,
}

export async function build(): Promise<void> {
    if (existsSync(outdir)) {
        rm('-rf', outdir)
    }

    const buildPromises: Promise<esbuild.BuildContext>[] = []

    if (TARGET_TYPE === 'node' || !TARGET_TYPE) {
        buildPromises.push(
            esbuild.context({
                entryPoints: { extension: path.join(__dirname, '/../src/extension.ts') },
                bundle: true,
                format: 'cjs',
                platform: 'node',
                external: ['vscode'],
                banner: { js: 'global.Buffer = require("buffer").Buffer' },
                define: {
                    'process.env.IS_TEST': isTest ? 'true' : 'false',
                },
                ...SHARED_CONFIG,
                outdir: path.join(SHARED_CONFIG.outdir, 'node'),
                mainFields: ['module', 'main'], // for node-jsonc-parser; see https://github.com/microsoft/node-jsonc-parser/issues/57#issuecomment-1634726605
            })
        )
    }
    if (TARGET_TYPE === 'webworker' || !TARGET_TYPE) {
        buildPromises.push(
            esbuild.context({
                entryPoints: { extension: path.join(__dirname, '/../src/extension.ts') },
                bundle: true,
                format: 'cjs',
                platform: 'browser',
                external: ['vscode'],
                define: {
                    'process.env.IS_TEST': isTest ? 'true' : 'false',
                    global: 'globalThis',
                },
                inject: ['./scripts/process-shim.js', './scripts/buffer-shim.js'],
                plugins: [
                    packageResolutionPlugin({
                        process: require.resolve('process/browser'),
                        path: require.resolve('path-browserify'),
                        http: require.resolve('stream-http'),
                        https: require.resolve('https-browserify'),
                        stream: require.resolve('stream-browserify'),
                        util: require.resolve('util'),
                        events: require.resolve('events'),
                        buffer: require.resolve('buffer/'),
                        './browserActionsNode': path.resolve(__dirname, '../src', 'commands', 'browserActionsWeb'),
                        'http-proxy-agent': path.resolve(
                            __dirname,
                            '../src',
                            'backend',
                            'proxy-agent-fake-for-browser.ts'
                        ),
                        'https-proxy-agent': path.resolve(
                            __dirname,
                            '../src',
                            'backend',
                            'proxy-agent-fake-for-browser.ts'
                        ),
                        'node-fetch': path.resolve(__dirname, '../src', 'backend', 'node-fetch-fake-for-browser.ts'),
                    }),
                ],
                ...SHARED_CONFIG,
                outdir: path.join(SHARED_CONFIG.outdir, 'webworker'),
            })
        )
    }

    buildPromises.push(
        esbuild.context({
            entryPoints: {
                helpSidebar: path.join(__dirname, '../src/webview/sidebars/help'),
                searchSidebar: path.join(__dirname, '../src/webview/sidebars/search'),
                searchPanel: path.join(__dirname, '../src/webview/search-panel'),
                style: path.join(__dirname, '../src/webview/index.scss'),
            },
            bundle: true,
            format: 'esm',
            platform: 'browser',
            splitting: true,
            plugins: [
                stylePlugin,
                workerPlugin,
                packageResolutionPlugin({
                    path: require.resolve('path-browserify'),
                    process: require.resolve('process/browser'),
                    http: require.resolve('stream-http'), // for stream search - event source polyfills
                    https: require.resolve('https-browserify'), // for stream search - event source polyfills
                    './RepoSearchResult': require.resolve('../src/webview/search-panel/alias/RepoSearchResult'),
                    './CommitSearchResult': require.resolve('../src/webview/search-panel/alias/CommitSearchResult'),
                    './SymbolSearchResult': require.resolve('../src/webview/search-panel/alias/SymbolSearchResult'),
                    './FileMatchChildren': require.resolve('../src/webview/search-panel/alias/FileMatchChildren'),
                    './RepoFileLink': require.resolve('../src/webview/search-panel/alias/RepoFileLink'),
                    '../documentation/ModalVideo': require.resolve('../src/webview/search-panel/alias/ModalVideo'),
                }),
                buildTimerPlugin,
            ],
            loader: {
                '.ttf': 'file',
            },
            define: {
                'process.env.INTEGRATION_TESTS': isTest ? 'true' : 'false',
            },
            assetNames: '[name]',
            ...SHARED_CONFIG,
            outdir: path.join(SHARED_CONFIG.outdir, 'webview'),
        })
    )

    const ctxs = await Promise.all(buildPromises)

    await Promise.all(ctxs.map(ctx => ctx.rebuild()))

    if (process.env.WATCH) {
        await Promise.all(ctxs.map(ctx => ctx.watch()))
        await new Promise(() => {}) // wait forever
    }

    await Promise.all(ctxs.map(ctx => ctx.dispose()))
}

if (require.main === module) {
    build().catch(error => {
        console.error('Error:', error)
        process.exit(1)
    })
}
