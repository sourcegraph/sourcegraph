import path from 'path'

import { sentryEsbuildPlugin } from '@sentry/esbuild-plugin'
import type * as esbuild from 'esbuild'

import { ROOT_PATH, STATIC_ASSETS_PATH } from '@sourcegraph/build-config'
import {
    stylePlugin,
    packageResolutionPlugin,
    monacoPlugin,
    buildTimerPlugin,
    workerPlugin,
} from '@sourcegraph/build-config/src/esbuild/plugins'
import { MONACO_LANGUAGES_AND_FEATURES } from '@sourcegraph/build-config/src/monaco-editor'

import type { EnvironmentConfig } from '../utils'

import { manifestPlugin } from './manifestPlugin'
import { WEB_BUILD_MANIFEST_FILENAME, webManifestBuilder } from './webmanifest'

/**
 * Creates esbuild build options for the client/web app.
 */
export function esbuildBuildOptions(ENVIRONMENT_CONFIG: EnvironmentConfig): esbuild.BuildOptions {
    return {
        entryPoints: ENVIRONMENT_CONFIG.CODY_APP
            ? [path.join(ROOT_PATH, 'client/web/src/enterprise/app/main.tsx')]
            : [
                  path.join(ROOT_PATH, 'client/web/src/enterprise/main.tsx'),
                  path.join(ROOT_PATH, 'client/web/src/enterprise/embed/embedMain.tsx'),
              ],
        bundle: true,
        minify: ENVIRONMENT_CONFIG.NODE_ENV === 'production',
        treeShaking: true,

        format: 'esm',
        logLevel: 'error',
        jsx: 'automatic',
        jsxDev: ENVIRONMENT_CONFIG.NODE_ENV === 'development',
        splitting: !ENVIRONMENT_CONFIG.DEV_WEB_BUILDER_NO_SPLITTING,
        chunkNames: 'chunks/chunk-[name]-[hash]',
        entryNames: '[name]-[hash]',
        outdir: STATIC_ASSETS_PATH,
        plugins: [
            stylePlugin,
            manifestPlugin({
                manifestFilename: WEB_BUILD_MANIFEST_FILENAME,
                builder: webManifestBuilder,
            }),
            workerPlugin,
            packageResolutionPlugin({
                path: require.resolve('path-browserify'),
                ...(ENVIRONMENT_CONFIG.DEV_WEB_BUILDER_OMIT_SLOW_DEPS
                    ? {
                          // Monaco
                          '@sourcegraph/shared/src/components/MonacoEditor':
                              '@sourcegraph/shared/src/components/NoMonacoEditor',
                          'monaco-editor': '/dev/null',
                          'monaco-editor/esm/vs/editor/editor.api': '/dev/null',
                          'monaco-yaml': '/dev/null',

                          // GraphiQL
                          './api/ApiConsole': path.join(ROOT_PATH, 'client/web/src/api/NoApiConsole.tsx'),
                          '@graphiql/react': '/dev/null',
                          graphiql: '/dev/null',

                          // Misc.
                          recharts: '/dev/null',

                          // TODO(sqs): force use of same version when developing on opencodegraph because `pnpm link` breaks
                          '@codemirror/state': path.join(ROOT_PATH, 'node_modules/@codemirror/state'),
                          '@codemirror/view': path.join(ROOT_PATH, 'node_modules/@codemirror/view'),
                          react: path.join(ROOT_PATH, 'node_modules/react'),
                          'react-dom': path.join(ROOT_PATH, 'node_modules/react-dom'),
                          'react-dom/client': path.join(ROOT_PATH, 'node_modules/react-dom/client'),
                      }
                    : null),
            }),
            ENVIRONMENT_CONFIG.DEV_WEB_BUILDER_OMIT_SLOW_DEPS ? null : monacoPlugin(MONACO_LANGUAGES_AND_FEATURES),
            buildTimerPlugin,
            ENVIRONMENT_CONFIG.SENTRY_UPLOAD_SOURCE_MAPS
                ? sentryEsbuildPlugin({
                      org: ENVIRONMENT_CONFIG.SENTRY_ORGANIZATION,
                      project: ENVIRONMENT_CONFIG.SENTRY_PROJECT,
                      authToken: ENVIRONMENT_CONFIG.SENTRY_DOT_COM_AUTH_TOKEN,
                      silent: true,
                      release: { name: `frontend@${ENVIRONMENT_CONFIG.VERSION}` },
                      sourcemaps: { assets: [path.join('dist', '*.map'), path.join('dist', 'chunks', '*.map')] },
                  })
                : null,
        ].filter((plugin): plugin is esbuild.Plugin => plugin !== null),
        define: {
            ...Object.fromEntries(
                Object.entries({ ...ENVIRONMENT_CONFIG, SOURCEGRAPH_API_URL: undefined }).map(([key, value]) => [
                    `process.env.${key}`,
                    JSON.stringify(value === undefined ? null : value),
                ])
            ),
            global: 'window',
        },
        loader: {
            '.yaml': 'text',
            '.ttf': 'file',
            '.woff2': 'file',
            '.png': 'file',
        },
        target: 'esnext',
        sourcemap: true,
    }
}
