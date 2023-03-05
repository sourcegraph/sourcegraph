import path from 'path'

import * as esbuild from 'esbuild'
// eslint-disable-next-line no-restricted-imports
import { EditorFeature, featuresArr } from 'monaco-editor-webpack-plugin/out/features'
// eslint-disable-next-line no-restricted-imports
import { EditorLanguage, languagesArr } from 'monaco-editor-webpack-plugin/out/languages'

import { ROOT_PATH } from '../paths'
import { MONACO_LANGUAGES_AND_FEATURES } from '../webpack/monaco-editor'

const monacoModulePath = (modulePath: string): string =>
    require.resolve(path.join('monaco-editor/esm', modulePath), {
        paths: [path.join(ROOT_PATH, 'node_modules')],
    })

/**
 * An esbuild plugin that omits some unneeded features and languages from monaco-editor when
 * bundling, to reduce bundle size and speed up builds. Similar to
 * https://github.com/microsoft/monaco-editor-webpack-plugin.
 */
export const monacoPlugin = ({
    languages,
    features,
}: Required<typeof MONACO_LANGUAGES_AND_FEATURES>): esbuild.Plugin => ({
    name: 'monaco',
    setup: build => {
        for (const feature of features) {
            if (feature.startsWith('!')) {
                throw new Error('negated features (starting with "!") are not supported')
            }
        }

        // Some feature exclusions don't work because their module exports a symbol needed by
        // another feature.
        const ALWAYS_ENABLED_FEATURES = new Set<EditorFeature>(['snippets'])

        const skipLanguageModules = languagesArr
            .filter(({ label }) => !languages.includes(label as EditorLanguage))
            .flatMap(({ entry }) => entry || [])
        const skipFeatureModules = featuresArr
            .filter(
                ({ label }) =>
                    !features.includes(label as EditorFeature) && !ALWAYS_ENABLED_FEATURES.has(label as EditorFeature)
            )
            .flatMap(({ entry }) => entry || [])

        const skipModulePaths = [...skipLanguageModules, ...skipFeatureModules].map(monacoModulePath)
        const filter = new RegExp(`^(${skipModulePaths.join('|')})$`)

        // For omitted features and languages, treat their modules as empty files.
        //
        // TODO(sqs): This is different from how
        // https://github.com/microsoft/monaco-editor-webpack-plugin does it. The
        // monaco-editor-webpack-plugin approach relies on injecting a different central module
        // file, rather than zeroing out each feature/language module. Our approach necessitates the
        // ALWAYS_ENABLED_FEATURES hack above. Our approach is fine for when esbuild is still an
        // optional prototype build method for local dev, but this implementation should be fixed if
        // we switch to esbuild by default.
        build.onLoad({ filter }, () => ({ contents: '', loader: 'js' }))
    },
})

// TODO(sqs): These Monaco Web Workers could be built as part of the main build if we switch to
// using MonacoEnvironment#getWorker (from #getWorkerUrl), which would then let us use a worker
// plugin to load these instead of needing to hardcode them as build entrypoints.
export const buildMonaco = async (outdir: string): Promise<esbuild.BuildContext> =>
    esbuild.context({
        entryPoints: {
            'scripts/editor.worker.bundle': 'monaco-editor/esm/vs/editor/editor.worker.js',
            'scripts/json.worker.bundle': 'monaco-editor/esm/vs/language/json/json.worker.js',
        },
        format: 'iife',
        target: 'es2021',
        bundle: true,
        outdir,
    })
