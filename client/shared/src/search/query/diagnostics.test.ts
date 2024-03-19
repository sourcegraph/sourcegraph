import { describe, expect, test, afterAll } from 'vitest'

import { SearchPatternType } from '../../graphql-operations'

import { getDiagnostics } from './diagnostics'
import { scanSearchQuery, type ScanSuccess, type ScanResult } from './scanner'
import type { Token } from './token'

expect.addSnapshotSerializer({
    serialize: value => JSON.stringify(value, null, 2),
    test: () => true,
})

const toSuccess = (result: ScanResult<Token[]>): Token[] => (result as ScanSuccess<Token[]>).term

function parseAndDiagnose(query: string, searchPattern: SearchPatternType): ReturnType<typeof getDiagnostics> {
    return getDiagnostics(toSuccess(scanSearchQuery(query)), searchPattern)
}

describe('getDiagnostics()', () => {
    describe('empty and invalid filter values', () => {
        test('do not raise invalid filter type', () => {
            expect(parseAndDiagnose('repos:^github.com/sourcegraph', SearchPatternType.standard)).toMatchInlineSnapshot(
                '[]'
            )
        })

        test('invalid filter value', () => {
            expect(parseAndDiagnose('case:maybe', SearchPatternType.standard)).toMatchInlineSnapshot(`
                [
                  {
                    "severity": "error",
                    "message": "Invalid filter value, expected one of: yes, no.",
                    "range": {
                      "start": 0,
                      "end": 10
                    }
                  }
                ]
            `)
        })

        test('search query containing colon, literal pattern type, do not raise error', () => {
            expect(parseAndDiagnose('Configuration::doStuff(...)', SearchPatternType.standard)).toMatchInlineSnapshot(
                '[]'
            )
        })

        test('search query containing quoted token, regexp pattern type', () => {
            expect(parseAndDiagnose('"Configuration::doStuff(...)"', SearchPatternType.regexp)).toMatchInlineSnapshot(
                '[]'
            )
        })

        test('search query containing parenthesized parameterss', () => {
            expect(parseAndDiagnose('repo:a (file:b and c)', SearchPatternType.regexp)).toMatchInlineSnapshot('[]')
        })

        test('search query with empty filter diagnostic', () => {
            expect(parseAndDiagnose('meatadata file: ', SearchPatternType.regexp)).toMatchInlineSnapshot(`
                [
                  {
                    "severity": "warning",
                    "message": "This filter is empty. Remove the space between the filter and value or quote the value to include the space. E.g. \`file:\\" a term\\"\`.",
                    "range": {
                      "start": 10,
                      "end": 15
                    }
                  }
                ]
            `)
        })

        test('search query with both invalid filter and empty filter returns only one diagnostic for the first issue', () => {
            expect(parseAndDiagnose('meatadata type: ', SearchPatternType.regexp)).toMatchInlineSnapshot(`
                [
                  {
                    "severity": "error",
                    "message": "Invalid filter value, expected one of: diff, commit, symbol, repo, path, file.",
                    "range": {
                      "start": 10,
                      "end": 15
                    }
                  }
                ]
            `)
        })
    })

    describe('diff and commit only filters', () => {
        test('detects invalid author/before/after/message filters', () => {
            expect(parseAndDiagnose('author:me', SearchPatternType.standard)).toMatchInlineSnapshot(`
                [
                  {
                    "severity": "error",
                    "message": "This filter requires \`type:commit\` or \`type:diff\` in the query",
                    "range": {
                      "start": 0,
                      "end": 9
                    },
                    "actions": [
                      {
                        "label": "Add \\"type:commit\\"",
                        "change": {
                          "from": 0,
                          "insert": "type:commit "
                        }
                      },
                      {
                        "label": "Add \\"type:diff\\"",
                        "change": {
                          "from": 0,
                          "insert": "type:diff "
                        }
                      }
                    ]
                  }
                ]
            `)
            expect(parseAndDiagnose('author:me test', SearchPatternType.standard)).toMatchInlineSnapshot(`
                [
                  {
                    "severity": "error",
                    "message": "This filter requires \`type:commit\` or \`type:diff\` in the query",
                    "range": {
                      "start": 0,
                      "end": 9
                    },
                    "actions": [
                      {
                        "label": "Add \\"type:commit\\"",
                        "change": {
                          "from": 0,
                          "insert": "type:commit "
                        }
                      },
                      {
                        "label": "Add \\"type:diff\\"",
                        "change": {
                          "from": 0,
                          "insert": "type:diff "
                        }
                      }
                    ]
                  }
                ]
            `)
            expect(
                parseAndDiagnose(
                    'author:me before:yesterday after:"last week" message:test',
                    SearchPatternType.standard
                )
            ).toMatchInlineSnapshot(`
                [
                  {
                    "severity": "error",
                    "message": "This filter requires \`type:commit\` or \`type:diff\` in the query",
                    "range": {
                      "start": 0,
                      "end": 9
                    },
                    "actions": [
                      {
                        "label": "Add \\"type:commit\\"",
                        "change": {
                          "from": 0,
                          "insert": "type:commit "
                        }
                      },
                      {
                        "label": "Add \\"type:diff\\"",
                        "change": {
                          "from": 0,
                          "insert": "type:diff "
                        }
                      }
                    ]
                  },
                  {
                    "severity": "error",
                    "message": "This filter requires \`type:commit\` or \`type:diff\` in the query",
                    "range": {
                      "start": 10,
                      "end": 26
                    },
                    "actions": [
                      {
                        "label": "Add \\"type:commit\\"",
                        "change": {
                          "from": 10,
                          "insert": "type:commit "
                        }
                      },
                      {
                        "label": "Add \\"type:diff\\"",
                        "change": {
                          "from": 10,
                          "insert": "type:diff "
                        }
                      }
                    ]
                  },
                  {
                    "severity": "error",
                    "message": "This filter requires \`type:commit\` or \`type:diff\` in the query",
                    "range": {
                      "start": 27,
                      "end": 44
                    },
                    "actions": [
                      {
                        "label": "Add \\"type:commit\\"",
                        "change": {
                          "from": 27,
                          "insert": "type:commit "
                        }
                      },
                      {
                        "label": "Add \\"type:diff\\"",
                        "change": {
                          "from": 27,
                          "insert": "type:diff "
                        }
                      }
                    ]
                  },
                  {
                    "severity": "error",
                    "message": "This filter requires \`type:commit\` or \`type:diff\` in the query",
                    "range": {
                      "start": 45,
                      "end": 57
                    },
                    "actions": [
                      {
                        "label": "Add \\"type:commit\\"",
                        "change": {
                          "from": 45,
                          "insert": "type:commit "
                        }
                      },
                      {
                        "label": "Add \\"type:diff\\"",
                        "change": {
                          "from": 45,
                          "insert": "type:diff "
                        }
                      }
                    ]
                  }
                ]
            `)
            expect(parseAndDiagnose('until:yesterday since:"last week" m:test', SearchPatternType.standard))
                .toMatchInlineSnapshot(`
                [
                  {
                    "severity": "error",
                    "message": "This filter requires \`type:commit\` or \`type:diff\` in the query",
                    "range": {
                      "start": 0,
                      "end": 15
                    },
                    "actions": [
                      {
                        "label": "Add \\"type:commit\\"",
                        "change": {
                          "from": 0,
                          "insert": "type:commit "
                        }
                      },
                      {
                        "label": "Add \\"type:diff\\"",
                        "change": {
                          "from": 0,
                          "insert": "type:diff "
                        }
                      }
                    ]
                  },
                  {
                    "severity": "error",
                    "message": "This filter requires \`type:commit\` or \`type:diff\` in the query",
                    "range": {
                      "start": 16,
                      "end": 33
                    },
                    "actions": [
                      {
                        "label": "Add \\"type:commit\\"",
                        "change": {
                          "from": 16,
                          "insert": "type:commit "
                        }
                      },
                      {
                        "label": "Add \\"type:diff\\"",
                        "change": {
                          "from": 16,
                          "insert": "type:diff "
                        }
                      }
                    ]
                  },
                  {
                    "severity": "error",
                    "message": "This filter requires \`type:commit\` or \`type:diff\` in the query",
                    "range": {
                      "start": 34,
                      "end": 40
                    },
                    "actions": [
                      {
                        "label": "Add \\"type:commit\\"",
                        "change": {
                          "from": 34,
                          "insert": "type:commit "
                        }
                      },
                      {
                        "label": "Add \\"type:diff\\"",
                        "change": {
                          "from": 34,
                          "insert": "type:diff "
                        }
                      }
                    ]
                  }
                ]
            `)
        })

        test('accepts author/before/after/message filters if type:diff is present', () => {
            expect(
                parseAndDiagnose(
                    'author:me before:yesterday after:"last week" message:test type:diff',
                    SearchPatternType.standard
                )
            ).toMatchInlineSnapshot('[]')
        })

        test('accepts author/before/after/message filters if type:commit is present', () => {
            expect(
                parseAndDiagnose(
                    'author:me before:yesterday after:"last week" message:test type:diff',
                    SearchPatternType.standard
                )
            ).toMatchInlineSnapshot('[]')
        })
    })

    describe('repo and rev filters', () => {
        test('detects rev without repo filter', () => {
            expect(parseAndDiagnose('rev:main test', SearchPatternType.standard)).toMatchInlineSnapshot(`
                [
                  {
                    "severity": "error",
                    "message": "Query contains \`rev:\` without \`repo:\`. Add a \`repo:\` filter.",
                    "range": {
                      "start": 0,
                      "end": 8
                    },
                    "actions": [
                      {
                        "label": "Add \\"repo:\\"",
                        "change": {
                          "from": 8,
                          "insert": " repo:"
                        },
                        "selection": {
                          "anchor": 14
                        }
                      }
                    ]
                  }
                ]
            `)
            expect(parseAndDiagnose('revision:main test', SearchPatternType.standard)).toMatchInlineSnapshot(`
                [
                  {
                    "severity": "error",
                    "message": "Query contains \`rev:\` without \`repo:\`. Add a \`repo:\` filter.",
                    "range": {
                      "start": 0,
                      "end": 13
                    },
                    "actions": [
                      {
                        "label": "Add \\"repo:\\"",
                        "change": {
                          "from": 13,
                          "insert": " repo:"
                        },
                        "selection": {
                          "anchor": 19
                        }
                      }
                    ]
                  }
                ]
            `)
        })

        test('detects rev with repo+rev tag filter', () => {
            expect(parseAndDiagnose('rev:main repo:test@dev test', SearchPatternType.standard)).toMatchInlineSnapshot(`
                [
                  {
                    "severity": "error",
                    "message": "You have specified both \`@<rev>\` and \`rev:\` for a repo filter and I don't know how to interpret this. Remove either \`@<rev>\` or \`rev:\`",
                    "range": {
                      "start": 9,
                      "end": 22
                    },
                    "actions": [
                      {
                        "label": "Remove @<rev>",
                        "change": {
                          "from": 18,
                          "to": 22
                        }
                      },
                      {
                        "label": "Remove \\"rev:\\" filter",
                        "change": {
                          "from": 0,
                          "to": 8
                        }
                      }
                    ]
                  },
                  {
                    "severity": "error",
                    "message": "You have specified both \`@<rev>\` and \`rev:\` for a repo filter and I don't know how to interpret this. Remove either \`@<rev>\` or \`rev:\`",
                    "range": {
                      "start": 0,
                      "end": 8
                    },
                    "actions": [
                      {
                        "label": "Remove @<rev>",
                        "change": {
                          "from": 18,
                          "to": 22
                        }
                      },
                      {
                        "label": "Remove \\"rev:\\" filter",
                        "change": {
                          "from": 0,
                          "to": 8
                        }
                      }
                    ]
                  }
                ]
            `)
            expect(parseAndDiagnose('rev:main r:test@dev test', SearchPatternType.standard)).toMatchInlineSnapshot(`
                [
                  {
                    "severity": "error",
                    "message": "You have specified both \`@<rev>\` and \`rev:\` for a repo filter and I don't know how to interpret this. Remove either \`@<rev>\` or \`rev:\`",
                    "range": {
                      "start": 9,
                      "end": 19
                    },
                    "actions": [
                      {
                        "label": "Remove @<rev>",
                        "change": {
                          "from": 15,
                          "to": 19
                        }
                      },
                      {
                        "label": "Remove \\"rev:\\" filter",
                        "change": {
                          "from": 0,
                          "to": 8
                        }
                      }
                    ]
                  },
                  {
                    "severity": "error",
                    "message": "You have specified both \`@<rev>\` and \`rev:\` for a repo filter and I don't know how to interpret this. Remove either \`@<rev>\` or \`rev:\`",
                    "range": {
                      "start": 0,
                      "end": 8
                    },
                    "actions": [
                      {
                        "label": "Remove @<rev>",
                        "change": {
                          "from": 15,
                          "to": 19
                        }
                      },
                      {
                        "label": "Remove \\"rev:\\" filter",
                        "change": {
                          "from": 0,
                          "to": 8
                        }
                      }
                    ]
                  }
                ]
            `)
        })

        test('detects rev with empty repo filter', () => {
            expect(parseAndDiagnose('rev:main repo: test', SearchPatternType.standard)).toMatchInlineSnapshot(`
                [
                  {
                    "severity": "warning",
                    "message": "This filter is empty. Remove the space between the filter and value or quote the value to include the space. E.g. \`repo:\\" a term\\"\`.",
                    "range": {
                      "start": 9,
                      "end": 14
                    }
                  },
                  {
                    "severity": "error",
                    "message": "Query contains \`rev:\` with an empty \`repo:\` filter. Add a non-empty \`repo:\` filter.",
                    "range": {
                      "start": 9,
                      "end": 14
                    },
                    "actions": [
                      {
                        "label": "Add \\"repo:\\" value",
                        "selection": {
                          "anchor": 14
                        }
                      }
                    ]
                  },
                  {
                    "severity": "error",
                    "message": "Query contains \`rev:\` with an empty \`repo:\` filter. Add a non-empty \`repo:\` filter.",
                    "range": {
                      "start": 0,
                      "end": 8
                    },
                    "actions": [
                      {
                        "label": "Add \\"repo:\\" value",
                        "selection": {
                          "anchor": 14
                        }
                      }
                    ]
                  }
                ]
            `)
            expect(parseAndDiagnose('rev:main repo:', SearchPatternType.standard)).toMatchInlineSnapshot(`
                [
                  {
                    "severity": "error",
                    "message": "Query contains \`rev:\` with an empty \`repo:\` filter. Add a non-empty \`repo:\` filter.",
                    "range": {
                      "start": 9,
                      "end": 14
                    },
                    "actions": [
                      {
                        "label": "Add \\"repo:\\" value",
                        "selection": {
                          "anchor": 14
                        }
                      }
                    ]
                  },
                  {
                    "severity": "error",
                    "message": "Query contains \`rev:\` with an empty \`repo:\` filter. Add a non-empty \`repo:\` filter.",
                    "range": {
                      "start": 0,
                      "end": 8
                    },
                    "actions": [
                      {
                        "label": "Add \\"repo:\\" value",
                        "selection": {
                          "anchor": 14
                        }
                      }
                    ]
                  }
                ]
            `)
        })

        test('accepts rev filter if valid repo filter is present', () => {
            expect(parseAndDiagnose('repo:test rev:main repo: -repo:main@dev test', SearchPatternType.standard))
                .toMatchInlineSnapshot(`
                [
                  {
                    "severity": "warning",
                    "message": "This filter is empty. Remove the space between the filter and value or quote the value to include the space. E.g. \`repo:\\" a term\\"\`.",
                    "range": {
                      "start": 19,
                      "end": 24
                    }
                  }
                ]
            `)
        })
    })

    describe('structural search and type: filter', () => {
        const origContext = window.context
        afterAll(() => {
            window.context = origContext
        })
        window.context = {
            experimentalFeatures: {
                structuralSearch: 'enabled',
            },
        }
        test('detects type: filter in structural search', () => {
            expect(parseAndDiagnose('type:symbol test lang:go', SearchPatternType.structural)).toMatchInlineSnapshot(`
                [
                  {
                    "severity": "error",
                    "message": "Structural search syntax only applies to searching file contents and is not compatible with \`type:\`. Remove this filter or switch to a different search type.",
                    "range": {
                      "start": 0,
                      "end": 11
                    }
                  }
                ]
            `)
            expect(parseAndDiagnose('type:symbol test lang:go patterntype:structural', SearchPatternType.standard))
                .toMatchInlineSnapshot(`
                [
                  {
                    "severity": "error",
                    "message": "Structural search syntax only applies to searching file contents and is not compatible with \`type:\`. Remove this filter or switch to a different search type.",
                    "range": {
                      "start": 0,
                      "end": 11
                    }
                  }
                ]
            `)
        })

        test('accepts type: filter in non-structure search', () => {
            expect(parseAndDiagnose('type:symbol test', SearchPatternType.regexp)).toMatchInlineSnapshot('[]')
            expect(parseAndDiagnose('type:symbol test', SearchPatternType.standard)).toMatchInlineSnapshot('[]')
            // patterntype: takes presedence
            expect(
                parseAndDiagnose('type:symbol test patterntype:literal', SearchPatternType.structural)
            ).toMatchInlineSnapshot('[]')
        })
    })

    describe('structural search without lang: filter', () => {
        const origContext = window.context
        afterAll(() => {
            window.context = origContext
        })
        window.context = {
            experimentalFeatures: {
                structuralSearch: 'enabled',
            },
        }
        test('detects structural search without lang filter', () => {
            expect(parseAndDiagnose('repo:foo bar', SearchPatternType.structural)).toMatchInlineSnapshot(`
                [
                  {
                    "severity": "warning",
                    "message": "Add a \`lang\` filter when using structural search. Structural search may miss results without a \`lang\` filter because it only guesses the language of files searched.",
                    "range": {
                      "start": 9,
                      "end": 12
                    }
                  }
                ]
            `)
        })
    })
})
