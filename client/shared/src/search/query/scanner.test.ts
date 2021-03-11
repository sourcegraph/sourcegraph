import { scanSearchQuery, scanBalancedLiteral, toPatternResult } from './scanner'
import { PatternKind } from './token'
import { SearchPatternType } from '../../graphql-operations'

expect.addSnapshotSerializer({
    serialize: value => JSON.stringify(value),
    test: () => true,
})

describe('scanBalancedPattern()', () => {
    const scanBalancedPattern = toPatternResult(scanBalancedLiteral, PatternKind.Literal)
    test('balanced, scans up to whitespace', () => {
        expect(scanBalancedPattern('foo OR bar', 0)).toMatchInlineSnapshot(
            '{"type":"success","term":{"type":"pattern","range":{"start":0,"end":3},"kind":1,"value":"foo"}}'
        )
    })

    test('balanced, consumes spaces', () => {
        expect(scanBalancedPattern('(hello there)', 0)).toMatchInlineSnapshot(
            '{"type":"success","term":{"type":"pattern","range":{"start":0,"end":13},"kind":1,"value":"(hello there)"}}'
        )
    })

    test('balanced, consumes unrecognized filter-like value', () => {
        expect(scanBalancedPattern('( general:kenobi )', 0)).toMatchInlineSnapshot(
            '{"type":"success","term":{"type":"pattern","range":{"start":0,"end":18},"kind":1,"value":"( general:kenobi )"}}'
        )
    })

    test('not recognized, contains not keyword', () => {
        expect(scanBalancedPattern('(foo not bar)', 0)).toMatchInlineSnapshot(
            '{"type":"error","expected":"no recognized filter or keyword","at":5}'
        )
    })

    test('not recognized, starts with a not keyword', () => {
        expect(scanBalancedPattern('(not chocolate)', 0)).toMatchInlineSnapshot(
            '{"type":"error","expected":"no recognized filter or keyword","at":1}'
        )
    })

    test('not recognized, contains an or keyword', () => {
        expect(scanBalancedPattern('(foo OR bar)', 0)).toMatchInlineSnapshot(
            '{"type":"error","expected":"no recognized filter or keyword","at":5}'
        )
    })

    test('not recognized, contains an and keyword', () => {
        expect(scanBalancedPattern('repo:foo AND bar', 0)).toMatchInlineSnapshot(
            '{"type":"error","expected":"no recognized filter or keyword","at":0}'
        )
    })

    test('not recognized, contains a recognized repo field', () => {
        expect(scanBalancedPattern('repo:foo bar', 0)).toMatchInlineSnapshot(
            '{"type":"error","expected":"no recognized filter or keyword","at":0}'
        )
    })

    test('balanced, no conflicting tokens', () => {
        expect(scanBalancedPattern('(bor band )', 0)).toMatchInlineSnapshot(
            '{"type":"success","term":{"type":"pattern","range":{"start":0,"end":11},"kind":1,"value":"(bor band )"}}'
        )
    })

    test('not recognized, unbalanced', () => {
        expect(scanBalancedPattern('foo(', 0)).toMatchInlineSnapshot(
            '{"type":"error","expected":"no unbalanced parentheses","at":4}'
        )
    })
})

describe('scanSearchQuery() for literal search', () => {
    test('empty', () => expect(scanSearchQuery('')).toMatchInlineSnapshot('{"type":"success","term":[]}'))

    test('whitespace', () =>
        expect(scanSearchQuery('  ')).toMatchInlineSnapshot(
            '{"type":"success","term":[{"type":"whitespace","range":{"start":0,"end":2}}]}'
        ))

    test('literal', () =>
        expect(scanSearchQuery('a')).toMatchInlineSnapshot(
            '{"type":"success","term":[{"type":"pattern","range":{"start":0,"end":1},"kind":1,"value":"a"}]}'
        ))

    test('triple quotes', () => {
        expect(scanSearchQuery('"""')).toMatchInlineSnapshot(
            '{"type":"success","term":[{"type":"pattern","range":{"start":0,"end":3},"kind":1,"value":"\\"\\"\\""}]}'
        )
    })

    test('filter', () =>
        expect(scanSearchQuery('f:b')).toMatchInlineSnapshot(
            '{"type":"success","term":[{"type":"filter","range":{"start":0,"end":3},"field":{"type":"literal","value":"f","range":{"start":0,"end":1}},"value":{"type":"literal","value":"b","range":{"start":2,"end":3},"quoted":false},"negated":false}]}'
        ))

    test('negated filter', () =>
        expect(scanSearchQuery('-f:b')).toMatchInlineSnapshot(
            '{"type":"success","term":[{"type":"filter","range":{"start":0,"end":4},"field":{"type":"literal","value":"-f","range":{"start":0,"end":2}},"value":{"type":"literal","value":"b","range":{"start":3,"end":4},"quoted":false},"negated":true}]}'
        ))

    test('filter with quoted value', () => {
        expect(scanSearchQuery('f:"b"')).toMatchInlineSnapshot(
            '{"type":"success","term":[{"type":"filter","range":{"start":0,"end":5},"field":{"type":"literal","value":"f","range":{"start":0,"end":1}},"value":{"type":"literal","value":"b","range":{"start":2,"end":5},"quoted":true},"negated":false}]}'
        )
    })

    test('filter with a value ending with a colon', () => {
        expect(scanSearchQuery('f:a:')).toMatchInlineSnapshot(
            '{"type":"success","term":[{"type":"filter","range":{"start":0,"end":4},"field":{"type":"literal","value":"f","range":{"start":0,"end":1}},"value":{"type":"literal","value":"a:","range":{"start":2,"end":4},"quoted":false},"negated":false}]}'
        )
    })

    test('filter where the value is a colon', () => {
        expect(scanSearchQuery('f:a:')).toMatchInlineSnapshot(
            '{"type":"success","term":[{"type":"filter","range":{"start":0,"end":4},"field":{"type":"literal","value":"f","range":{"start":0,"end":1}},"value":{"type":"literal","value":"a:","range":{"start":2,"end":4},"quoted":false},"negated":false}]}'
        )
    })

    test('quoted, double quotes', () =>
        expect(scanSearchQuery('"a:b"')).toMatchInlineSnapshot(
            '{"type":"success","term":[{"type":"literal","value":"a:b","range":{"start":0,"end":5},"quoted":true}]}'
        ))

    test('quoted, single quotes', () =>
        expect(scanSearchQuery("'a:b'")).toMatchInlineSnapshot(
            '{"type":"success","term":[{"type":"literal","value":"a:b","range":{"start":0,"end":5},"quoted":true}]}'
        ))

    test('quoted (escaped quotes)', () =>
        expect(scanSearchQuery('"-\\"a\\":b"')).toMatchInlineSnapshot(
            '{"type":"success","term":[{"type":"literal","value":"-\\\\\\"a\\\\\\":b","range":{"start":0,"end":10},"quoted":true}]}'
        ))

    test('complex query', () =>
        expect(scanSearchQuery('repo:^github\\.com/gorilla/mux$ lang:go -file:mux.go Router')).toMatchInlineSnapshot(
            '{"type":"success","term":[{"type":"filter","range":{"start":0,"end":30},"field":{"type":"literal","value":"repo","range":{"start":0,"end":4}},"value":{"type":"literal","value":"^github\\\\.com/gorilla/mux$","range":{"start":5,"end":30},"quoted":false},"negated":false},{"type":"whitespace","range":{"start":30,"end":31}},{"type":"filter","range":{"start":31,"end":38},"field":{"type":"literal","value":"lang","range":{"start":31,"end":35}},"value":{"type":"literal","value":"go","range":{"start":36,"end":38},"quoted":false},"negated":false},{"type":"whitespace","range":{"start":38,"end":39}},{"type":"filter","range":{"start":39,"end":51},"field":{"type":"literal","value":"-file","range":{"start":39,"end":44}},"value":{"type":"literal","value":"mux.go","range":{"start":45,"end":51},"quoted":false},"negated":true},{"type":"whitespace","range":{"start":51,"end":52}},{"type":"pattern","range":{"start":52,"end":58},"kind":1,"value":"Router"}]}'
        ))

    test('parenthesized parameters', () => {
        expect(scanSearchQuery('repo:a (file:b and c)')).toMatchInlineSnapshot(
            '{"type":"success","term":[{"type":"filter","range":{"start":0,"end":6},"field":{"type":"literal","value":"repo","range":{"start":0,"end":4}},"value":{"type":"literal","value":"a","range":{"start":5,"end":6},"quoted":false},"negated":false},{"type":"whitespace","range":{"start":6,"end":7}},{"type":"openingParen","range":{"start":7,"end":8}},{"type":"filter","range":{"start":8,"end":14},"field":{"type":"literal","value":"file","range":{"start":8,"end":12}},"value":{"type":"literal","value":"b","range":{"start":13,"end":14},"quoted":false},"negated":false},{"type":"whitespace","range":{"start":14,"end":15}},{"type":"keyword","value":"and","range":{"start":15,"end":18},"kind":"and"},{"type":"whitespace","range":{"start":18,"end":19}},{"type":"pattern","range":{"start":19,"end":20},"kind":1,"value":"c"},{"type":"closingParen","range":{"start":20,"end":21}}]}'
        )
    })

    test('nested parenthesized parameters', () => {
        expect(scanSearchQuery('(a and (b or c) and d)')).toMatchInlineSnapshot(
            '{"type":"success","term":[{"type":"openingParen","range":{"start":0,"end":1}},{"type":"pattern","range":{"start":1,"end":2},"kind":1,"value":"a"},{"type":"whitespace","range":{"start":2,"end":3}},{"type":"keyword","value":"and","range":{"start":3,"end":6},"kind":"and"},{"type":"whitespace","range":{"start":6,"end":7}},{"type":"openingParen","range":{"start":7,"end":8}},{"type":"pattern","range":{"start":8,"end":9},"kind":1,"value":"b"},{"type":"whitespace","range":{"start":9,"end":10}},{"type":"keyword","value":"or","range":{"start":10,"end":12},"kind":"or"},{"type":"whitespace","range":{"start":12,"end":13}},{"type":"pattern","range":{"start":13,"end":14},"kind":1,"value":"c"},{"type":"closingParen","range":{"start":14,"end":15}},{"type":"whitespace","range":{"start":15,"end":16}},{"type":"keyword","value":"and","range":{"start":16,"end":19},"kind":"and"},{"type":"whitespace","range":{"start":19,"end":20}},{"type":"pattern","range":{"start":20,"end":21},"kind":1,"value":"d"},{"type":"closingParen","range":{"start":21,"end":22}}]}'
        )
    })

    test('do not treat links as filters', () => {
        expect(scanSearchQuery('http://example.com repo:a')).toMatchInlineSnapshot(
            '{"type":"success","term":[{"type":"pattern","range":{"start":0,"end":18},"kind":1,"value":"http://example.com"},{"type":"whitespace","range":{"start":18,"end":19}},{"type":"filter","range":{"start":19,"end":25},"field":{"type":"literal","value":"repo","range":{"start":19,"end":23}},"value":{"type":"literal","value":"a","range":{"start":24,"end":25},"quoted":false},"negated":false}]}'
        )
    })
})

describe('scanSearchQuery() for regexp', () => {
    test('interpret regexp pattern with match groups', () => {
        expect(
            scanSearchQuery('((sauce|graph)(\\s)?)is best(g*r*a*p*h*)', false, SearchPatternType.regexp)
        ).toMatchInlineSnapshot(
            '{"type":"success","term":[{"type":"pattern","range":{"start":0,"end":22},"kind":2,"value":"((sauce|graph)(\\\\s)?)is"},{"type":"whitespace","range":{"start":22,"end":23}},{"type":"pattern","range":{"start":23,"end":39},"kind":2,"value":"best(g*r*a*p*h*)"}]}'
        )
    })

    test('interpret regexp pattern with match groups between keywords', () => {
        expect(
            scanSearchQuery('(((sauce|graph)\\s?) or (best)) and (gr|aph)', false, SearchPatternType.regexp)
        ).toMatchInlineSnapshot(
            '{"type":"success","term":[{"type":"openingParen","range":{"start":0,"end":1}},{"type":"pattern","range":{"start":1,"end":19},"kind":2,"value":"((sauce|graph)\\\\s?)"},{"type":"whitespace","range":{"start":19,"end":20}},{"type":"keyword","value":"or","range":{"start":20,"end":22},"kind":"or"},{"type":"whitespace","range":{"start":22,"end":23}},{"type":"pattern","range":{"start":23,"end":29},"kind":2,"value":"(best)"},{"type":"closingParen","range":{"start":29,"end":30}},{"type":"whitespace","range":{"start":30,"end":31}},{"type":"keyword","value":"and","range":{"start":31,"end":34},"kind":"and"},{"type":"whitespace","range":{"start":34,"end":35}},{"type":"pattern","range":{"start":35,"end":43},"kind":2,"value":"(gr|aph)"}]}'
        )
    })

    test('interpret regexp slash quotes', () => {
        expect(scanSearchQuery('r:a /a regexp \\ pattern/', false, SearchPatternType.regexp)).toMatchInlineSnapshot(
            '{"type":"success","term":[{"type":"filter","range":{"start":0,"end":3},"field":{"type":"literal","value":"r","range":{"start":0,"end":1}},"value":{"type":"literal","value":"a","range":{"start":2,"end":3},"quoted":false},"negated":false},{"type":"whitespace","range":{"start":3,"end":4}},{"type":"literal","value":"a regexp \\\\ pattern","range":{"start":4,"end":24},"quoted":true}]}'
        )
    })
})

describe('scanSearchQuery() with comments', () => {
    test('interpret C-style comments', () => {
        const query = `// saucegraph is best graph
repo:sourcegraph
// search for thing
thing`
        expect(scanSearchQuery(query, true)).toMatchInlineSnapshot(
            '{"type":"success","term":[{"type":"comment","value":"// saucegraph is best graph","range":{"start":0,"end":27}},{"type":"whitespace","range":{"start":27,"end":28}},{"type":"filter","range":{"start":28,"end":44},"field":{"type":"literal","value":"repo","range":{"start":28,"end":32}},"value":{"type":"literal","value":"sourcegraph","range":{"start":33,"end":44},"quoted":false},"negated":false},{"type":"whitespace","range":{"start":44,"end":45}},{"type":"comment","value":"// search for thing","range":{"start":45,"end":64}},{"type":"whitespace","range":{"start":64,"end":65}},{"type":"pattern","range":{"start":65,"end":70},"kind":1,"value":"thing"}]}'
        )
    })

    test('do not interpret C-style comments', () => {
        expect(scanSearchQuery('// thing')).toMatchInlineSnapshot(
            '{"type":"success","term":[{"type":"pattern","range":{"start":0,"end":2},"kind":1,"value":"//"},{"type":"whitespace","range":{"start":2,"end":3}},{"type":"pattern","range":{"start":3,"end":8},"kind":1,"value":"thing"}]}'
        )
    })
})
