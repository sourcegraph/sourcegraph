import { selectorCompletion } from './selectFilter'
import { Literal, createLiteral } from './token'

expect.addSnapshotSerializer({
    serialize: (value: string[]): string => value.join(',\n'),
    test: () => true,
})

const create = (value: string): Literal => createLiteral(value, { start: 0, end: 0 })

describe('selectorCompletion', () => {
    test('suggest depth 0 completions', () => {
        expect(selectorCompletion(create('foo'))).toMatchInlineSnapshot(`
            repo,
            file,
            content,
            symbol,
            commit
        `)
    })

    test('suggest depth 1 symbol completions', () => {
        expect(selectorCompletion(create('symbol.f'))).toMatchInlineSnapshot(`
            symbol,
            symbol.file,
            symbol.module,
            symbol.namespace,
            symbol.package,
            symbol.class,
            symbol.method,
            symbol.property,
            symbol.field,
            symbol.constructor,
            symbol.enum,
            symbol.interface,
            symbol.function,
            symbol.variable,
            symbol.constant,
            symbol.string,
            symbol.number,
            symbol.boolean,
            symbol.array,
            symbol.object,
            symbol.key,
            symbol.null,
            symbol.enum-member,
            symbol.struct,
            symbol.event,
            symbol.operator,
            symbol.type-parameter
        `)
    })
})
