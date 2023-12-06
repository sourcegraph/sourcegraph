// @ts-check

const { isScalarType } = require('graphql')

const logger = require('signale')

/**
 *
 * @param {import('graphql').GraphQLSchema} schema
 * @param {import('@graphql-codegen/plugin-helpers').Types.DocumentFile[]} documents
 * @param {{interfaceNameForOperations?: string}} config
 */
const plugin = (schema, documents, config) => {
  const { interfaceNameForOperations = 'TypeMocks' } = config
  schema.getTypeMap()

  const interfaceFields = Object.values(schema.getTypeMap())
    .filter(value => !value.name.startsWith('__'))
    .map(
      value =>
        `${value.name}?: () => ${isScalarType(value) ? `Scalars['${value.name}']` : `DeepPartial<${value.name}>`}\n`
    )

  if (interfaceFields.length === 0) {
    logger.warn('No operations found to generate interface ' + interfaceNameForOperations)
  }
  return [
    '',
    'type DeepPartial<T> = T extends object ? {',
    '    [P in keyof T]?: DeepPartial<T[P]>;',
    '} : T;',
    `export interface ${interfaceNameForOperations} {`,
    '     [key: string]: () => unknown;',
    `    ${interfaceFields.join('    ')}`,
    '}',
  ].join('\n')
}

module.exports = { plugin }
