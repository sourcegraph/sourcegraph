import path from 'path'

import type { StorybookConfigVite } from '@storybook/builder-vite'
import type { StorybookConfig as ReactViteStorybookConfig } from '@storybook/react-vite'
import type { StorybookConfig } from '@storybook/types'
import turbosnap from 'vite-plugin-turbosnap'

import { ROOT_PATH, STATIC_ASSETS_PATH, getEnvironmentBoolean } from '@sourcegraph/build-config'

import { ENVIRONMENT_CONFIG } from './environment-config'

const getStoriesGlob = (): string[] => {
    if (ENVIRONMENT_CONFIG.STORIES_GLOB) {
        return [path.resolve(ROOT_PATH, ENVIRONMENT_CONFIG.STORIES_GLOB)]
    }

    // Due to an issue with constant recompiling (https://github.com/storybookjs/storybook/issues/14342)
    // we need to make the globs more specific (`(web|shared..)` also doesn't work). Once the above issue
    // is fixed, this can be removed and watched for `client/**/*.story.tsx` again.
    const directoriesWithStories = ['branded', 'browser', 'jetbrains/webview', 'shared', 'web', 'wildcard']
    const storiesGlobs = directoriesWithStories.map(packageDirectory =>
        path.resolve(ROOT_PATH, `client/${packageDirectory}/src/**/*.story.tsx`)
    )

    return [...storiesGlobs]
}

const config: StorybookConfig & StorybookConfigVite & ReactViteStorybookConfig = {
    // TODO: This has to be an object and not a string for now due to a bug in Chromatic
    // that would cause the builder to not be identified correctly.
    framework: {
        name: '@storybook/react-vite',
        options: {},
    },
    staticDirs: [path.resolve(__dirname, '../assets'), STATIC_ASSETS_PATH],
    stories: getStoriesGlob(),

    addons: [
        '@storybook/addon-actions',
        '@storybook/addon-designs',
        'storybook-dark-mode',
        '@storybook/addon-a11y',
        '@storybook/addon-toolbars',
        '@storybook/addon-controls',
        '@storybook/addon-storysource',
    ],

    core: {
        disableTelemetry: true,
        builder: '@storybook/builder-vite',
    },

    typescript: {
        check: false,
        reactDocgen: false,
    },

    viteFinal: (config, { configType }) => {
        const isChromatic = getEnvironmentBoolean('CHROMATIC')
        config.define = { ...config.define, 'process.env.CHROMATIC': isChromatic }
        if (isChromatic && configType === 'PRODUCTION') {
            // eslint-disable-next-line no-console
            console.log('Using TurboSnap plugin!')
            config.plugins = config.plugins ?? []
            config.plugins.push(turbosnap({ rootDir: config.root ?? ROOT_PATH }))
        }

        config.build = {
            ...config.build,
            minify: false,

            // HACK(sqs): cssCodeSplit is needed to avoid `Failed to fetch dynamically imported
            // module: ...` errors where SourcegraphWebApp.scss's JavaScript stub file with the CSS
            // module class names is not emitted in the Storybook build. (It works in the dev
            // server.) This is not a perfect workaround as there are some incorrect global styles
            // being applied, but it's mostly fine (and any discrepancies are likely due to our
            // misuse of global CSS anyway).
            cssCodeSplit: false,
        }

        config.css = {
            ...config.css,
            modules: {
                ...config.css?.modules,
                localsConvention: 'camelCase',
            },
            preprocessorOptions: {
                scss: { includePaths: [path.resolve(ROOT_PATH, 'node_modules'), path.resolve(ROOT_PATH, 'client')] },
            },
        }

        config.optimizeDeps = {
            ...config.optimizeDeps,
            exclude: [
                ...(config.optimizeDeps?.exclude || []),
                // Work around an error on the Storybook dev server. See
                // https://github.com/vitejs/vite/issues/4245#issuecomment-879805637.
                'linguist-languages',
            ],
        }

        config.assetsInclude = ['**/*.yaml']
        config.resolve = {
            alias: {
                path: 'path-browserify',
                '@sourcegraph/shared/src/polyfills/vendor/eventSource': path.resolve(
                    __dirname,
                    'dummyEventSourcePolyfill'
                ),
                // msw-storybook-addon conditionally imports msw/native or msw/browser depending on the environment (msw/native is used for React Native).
                // But vite eagerly imports all dependencies, so it tries to load the browser version of msw/native
                // which doesn't exist and causes a build error. This alias is a workaround to avoid this error
                // (the msw/native import is never used in the browser).
                'msw/native': 'msw/browser',
            },
        }

        config.ssr = {
            // Without having vitest/vite process @graphql-tools/* packages, those packages will
            // import the CommonJS version of graphql, whereas we are importing the ES module version.
            // This results in duplicate graphql symbols and errors like "Cannot use GraphQLSchema
            // "[object Object]" from another module or realm."
            noExternal: [/^@graphql-tools\/.*/],
        }

        return config
    },
}

// TODO: We need to replace the @storybook/addon-storysource plugin with an object
// definition to supply options here because chromatic CLI does not properly understand
// the configured addons otherwise.
const idx = config.addons?.findIndex(addon => addon === '@storybook/addon-storysource')
if (idx !== undefined && idx >= 0) {
    config.addons![idx] = {
        name: '@storybook/addon-storysource',
        options: {
            rule: {
                test: /\.story\.tsx?$/,
            },
            sourceLoaderOptions: {
                injectStoryParameters: false,
                prettierConfig: { printWidth: 80, singleQuote: false },
            },
        },
    }
}

module.exports = config
