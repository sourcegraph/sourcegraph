// @ts-check
const {
    visit,
    concatAST,
    isObjectType,
    isNonNullType,
    isListType,
    isInterfaceType,
    isUnionType,
    isEnumType,
    isScalarType,
} = require('graphql')
const logger = require('signale')

/**
 * @param {import('@graphql-codegen/plugin-helpers').Types.DocumentFile[]} documents
 * @returns {import('graphql').ASTNode}
 */
function documentsToAST(documents) {
    const documentNodes = []
    for (const document of documents) {
        if (document.document) {
            documentNodes.push(document.document)
        }
    }
    return concatAST(documentNodes)
}

/**
 * @param {import('graphql').GraphQLNamedType} type
 * @returns {string}
 */
function getMockTypeName(type) {
    return type.name + 'Mock'
}

/**
 * @param {string[]} lines
 * @param {string} indent
 * @returns {string}
 */
function formatLines(lines, indent = '') {
    return lines.map(line => indent + line).join('\n')
}

/**
 * @param {import('graphql').GraphQLSchema} schema
 * @param {import('graphql').GraphQLNamedType} type
 * @returns {string}
 */
function generateObjectTypeFields(schema, type, indent = '') {
    if (!isObjectType(type) && !isInterfaceType(type)) {
        throw new Error('Unsupported type ' + type)
    }
    let lines = []
    if (isObjectType(type)) {
        lines.push(`__typename?: '${type.name}',`)
    }
    if (isInterfaceType(type)) {
        lines.push(
            `__typename?: ${schema
                .getImplementations(type)
                .objects.map(type => `'${type.name}'`)
                .join(' | ')},`
        )
    }

    Object.entries(type.getFields()).forEach(([fieldName, field]) => {
        if (field.description || field.deprecationReason) {
            lines.push('/**')
            if (field.description) {
                field.description.split('\n').forEach(line => lines.push(` * ${line}`))
            }
            if (field.deprecationReason) {
                lines.push(` * @deprecated ${field.deprecationReason}`)
            }
            lines.push(' */')
        }
        lines.push(`${fieldName}?: ${generateTSTypeForNullableGraphQLType(field.type, indent)},`)
    })
    return formatLines(lines, indent)
}

/**
 * @param {import('graphql').GraphQLType} type
 * @param {string} indent
 * @returns {string}
 */
function generateTSTypeForNullableGraphQLType(type, indent = '') {
    if (isNonNullType(type)) {
        return generateTSTypeForGraphQLType(type.ofType, indent)
    }
    return generateTSTypeForGraphQLType(type, indent) + ' | null'
}

/**
 * @param {import('graphql').GraphQLType} type
 * @returns {string}
 */
function generateTSTypeForGraphQLType(type, indent = '') {
    if (isListType(type)) {
        // Using Array<...> instead of ...[] to avoid having to wrap some inner types in parentheses
        return `Array<${generateTSTypeForNullableGraphQLType(type.ofType, indent)}>`
    }
    if (isUnionType(type)) {
        return type
            .getTypes()
            .map(type => generateTSTypeForGraphQLType(type, indent))
            .join(' | ')
    }
    if (isObjectType(type) || isInterfaceType(type)) {
        return getMockTypeName(type)
    }
    if (isEnumType(type)) {
        return type
            .getValues()
            .map(value => `'${value.name}'`)
            .join(' | ')
    }
    if (isScalarType(type)) {
        return `Scalars['${type.name}']['output']`
    }
    throw new Error('Unsupported type ' + type)
}

/**
 *
 * @param {import('graphql').GraphQLSchema} schema
 * @param {import('@graphql-codegen/plugin-helpers').Types.DocumentFile[]} documents
 * @param {{mockInterfaceName?: string, operationResultSuffix?: string}} config
 * @returns {import('@graphql-codegen/plugin-helpers').Types.PluginOutput}
 */
const plugin = (schema, documents, config) => {
    const { mockInterfaceName = 'TypeMocks', operationResultSuffix = '' } = config

    const interfaceFields = Object.values(schema.getTypeMap())
        .filter(value => !value.name.startsWith('__') && (isScalarType(value) || isObjectType(value)))
        .map(
            value =>
                `${value.name}?: MockFunction<${
                    isScalarType(value) ? `Scalars['${value.name}']['output']` : getMockTypeName(value)
                }>`
        )
    const objectTypes = Object.values(schema.getTypeMap()).filter(
        value => !value.name.startsWith('__') && (isObjectType(value) || isInterfaceType(value))
    )

    const operations = []
    visit(documentsToAST(documents), {
        OperationDefinition(node) {
            operations.push(node)
        },
    })

    if (interfaceFields.length === 0) {
        logger.warn('No types found to generate interface ' + mockInterfaceName)
    }

    return {
        prepend: [
            `import type { GraphQLResolveInfo } from 'graphql'`,
            'type DeepPartial<T> = T extends object ? {',
            '    [P in keyof T]?: DeepPartial<T[P]>',
            '} : T',
            'type MockFunction<Return> = (info: GraphQLResolveInfo) => Return',
        ],
        content: [
            ...objectTypes.map(
                type =>
                    `export interface ${getMockTypeName(type)} {\n${generateObjectTypeFields(schema, type, '  ')}\n}\n`
            ),
            `export interface ${mockInterfaceName} {`,
            `    [key: string]: MockFunction<any>|undefined`,
            `    ${interfaceFields.join('\n    ')}`,
            '}',
            `export interface OperationMocks {`,
            `    [key: string]: ((variables: any) => DeepPartial<any>)|undefined`,
            `    ${operations
                .map(
                    operation =>
                        `${operation.name.value}?: (variables: ${operation.name.value}Variables) => DeepPartial<${operation.name.value}${operationResultSuffix}>`
                )
                .join('\n    ')}`,
            `}`,
            'export type ObjectMock = ' +
                objectTypes
                    .filter(type => !isInterfaceType(type))
                    .map(type => getMockTypeName(type))
                    .join(' | '),
        ].join('\n'),
    }
}
module.exports = { plugin }
