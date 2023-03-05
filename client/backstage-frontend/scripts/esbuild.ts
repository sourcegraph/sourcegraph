import { existsSync } from 'fs'
import path from 'path'
import { nodeExternalsPlugin } from 'esbuild-node-externals'

import * as esbuild from 'esbuild'
import { rm } from 'shelljs'

import { stylePlugin, buildTimerPlugin, WORKSPACES_PATH } from '@sourcegraph/build-config'

const PACKAGE_ROOT_PATH = path.resolve(WORKSPACES_PATH, 'backstage-frontend')
const DIST_PATH = path.resolve(PACKAGE_ROOT_PATH, 'dist')

async function build(): Promise<void> {
    if (existsSync(DIST_PATH)) {
        rm('-rf', DIST_PATH)
    }

    await esbuild.build({
        entryPoints: [path.resolve(PACKAGE_ROOT_PATH, 'src', 'index.ts')],
        bundle: true,
        format: 'esm',
        logLevel: 'error',
        jsx: 'automatic',
        external: ['@backstage/*', '@material-ui/*', 'react-use', 'react', 'react-dom'],
        plugins: [stylePlugin, buildTimerPlugin, nodeExternalsPlugin()],
        // mainFields: ['browser', 'module', 'main'],
        // platform: 'browser',
        define: {
            'process.env.IS_TEST': 'false',
            global: 'window',
        },
        loader: {
            '.yaml': 'text',
            '.ttf': 'file',
            '.png': 'file',
        },
        treeShaking: true,
        target: 'esnext',
        sourcemap: true,
        outdir: DIST_PATH,
    })
}

if (require.main == module) {
    build()
        .catch(error => console.error('Error:', error))
        .finally(() => process.exit(0))
}
