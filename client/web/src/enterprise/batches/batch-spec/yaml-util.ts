import { escapeRegExp, find, filter } from 'lodash'
import {
    load,
    Kind as YAMLKind,
    type YamlMap as YAMLMap,
    type YAMLMapping,
    type YAMLNode,
    type YAMLSequence,
    type YAMLScalar,
    determineScalarType,
    ScalarType,
} from 'yaml-ast-parser'

const isYAMLMap = (node: YAMLNode): node is YAMLMap => node.kind === YAMLKind.MAP
const isYAMLSequence = (node: YAMLNode): node is YAMLSequence => node.kind === YAMLKind.SEQ
const isYAMLScalar = (node: YAMLNode): node is YAMLScalar => node.kind === YAMLKind.SCALAR

/**
 * A successful result from manipulating the raw batch spec YAML from its AST, even if the
 * manipulation was a no-op
 */
interface YAMLManipulationSuccess {
    success: true
    spec: string
}

/**
 * An unsuccessful result from manipulating the raw batch spec YAML from its AST
 */
interface YAMLManipulationFailure {
    success: false
    error: string
    spec: string
}

type YAMLManipulationResult = YAMLManipulationSuccess | YAMLManipulationFailure

/**
 * Searches the given batch spec YAML AST for a valid "repositoriesMatchingQuery"
 * directive and, if found, adds "-repo:<repo>" to the query string in order to exclude
 * the provided repo from the batch spec workspace results
 *
 * @param spec the raw batch spec YAML string
 * @param ast the batch spec YAML loaded as an AST root node, which should be a `YAMLMap`
 * @param repo the name of the repository to exclude from the batch spec
 * "repositoriesMatchingQuery"
 */
const appendExcludeRepoToQuery = (spec: string, ast: YAMLMap, repo: string): YAMLManipulationResult => {
    // Find the `YAMLMapping` node with the key "on"
    const onMapping = find(ast.mappings, mapping => mapping.key.value === 'on')
    // Take the sequence of values for the "on" key
    const onSequence = onMapping?.value

    if (!onSequence || !isYAMLSequence(onSequence)) {
        return { spec, success: false, error: 'Non-sequence value found for "on" key' }
    }

    // From the sequence, look for a `YAMLMap` node that contains a `YAMLMapping` node
    // with the key "repositoriesMatchingQuery"
    const queryMap: YAMLNode | undefined = find(
        onSequence.items,
        item => isYAMLMap(item) && !!find(item.mappings, mapping => mapping.key.value === 'repositoriesMatchingQuery')
    )

    if (!queryMap || !isYAMLMap(queryMap)) {
        // This just means there wasn't a "repositoriesMatchingQuery" node in the "on"
        // sequence, so return the unaltered spec
        return { spec, success: true }
    }

    // Extract the `YAMLMapping` node with the key "repositoriesMatchingQuery"
    const queryMapping = find(queryMap.mappings, mapping => mapping.key.value === 'repositoriesMatchingQuery')
    // Take the value for the "repositoriesMatchingQuery" key -- this should be a
    // `YAMLScalar` for the query search string
    const queryValue = queryMapping?.value

    if (!queryValue || !isYAMLScalar(queryValue)) {
        return { spec, success: false, error: 'Non-scalar value found for "repositoriesMatchingQuery" key' }
    }

    const isQuoted = queryValue.doubleQuoted || queryValue.singleQuoted || false

    // If the value is not quoted, we need to escape characters
    const excludeQualifierString = isQuoted ? ` -repo:^${repo}$` : ` -repo:^${escapeRegExp(repo)}$`
    // If the value is quoted, we also need to shift the slice position so that the string
    // is inserted inside of the quotes
    const slicePosition = isQuoted ? queryValue.endPosition - 1 : queryValue.endPosition

    // Insert "-repo:" qualifier at the end of the query string
    // TODO: In the future this can be integrated into the batch spec under its own
    // "excludes" keyword instead.
    const newSpec = spec.slice(0, slicePosition) + excludeQualifierString + spec.slice(slicePosition)

    return { success: true, spec: newSpec }
}

/**
 * Searches the given batch spec YAML AST for any valid "repository" directives that match
 * the provided repo name (and branch name, if applicable) and, if found, removes the
 * directive
 *
 * @param spec the raw batch spec YAML string
 * @param ast the batch spec YAML loaded as an AST root node, which should be a `YAMLMap`
 * @param repo the name of the repository to omit the "repository" directive for
 * @param branch the name of the repository branch to omit the "repository" directive for
 */
const removeRepoDirective = (spec: string, ast: YAMLMap, repo: string, branch: string): YAMLManipulationResult => {
    // Find the `YAMLMapping` node with the key "on"
    const onMapping = find(ast.mappings, mapping => mapping.key.value === 'on')
    // Take the sequence of values for the "on" key
    const onSequence = onMapping?.value

    if (!onSequence || !isYAMLSequence(onSequence)) {
        return { spec, success: false, error: 'Non-sequence value found for "on" key' }
    }

    // From the sequence, filter to any `YAMLMap` nodes that contain a `YAMLMapping` node
    // with the key "repository" and a `YAMLScalar` value whose value matches the repo
    // name (there may be none, one, or multiple `YAMLMap`s for different branches)
    const repositoryMatchMaps: YAMLMap[] = filter(
        onSequence.items,
        (item): item is YAMLMap =>
            isYAMLMap(item) &&
            !!find(
                item.mappings,
                mapping =>
                    mapping.key.value === 'repository' &&
                    isYAMLScalar(mapping.value) &&
                    // Compare the values case-insensitively
                    mapping.value.value.toLowerCase() === repo.toLowerCase()
            )
    )

    if (repositoryMatchMaps.length === 0) {
        // This just means there wasn't a matching "repository" directive in the "on"
        // sequence, so return the unaltered spec
        return { spec, success: true }
    }

    // If there's only one matching `YAMLMap` node, we can just remove it from the spec
    if (repositoryMatchMaps.length === 1) {
        const repositoryMatchMap = repositoryMatchMaps[0]
        return {
            success: true,
            spec:
                // NOTE: We also need to trim the sequence delimiter, which is not
                // included in the `YAMLMap`'s start position to end position range
                trimLastSequenceItemDelimiter(spec.slice(0, repositoryMatchMap.startPosition)) +
                spec.slice(repositoryMatchMap.endPosition),
        }
    }

    // Otherwise, if there are multiple matches, look for one that contains a
    // `YAMLMapping` node with the key "branch" and a `YAMLScalar` value whose value
    // matches the branch argument name
    const branchMatchMap: YAMLMap | undefined = find(
        repositoryMatchMaps,
        map =>
            !!find(
                map.mappings,
                mapping =>
                    mapping.key.value === 'branch' &&
                    isYAMLScalar(mapping.value) &&
                    // Compare the values case-insensitively
                    mapping.value.value.toLowerCase() === branch.toLowerCase()
            )
    )

    // If we found no branch match
    if (!branchMatchMap) {
        // This just means none of the matching "repository" directives also matched in
        // the "branch" specified, so return the unaltered spec
        return { spec, success: true }
    }

    // Otherwise, remove the matching `YAMLMap` node from the spec
    return {
        success: true,
        spec:
            // NOTE: We also need to trim the sequence delimiter, which is not included in
            // the `YAMLMap`'s start position to end position range
            trimLastSequenceItemDelimiter(spec.slice(0, branchMatchMap.startPosition)) +
            spec.slice(branchMatchMap.endPosition),
    }
}

/**
 * Trims the final sequence delimiter (i.e. a set of newlines, spaces, and a dash) from
 * the given slice of raw batch spec.
 *
 * This is "sorry-pls-don't-hate-me"-level hack but unfortunately the easiest way around a
 * limitation of working with the YAML AST. The YAML AST parser will not include these
 * characters itself in the character "range" for a node, i.e. they will be present an
 * indeterminate number of characters before `node.startPosition`. So removing a node from
 * a sequence in the spec without also invoking this helper would leave an "empty"
 * sequence item behind and result in parsing errors, like:
 *
 * ```yaml
 * on:
 *   - repository: github.com/sourcegraph/sourcegraph
 *   -
 *   - repository: github.com/sourcegraph/about
 * ```
 *
 * @param specSlice the slice of a raw batch spec YAML string from the beginning up to and
 * including the last set of sequence delimiter characters (one or more newlines followed
 * by zero or more spaces, then a single dash, and then zero or more spaces)
 */
const trimLastSequenceItemDelimiter = (specSlice: string): string =>
    // Trim the last instance of one or more newlines, zero or more spaces, a single dash,
    // and then zero or more spaces, e.g. "\n  - "
    specSlice.replace(/\n+\s*-\s*$/, '')

/**
 * Modifies the provided raw batch spec YAML string in order to exclude a repo resolved in
 * the workspaces preview from the "repositoriesMatchingQuery" value and remove any single
 * "repository" directive that matches the repo name (and branch name, if applicable).
 *
 * @param spec the raw batch spec YAML string
 * @param repo the name of the repository to omit from the batch spec
 * @param branch the name of the repository branch to match when omitting from the batch
 * spec
 */
export const excludeRepo = (spec: string, repo: string, branch: string): YAMLManipulationResult => {
    let ast = load(spec)

    if (!isYAMLMap(ast) || ast.errors.length > 0) {
        return { spec, success: false, error: 'Spec not parseable' }
    }

    // First, try to update the "repositoriesMatchingQuery" string with "-repo:<repo>"
    const appendToQueryResult = appendExcludeRepoToQuery(spec, ast, repo)

    if (!appendToQueryResult.success) {
        return appendToQueryResult
    }

    // Re-parse the AST from the updated result
    ast = load(appendToQueryResult.spec)

    if (!isYAMLMap(ast) || ast.errors.length > 0) {
        return { spec, success: false, error: 'Could not parse spec after updating "repositoriesMatchingQuery"' }
    }

    // Then, also update in case we need to remove any single repository directives that
    // match the repo and branch name
    const removeRepoResult = removeRepoDirective(appendToQueryResult.spec, ast, repo, branch)

    return removeRepoResult
}

/**
 * Finds and returns the value for a node within the `ast` mappings whose key is "on".
 *
 * @param ast the `YAMLMap` node to scan
 */
const findOnStatement = (ast: YAMLMap): YAMLNode | undefined => {
    // Find the `YAMLMapping` node with the key "on"
    const onMapping = find(ast.mappings, mapping => mapping.key.value === 'on')
    return onMapping?.value
}

/**
 * Finds and returns the value for a node within the `ast` mappings whose key is
 * "importChangesets".
 *
 * @param ast the `YAMLMap` node to scan
 */
const findImportChangesetsStatement = (ast: YAMLMap): YAMLNode | undefined => {
    // Find the `YAMLMapping` node with the key "importChangesets"
    const importMapping = find(ast.mappings, mapping => mapping.key.value === 'importChangesets')
    return importMapping?.value
}

/**
 * Finds and returns the value for a node within the `ast` mappings whose key is
 * "workspaces".
 *
 * @param ast the `YAMLMap` node to scan
 */
const findWorkspacesStatement = (ast: YAMLMap): YAMLNode | undefined => {
    // Find the `YAMLMapping` node with the key "workspaces"
    const workspacesMapping = find(ast.mappings, mapping => mapping.key.value === 'workspaces')
    return workspacesMapping?.value
}

/**
 * Checks for a valid "on: " sequence within the provided YAML AST parsed from the input
 * batch spec.
 *
 * @param ast the `YAMLMap` node parsed from the input batch spec
 */
const hasOnStatement = (ast: YAMLMap): boolean => {
    // Find the sequence of values for the key "on"
    const onSequence = findOnStatement(ast)
    return Boolean(onSequence && isYAMLSequence(onSequence) && onSequence.items.length > 0)
}

/**
 * Checks for a valid "importChangesets: " sequence within the provided YAML AST parsed
 * from the input batch spec.
 *
 * @param ast the `YAMLMap` node parsed from the input batch spec
 */
const hasImportChangesetsStatement = (ast: YAMLMap): boolean => {
    // Find the `YAMLMapping` node with the key "importing changesets"
    const importMapping = find(ast.mappings, mapping => mapping.key.value === 'importChangesets')
    // Take the sequence of values for the "importChangesets" key
    const importSequence = importMapping?.value

    return Boolean(importSequence && isYAMLSequence(importSequence) && importSequence.items.length > 0)
}

/**
 * Checks for a valid "on" or "importChangesets: " sequence within the provided raw batch
 * spec YAML string.
 *
 * @param spec the raw batch spec YAML string
 */
export const hasOnOrImportChangesetsStatement = (spec: string): boolean => {
    const ast = load(spec)

    if (!isYAMLMap(ast) || ast.errors.length > 0) {
        return false
    }

    return hasOnStatement(ast) || hasImportChangesetsStatement(ast)
}

/**
 * Checks whether or not the provided raw batch spec YAML string is a minimal batch spec
 * (i.e. the type auto-created for a brand new draft batch change) or something that the
 * user has touched. If the spec is not parseable, as it might be for an in-progress draft
 * batch spec, this function will return `false`.
 *
 * @param spec the raw batch spec YAML string to check
 */
export const isMinimalBatchSpec = (spec: string): boolean => {
    const ast = load(spec)

    if (!isYAMLMap(ast) || ast.errors.length > 0) {
        return false
    }

    return ast.mappings.length === 1 && ast.mappings[0].key.value === 'name'
}

/**
 * Inspects a given string value and determines if it needs to be quoted to produce valid yaml.
 *
 * @param value the string value to inspect
 */
export function quoteYAMLString(value: string): string {
    let needsQuotes = false
    let needsEscaping = false

    // First we need to craft an AST where the value is the value to a key in an object.
    let ast = load('name: ' + value + '\n')

    // If that is not parseable, we might need quotes. Try that.
    if (!isYAMLMap(ast) || ast.errors.length > 0) {
        ast = load('name: "' + value + '"\n')
        needsQuotes = true

        // We don't bail out here, we assume some characters in the value needs escaping,
        // we set the value of `needsEscaping` to true.
        // Also, check if there are more than one mapping. If there are, then the string value has a colon in it and an
        // unbalanced quote - for example, "name": "value
        // The unbalanced quote tricks the second load to think it is valid YAML since the value is closed with the
        // ending quote.
        if (!isYAMLMap(ast) || ast.errors.length > 0 || ast.mappings.length > 1) {
            needsQuotes = false
            needsEscaping = true
        }
    }

    // Then we traverse the AST to find the name key, so we can get the YAMLValue.
    const nameMapping = find(ast.mappings, mapping => mapping.key.value === 'name') as YAMLMapping
    if (!nameMapping || !isYAMLScalar(nameMapping.value)) {
        return value
    }

    // For that value, we let the parser determine the type. If the type is not string,
    // we also want to quote the value.
    const type = determineScalarType(nameMapping.value)
    if (type !== ScalarType.string) {
        needsQuotes = true
    }

    if (needsQuotes) {
        return `"${value}"`
    }

    if (needsEscaping) {
        // to properly escape the characters, we pass the raw string value into `JSON.stringify`,
        // we use the raw string because we don't want to ignore special characters in regular expressions.
        const updatedValue = JSON.stringify(String.raw`${value}`)
        ast = load('name: ' + updatedValue + '\n')

        // if there are no errors then we assume double quoting and escaping special characters works.
        if (ast.errors.length === 0) {
            return updatedValue
        }
    }

    return value
}

/**
 * Replaces the <key> value of the provided `librarySpec` with the provided `value`. If
 * `librarySpec` or its <key> is not properly parsable, just returns the original
 * `librarySpec`.
 *
 * @param librarySpec the raw batch spec YAML example code from a library spec
 * @param value the value of the field to be updated
 * @param key the name of the field in the spec to be updated
 * @param quotable indicates if the value can be quoted or not
 * @param commentExistingValue indicates whether the existing value should be commented instead of removed
 */
export const insertFieldIntoLibraryItem = (
    librarySpec: string,
    value: string,
    key: string,
    quotable: boolean,
    commentExistingValue: boolean = false
): string => {
    const ast = load(librarySpec)
    let existingValue = ''

    if (!isYAMLMap(ast) || ast.errors.length > 0) {
        return librarySpec
    }

    // Find the `YAMLMapping` node with <key>..
    const fieldMapping = find(ast.mappings, mapping => mapping.key.value === key)

    if (!fieldMapping) {
        return librarySpec
    }

    const finalValue = quotable ? quoteYAMLString(value) : value

    if (commentExistingValue) {
        /**
         * We get a slice of the spec containing the fields we want to replace. By doing this we can have a copy of the old value.
         * We then split by new line so we can check each line and comment it out, we want to also make sure we don't comment out
         * existing comments, so we check if each line starts with "#".
         */
        const existingValueArray = librarySpec
            .slice(fieldMapping.value.startPosition, fieldMapping.value.endPosition)
            .split('\n')
            .map(line => (line === '' || line.startsWith('#') ? line : `# ${line}`))

        existingValue = existingValueArray.join('\n')
        // If the existing value contains a trailing new line, we also want to add that in.
        if (existingValueArray.at(-1) === '') {
            existingValue = existingValue.trim()
            existingValue = existingValue + '\n'
        }
    }

    // Stitch the new <value> into the spec.
    return (
        librarySpec.slice(0, fieldMapping.value.startPosition) +
        finalValue +
        existingValue +
        librarySpec.slice(fieldMapping.value.endPosition)
    )
}

/**
 * Replaces the name of the provided `librarySpec`. If `librarySpec` or its name
 * is not properly parsable, just returns the original `librarySpec`.
 *
 * @param librarySpec the raw batch spec YAML example code from a library spec
 * @param name the name of the batch change to be inserted
 */
export const insertNameIntoLibraryItem = (librarySpec: string, name: string): string =>
    insertFieldIntoLibraryItem(librarySpec, name, 'name', true)

/**
 * Replaces the query of the provided `librarySpec`. If `librarySpec` or its query
 * is not properly parsable, just returns the original `librarySpec`.
 *
 * @param librarySpec the raw batch spec YAML example code from a library spec
 * @param query the updated query to be inserted
 * @param commentExistingQuery indicates whether the existing query should be commented instead of deleted
 */
export const insertQueryIntoLibraryItem = (
    librarySpec: string,
    query: string,
    commentExistingQuery: boolean
): string => {
    // we pass in a key of `repositoriesMatchingQuery` into quoteYAMLString because we want to simplify
    // the operation for quoting a YAML String. Passing in a YAMLSequence adds an unnecessary overhead,
    // since we are concerned with quoting the value, passing in a normal string works just fine.
    const possiblyQuotedQuery = quoteYAMLString(query)
    return insertFieldIntoLibraryItem(
        librarySpec,
        `- repositoriesMatchingQuery: ${possiblyQuotedQuery}\n`,
        'on',
        false,
        commentExistingQuery
    )
}

/**
 * Parses and performs a comparison between the values for the "on", "importChangesets",
 * and "workspaces" statements of two different batch specs, returning true if the
 * statements match or "UNKNOWN" if the specs are not able to be parsed and compared.
 *
 * TODO: These checks will eventually move to the backend when we persist the results of a
 * preview from one batch spec to another.
 *
 * @param spec1 the first raw batch spec YAML code to compare against `spec2`
 * @param spec2 the second raw batch spec YAML code to compare against `spec1`
 */
export const haveMatchingWorkspaces = (spec1: string, spec2: string): boolean | 'UNKNOWN' => {
    const ast1 = load(spec1)
    const ast2 = load(spec2)

    if (!isYAMLMap(ast1) || ast1.errors.length > 0 || !isYAMLMap(ast2) || ast2.errors.length > 0) {
        return 'UNKNOWN'
    }

    // Find the value for the "on" statement for both specs
    const on1 = findOnStatement(ast1)
    const on2 = findOnStatement(ast2)

    if (!on1 || !on2) {
        return 'UNKNOWN'
    }

    const onString1 = spec1.slice(on1.startPosition, on1.endPosition)
    const onString2 = spec2.slice(on2.startPosition, on2.endPosition)

    if (onString1 !== onString2) {
        return false
    }

    // Find the value for the "importChangesets" statement for both specs
    const import1 = findImportChangesetsStatement(ast1)
    const import2 = findImportChangesetsStatement(ast2)

    if ((import1 && !import2) || (!import1 && import2)) {
        return false
    }

    if (import1 && import2) {
        const importString1 = spec1.slice(import1.startPosition, import1.endPosition)
        const importString2 = spec2.slice(import2.startPosition, import2.endPosition)

        if (importString1 !== importString2) {
            return false
        }
    }

    // Find the value for the "workspaces" statement for both specs
    const workspaces1 = findWorkspacesStatement(ast1)
    const workspaces2 = findWorkspacesStatement(ast2)

    if ((workspaces1 && !workspaces2) || (!workspaces1 && workspaces2)) {
        return false
    }

    if (!workspaces1 || !workspaces2) {
        return true
    }

    const workspacesString1 = spec1.slice(workspaces1.startPosition, workspaces1.endPosition)
    const workspacesString2 = spec2.slice(workspaces2.startPosition, workspaces2.endPosition)

    return workspacesString1 === workspacesString2
}
