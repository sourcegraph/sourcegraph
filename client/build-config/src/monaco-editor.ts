import type MonacoWebpackPlugin from 'monaco-editor-webpack-plugin'

/**
 * Configuration for https://github.com/microsoft/monaco-editor-webpack-plugin.
 */
export const MONACO_LANGUAGES_AND_FEATURES: Required<
    Pick<
        NonNullable<ConstructorParameters<typeof MonacoWebpackPlugin>[0]>,
        'languages' | 'features' | 'customLanguages'
    >
> = {
    languages: ['json', 'yaml', 'lua'],
    customLanguages: [
        {
            label: 'yaml',
            entry: require.resolve('monaco-yaml/lib/esm/monaco.contribution'),
            worker: { id: 'vs/language/yaml/yamlWorker', entry: require.resolve('monaco-yaml/lib/esm/yaml.worker') },
        },
    ],
    features: [
        'bracketMatching',
        'clipboard',
        'coreCommands',
        'cursorUndo',
        'find',
        'format',
        'gotoError', // needed for warning and error squiggles
        'hover',
        'inPlaceReplace',
        'iPadShowKeyboard',
        'links',
        'suggest',
    ],
}
