import { readFileSync } from 'fs'
import path from 'path'

import { type CodegenConfig, generate } from '@graphql-codegen/cli'
import { glob } from 'glob'
import { GraphQLError } from 'graphql'

const ROOT_FOLDER = path.resolve(__dirname, '../../../')

const WEB_FOLDER = path.resolve(ROOT_FOLDER, './client/web')
const SVELTEKIT_FOLDER = path.resolve(ROOT_FOLDER, './client/web-sveltekit')
const BROWSER_FOLDER = path.resolve(ROOT_FOLDER, './client/browser')
const SHARED_FOLDER = path.resolve(ROOT_FOLDER, './client/shared')
const VSCODE_FOLDER = path.resolve(ROOT_FOLDER, './client/vscode')
const JETBRAINS_FOLDER = path.resolve(ROOT_FOLDER, './client/jetbrains')
const SCHEMA_PATH = path.join(ROOT_FOLDER, './cmd/frontend/graphqlbackend/*.graphql')

const SHARED_DOCUMENTS_GLOB = [`${SHARED_FOLDER}/src/**/*.{ts,tsx}`]

const WEB_DOCUMENTS_GLOB = [
    `${WEB_FOLDER}/src/**/*.{ts,tsx}`,
    `${WEB_FOLDER}/src/regression/**/*.*`,
    `!${WEB_FOLDER}/src/end-to-end/**/*.*`, // TODO(bazel): can remove when non-bazel dropped
]

const SVELTEKIT_DOCUMENTS_GLOB = [`${SVELTEKIT_FOLDER}/src/lib/**/*.ts`]

const BROWSER_DOCUMENTS_GLOB = [
    `${BROWSER_FOLDER}/src/**/*.{ts,tsx}`,
    `!${BROWSER_FOLDER}/src/end-to-end/**/*.*`, // TODO(bazel): can remove when non-bazel dropped
    '!**/*.d.ts',
]

const VSCODE_DOCUMENTS_GLOB = [`${VSCODE_FOLDER}/src/**/*.{ts,tsx}`]

const JETBRAINS_DOCUMENTS_GLOB = [`${JETBRAINS_FOLDER}/webview/src/**/*.{ts,tsx}`]

const SHARED_PLUGINS = [
    `${SHARED_FOLDER}/dev/extractGraphQlOperationCodegenPlugin.js`,
    'typescript',
    'typescript-operations',
]

interface Input extends Record<string, unknown> {
    outputPath: string
    globs: string[]
    plugins?: string[]
}

export const ALL_INPUTS: Record<string, Input> = {
    'shared-operations': {
        outputPath: path.join(SHARED_FOLDER, './src/graphql-operations.ts'),
        interfaceNameForOperations: 'SharedGraphQlOperations',
        globs: SHARED_DOCUMENTS_GLOB,
        plugins: ['typescript-apollo-client-helpers'],
        onlyOperationTypes: false,
        enumValues: undefined,
    },
    'shared-types': {
        outputPath: path.join(SHARED_FOLDER, './src/graphql-types.ts'),
        globs: [],
        plugins: [`${SHARED_FOLDER}/dev/extractGraphQlTypesCodegenPlugin.js`],
        onlyOperationTypes: false,
        enumValues: './graphql-operations',
    },
    'browser-operations': {
        outputPath: path.join(BROWSER_FOLDER, './src/graphql-operations.ts'),
        interfaceNameForOperations: 'BrowserGraphQlOperations',
        globs: BROWSER_DOCUMENTS_GLOB,
    },
    'web-operations': {
        outputPath: path.join(WEB_FOLDER, './src/graphql-operations.ts'),
        interfaceNameForOperations: 'WebGraphQlOperations',
        globs: WEB_DOCUMENTS_GLOB,
    },
    'sveltekit-operations': {
        outputPath: path.join(SVELTEKIT_FOLDER, './src/lib/graphql-operations.ts'),
        interfaceNameForOperations: 'SvelteKitGraphQlOperations',
        globs: SVELTEKIT_DOCUMENTS_GLOB,
    },
    'vscode-operations': {
        outputPath: path.join(VSCODE_FOLDER, './src/graphql-operations.ts'),
        interfaceNameForOperations: 'VSCodeGraphQlOperations',
        globs: VSCODE_DOCUMENTS_GLOB,
    },
    'jetbrains-operations': {
        outputPath: path.join(JETBRAINS_FOLDER, './webview/src/graphql-operations.ts'),
        interfaceNameForOperations: 'JetBrainsGraphQlOperations',
        globs: JETBRAINS_DOCUMENTS_GLOB,
    },
}

/**
 * Resolve the globs to files and filter to only files containing "gql`" (which indicates that they
 * contain a GraphQL operation). The @graphql-codegen/typescript plugin does more advanced filtering
 * using an AST parse tree, but this simple string check skips AST parsing and saves a lot of time.
 */
function resolveAndFilterGlobs(globs: string[]): string[] {
    const files = globs.flatMap(p =>
        p.startsWith('!') ? p : glob.sync(p).filter(file => readFileSync(file, 'utf-8').includes('gql`'))
    )
    return files
}

function createCodegenConfig(operations: Input[]): CodegenConfig {
    const generates: CodegenConfig['generates'] = {}
    for (const { outputPath, globs, plugins = [], ...config } of operations) {
        generates[outputPath] = {
            documents: resolveAndFilterGlobs(globs),
            config: {
                onlyOperationTypes: true,
                noExport: false,
                enumValues: '@sourcegraph/shared/src/graphql-operations',
                ...config,
            },
            plugins: [...SHARED_PLUGINS, ...plugins],
        }
    }

    return {
        schema: SCHEMA_PATH,
        errorsOnly: true,
        silent: true,
        config: {
            // https://the-guild.dev/graphql/codegen/plugins/typescript/typescript-operations#config-api-reference
            arrayInputCoercion: false,
            preResolveTypes: true,
            operationResultSuffix: 'Result',
            omitOperationSuffix: true,
            namingConvention: {
                typeNames: 'keep',
                enumValues: 'keep',
                transformUnderscore: true,
            },
            declarationKind: 'interface',
            avoidOptionals: {
                field: true,
                inputValue: false,
                object: true,
            },
            scalars: {
                DateTime: 'string',
                JSON: 'object',
                JSONValue: 'unknown',
                GitObjectID: 'string',
                JSONCString: 'string',
                PublishedValue: "boolean | 'draft'",
                BigInt: 'string',
            },
        },
        generates,
    }
}

if (require.main === module) {
    // Entry point to generate all GraphQL operations files, or a single one.
    async function main(args: string[]): Promise<void> {
        let inputs: Input[] = []
        switch (args.length) {
            case 0: {
                inputs = Object.values(ALL_INPUTS)
                break
            }
            case 2: {
                const configName = args[0]
                if (!(configName in ALL_INPUTS)) {
                    throw new Error(`Unknown config name: ${args[0]}`)
                }
                inputs = [{ ...ALL_INPUTS[configName], outputPath: args[1] }]
                break
            }
            default: {
                throw new Error('Usage: [<configName> <outputPath>]')
            }
        }
        await generate(createCodegenConfig(inputs))
    }
    main(process.argv.slice(2)).catch(error => {
        console.error(error)
        if (error instanceof AggregateError) {
            for (const e of error.errors) {
                if (e instanceof GraphQLError) {
                    console.error(e.source, e.locations)
                }
            }
        }
        process.exit(1)
    })
}
