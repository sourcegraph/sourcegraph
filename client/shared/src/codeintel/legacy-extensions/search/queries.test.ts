import * as assert from 'assert'

import * as sourcegraph from '../api'

import { definitionQuery, referencesQuery } from './queries'

describe('search requests', () => {
    it('makes correct search requests for goto definition', () => {
        interface DefinitionTest {
            doc: sourcegraph.TextDocument
            expectedSearchQueryTerms: string[]
        }
        const tests: DefinitionTest[] = [
            {
                doc: {
                    uri: 'git://github.com/foo/bar?rev#file.cpp',
                    languageId: 'cpp',
                    text: 'token',
                },
                expectedSearchQueryTerms: [
                    '^token$',
                    'type:symbol',
                    'patternType:regexp',
                    'count:50',
                    'case:yes',
                    'file:\\.(cpp)$',
                ],
            },
        ]

        for (const test of tests) {
            assert.deepStrictEqual(
                definitionQuery({
                    searchToken: 'token',
                    doc: test.doc,
                    fileExts: ['cpp'],
                }),
                test.expectedSearchQueryTerms
            )
        }
    })

    it('makes correct search requests for references', () => {
        interface ReferencesTest {
            doc: sourcegraph.TextDocument
            expectedSearchQueryTerms: string[]
        }
        const tests: ReferencesTest[] = [
            {
                doc: {
                    uri: 'git://github.com/foo/bar?rev#file.cpp',
                    languageId: 'cpp',
                    text: 'token',
                },
                expectedSearchQueryTerms: [
                    '\\btoken\\b',
                    'type:file',
                    'patternType:regexp',
                    'count:500',
                    'case:yes',
                    'file:\\.(cpp)$',
                ],
            },
        ]

        for (const test of tests) {
            assert.deepStrictEqual(
                referencesQuery({
                    searchToken: 'token',
                    doc: test.doc,
                    fileExts: ['cpp'],
                }),
                test.expectedSearchQueryTerms
            )
        }
    })
})
