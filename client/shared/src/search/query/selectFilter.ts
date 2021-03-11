import { Literal } from './token'

interface Selector {
    kind: string
    fields?: Selector[]
}

export const SELECTORS: Selector[] = [
    {
        kind: 'repo',
    },
    {
        kind: 'file',
    },
    {
        kind: 'content',
    },
    {
        kind: 'symbol',
        fields: [
            { kind: 'file' },
            { kind: 'module' },
            { kind: 'namespace' },
            { kind: 'package' },
            { kind: 'class' },
            { kind: 'method' },
            { kind: 'property' },
            { kind: 'field' },
            { kind: 'constructor' },
            { kind: 'enum' },
            { kind: 'interface' },
            { kind: 'function' },
            { kind: 'variable' },
            { kind: 'constant' },
            { kind: 'string' },
            { kind: 'number' },
            { kind: 'boolean' },
            { kind: 'array' },
            { kind: 'object' },
            { kind: 'key' },
            { kind: 'null' },
            { kind: 'enum-member' },
            { kind: 'struct' },
            { kind: 'event' },
            { kind: 'operator' },
            { kind: 'type-parameter' },
        ],
    },
    {
        kind: 'commit',
    },
]

/**
 * Returns all paths rooted at a {@link selector} up to {@param depth}.
 */
export const selectDiscreteValues = (selectors: Selector[], depth: number): string[] => {
    if (depth < 0) {
        return []
    }
    const paths: string[] = []
    for (const entry of selectors) {
        paths.push(`${entry.kind}`)
        if (entry.fields) {
            paths.push(...selectDiscreteValues(entry.fields, depth - 1).map(value => `${entry.kind}.` + value))
        }
    }
    return paths
}

export const selectorCompletion = (value: Literal | undefined): string[] => {
    if (!value) {
        return selectDiscreteValues(SELECTORS, 0)
    }

    if (value.value.endsWith('.') || value.value.split('.').length > 1) {
        // Resolve completions to greater depth for `foo.` if the value is `foo.` or `foo.bar`.
        const kind = value.value.split('.')[0]
        return selectDiscreteValues(
            SELECTORS.filter(value => value.kind === kind),
            1
        )
    }
    return selectDiscreteValues(SELECTORS, 0)
}
