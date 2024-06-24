import { join } from 'path'

import { sveltekit } from '@sveltejs/kit/vite'
import AutoImport from 'unplugin-auto-import/vite'
import { FileSystemIconLoader } from 'unplugin-icons/loaders'
import IconsResolver from 'unplugin-icons/resolver'
import Icons from 'unplugin-icons/vite'
import { defineConfig, mergeConfig, type UserConfig } from 'vite'
import inspect from 'vite-plugin-inspect'
import type { UserConfig as VitestUserConfig } from 'vitest'

import graphqlCodegen from './dev/vite-graphql-codegen'

const BAZEL = !!process.env.BAZEL

export default defineConfig(({ mode }) => {
    // Using & VitestUserConfig shouldn't be necessary but without it `svelte-check` complains when run
    // in bazel. It's not clear what needs to be done to make it work without it, just like it does
    // locally.
    let config: UserConfig & VitestUserConfig = {
        plugins: [
            sveltekit(),
            AutoImport({
                // Ignore TS when running Bazel. It would try to write to the file which is not
                // possible in bazel.
                dts: BAZEL ? false : './src/auto-imports.d.ts',
                resolvers: [
                    IconsResolver({
                        prefix: 'i',
                        customCollections: ['sg', 'symbol'],
                    }),
                ],
            }),
            Icons({
                compiler: 'svelte',
                customCollections: {
                    sg: FileSystemIconLoader('./assets/icons'),
                    symbol: FileSystemIconLoader('./assets/symbol-icons'),
                },
            }),
            // Generates typescript types for gql-tags and .gql files
            graphqlCodegen(),
            inspect(),
        ],
        build: {
            sourcemap: true,
        },
        define:
            mode === 'test'
                ? {}
                : {
                      'process.platform': '"browser"',
                      'process.env.VITEST': 'null',
                      'process.env.NODE_ENV': `"${mode}"`,
                      'process.env.SOURCEGRAPH_API_URL': JSON.stringify(process.env.SOURCEGRAPH_API_URL),
                      'process.env': '{}',
                  },
        css: {
            preprocessorOptions: {
                scss: {
                    includePaths: [
                        // Our scss files and scss files in client/* often import global styles via @import 'wildcard/src/...'
                        // Adding '..' as load path causes scss to look for these imports in the client folder.
                        // (without it scss @import paths are always relative to the importing file)
                        join(__dirname, '..'),
                    ],
                    additionalData: `@use '$lib/styles/breakpoints.scss';`,
                },
            },
            modules: {
                localsConvention: 'camelCase',
            },
        },
        server: {
            // Allow setting the port via env variables to make it easier to integrate with
            // our existing caddy setup (which proxies requests to a specific port).
            port: process.env.SK_PORT ? +process.env.SK_PORT : undefined,
            strictPort: !!process.env.SV_PORT,
            proxy: {
                // Proxy requests to specific endpoints to a real Sourcegraph
                // instance.
                '^(/sign-in|/.assets|/-|/.api|/search/stream|/users|/notebooks|/insights|/batch-changes)|/-/(raw|compare|own|code-graph|batch-changes|settings)(/|$)':
                    {
                        target: process.env.SOURCEGRAPH_API_URL || 'https://sourcegraph.com',
                        changeOrigin: true,
                        secure: false,
                    },
            },
        },

        resolve: {
            alias: [
                // Unclear why Vite fails. It claims that index.esm.js doesn't have this export (it does).
                // Rewriting this to index.js fixes the issue. Error:
                // import { CiWarning, CiSettings, CiTextAlignLeft } from "react-icons/ci/index.esm.js";
                //                     ^^^^^^^^^^
                // SyntaxError: Named export 'CiSettings' not found. The requested module 'react-icons/ci/index.esm.js'
                // is a CommonJS module, which may not support all module.exports as named exports.
                {
                    find: /^react-icons\/(.+)$/,
                    replacement: 'react-icons/$1/index.js',
                },
                // We generate corresponding .gql.ts files for .gql files.
                // This alias allows us to import .gql files and have them resolved to the generated .gql.ts files.
                {
                    find: /^(.*)\.gql$/,
                    replacement: '$1.gql.ts',
                },
                // Without aliasing lodash to lodash-es we get the following error:
                // SyntaxError: Named export 'castArray' not found. The requested module 'lodash' is a CommonJS module, which may not support all module.exports as named exports.
                {
                    find: /^lodash$/,
                    replacement: 'lodash-es',
                },
            ],
        },

        optimizeDeps: {
            exclude: [
                // Without addings this Vite throws an error
                'linguist-languages',
            ],
        },

        test: {
            setupFiles: './src/testing/setup.ts',
            include: ['src/**/*.test.ts'],
        },

        legacy: {
            // Our existing codebase imports many CommonJS modules as if they were ES modules. The default
            // Vite 5 behavior doesn't work with this. Enabling this should be OK since we don't
            // actually use SSR at the moment, so the difference between the dev and prod builds don't matter.
            // We should revisit this at some point though.
            // See https://vitejs.dev/guide/migration.html#ssr-externalized-modules-value-now-matches-production
            proxySsrExternalModules: true,
        },
    }

    if (BAZEL) {
        // Merge settings necessary to make the build work with bazel
        config = mergeConfig(config, {
            resolve: {
                alias: [
                    // When using Bazel, @sourcegraph/* dependencies will refer to the built packages.
                    // These do not contain the source *.module.scss files but still contain import statements
                    // that reference *.scss files. Processing them with vite throws an error unless we
                    // update the imports to reference the corresponding *.css files instead.
                    // Additionally our own source files might reference *.module.scss files, which we also want
                    // to rewrite.
                    {
                        find: /^(.+)\.module\.scss$/,
                        replacement: '$1.module.css',
                        customResolver(source, importer, options) {
                            // The this.resolve(...) part is taken from the @rollup/plugin-alias implementation. Without
                            // it it appears the bundler tries to resolve relative module IDs to the current working
                            // directory.
                            return source.includes('@sourcegraph') || importer?.includes('@sourcegraph/')
                                ? this.resolve(source, importer, { skipSelf: true, ...options }).then(
                                      resolved => resolved || { id: source }
                                  )
                                : null
                        },
                    },
                ],
            },
            ssr: {
                // By default vite treats dependencies that are links to other packages in the monorepo as source code
                // and processes them as well.
                // In a bazel sandbox however all @sourcegraph/* dependencies are built packages and thus not processed
                // by vite without this additional setting.
                // We have to process those files to apply certain "fixes", such as aliases defined in here
                // and in svelte.config.js.
                noExternal: [/@sourcegraph\/.*/],
                // Exceptions to the above rule. These are packages that are not part of this monorepo and should
                // not be processed by vite.
                external: ['@sourcegraph/telemetry'],
            },
        } satisfies UserConfig)
    }

    return config
})
