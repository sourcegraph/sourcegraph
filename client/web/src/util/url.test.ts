import { describe, expect, test } from 'vitest'

import type { RepoFile } from '@sourcegraph/shared/src/util/url'

import { getURLToFileCommit, parseBrowserRepoURL, toTreeURL } from './url'

/**
 * Asserts deep object equality using node's assert.deepEqual, except it (1) ignores differences in the
 * prototype (because that causes 2 object literals to fail the test) and (2) treats undefined properties as
 * missing.
 */
function assertDeepStrictEqual<T>(actual: T, expected: T): void {
    actual = JSON.parse(JSON.stringify(actual))
    expected = JSON.parse(JSON.stringify(expected))
    expect(actual).toEqual(expected)
}

describe('toTreeURL', () => {
    test('formats url', () => {
        const target: RepoFile = {
            repoName: 'github.com/gorilla/mux',
            revision: '24fca303ac6da784b9e8269f724ddeb0b2eea5e7',
            filePath: 'mux.go',
        }
        expect(toTreeURL(target)).toBe('/github.com/gorilla/mux@24fca303ac6da784b9e8269f724ddeb0b2eea5e7/-/tree/mux.go')
    })

    // other cases are gratuitous given tests for other URL functions
})

describe('getURLToFileCommit', () => {
    test('should return valid URL', () => {
        const newURL = getURLToFileCommit(
            'https://sourcegraph.com/github.com/gorilla/mux@foo/baz/bar/-/blob/mux.go',
            'newfile.go',
            'somerev'
        )
        expect(newURL).toEqual('/github.com/gorilla/mux@somerev/-/blob/newfile.go')
    })
})

describe('parseBrowserRepoURL', () => {
    test('should parse github repo', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/github.com/gorilla/mux')
        assertDeepStrictEqual(parsed, {
            repoName: 'github.com/gorilla/mux',
        })
    })
    test('should parse repo', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/gorilla/mux')
        assertDeepStrictEqual(parsed, {
            repoName: 'gorilla/mux',
        })
    })

    test('should parse github repo with revision', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/github.com/gorilla/mux@branch')
        assertDeepStrictEqual(parsed, {
            rawRevision: 'branch',
            repoName: 'github.com/gorilla/mux',
            revision: 'branch',
        })
    })
    test('should parse repo with revision', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/gorilla/mux@branch')
        assertDeepStrictEqual(parsed, {
            rawRevision: 'branch',
            repoName: 'gorilla/mux',
            revision: 'branch',
        })
    })

    test('should parse perforce depot with changelist ID', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/gorilla/mux@changelist/136130')
        assertDeepStrictEqual(parsed, {
            repoName: 'gorilla/mux',
            revision: '136130',
            rawRevision: 'changelist/136130',
        })
    })

    test('should parse perforce depot with multi-path-part changelist', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/gorilla/mux@changelist/136130/-/blob/mux.go')
        assertDeepStrictEqual(parsed, {
            repoName: 'gorilla/mux',
            revision: '136130',
            rawRevision: 'changelist/136130',
            filePath: 'mux.go',
        })
    })

    test('should parse github repo with multi-path-part revision', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/github.com/gorilla/mux@foo/baz/bar')
        assertDeepStrictEqual(parsed, {
            rawRevision: 'foo/baz/bar',
            repoName: 'github.com/gorilla/mux',
            revision: 'foo/baz/bar',
        })
        const parsed2 = parseBrowserRepoURL('https://sourcegraph.com/github.com/gorilla/mux@foo/baz/bar/-/blob/mux.go')
        assertDeepStrictEqual(parsed2, {
            repoName: 'github.com/gorilla/mux',
            revision: 'foo/baz/bar',
            rawRevision: 'foo/baz/bar',
            filePath: 'mux.go',
        })
    })
    test('should parse repo with multi-path-part revision', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/gorilla/mux@foo/baz/bar')
        assertDeepStrictEqual(parsed, {
            repoName: 'gorilla/mux',
            revision: 'foo/baz/bar',
            rawRevision: 'foo/baz/bar',
        })
        const parsed2 = parseBrowserRepoURL('https://sourcegraph.com/gorilla/mux@foo/baz/bar/-/blob/mux.go')
        assertDeepStrictEqual(parsed2, {
            repoName: 'gorilla/mux',
            revision: 'foo/baz/bar',
            rawRevision: 'foo/baz/bar',
            filePath: 'mux.go',
        })
    })

    test('should parse github repo with commitID', () => {
        const parsed = parseBrowserRepoURL(
            'https://sourcegraph.com/github.com/gorilla/mux@24fca303ac6da784b9e8269f724ddeb0b2eea5e7'
        )
        assertDeepStrictEqual(parsed, {
            rawRevision: '24fca303ac6da784b9e8269f724ddeb0b2eea5e7',
            repoName: 'github.com/gorilla/mux',
            revision: '24fca303ac6da784b9e8269f724ddeb0b2eea5e7',
            commitID: '24fca303ac6da784b9e8269f724ddeb0b2eea5e7',
        })
    })
    test('should parse repo with commitID', () => {
        const parsed = parseBrowserRepoURL(
            'https://sourcegraph.com/gorilla/mux@24fca303ac6da784b9e8269f724ddeb0b2eea5e7'
        )
        assertDeepStrictEqual(parsed, {
            rawRevision: '24fca303ac6da784b9e8269f724ddeb0b2eea5e7',
            repoName: 'gorilla/mux',
            revision: '24fca303ac6da784b9e8269f724ddeb0b2eea5e7',
            commitID: '24fca303ac6da784b9e8269f724ddeb0b2eea5e7',
        })
    })

    test('should parse github repo with revision and file', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/github.com/gorilla/mux@branch/-/blob/mux.go')
        assertDeepStrictEqual(parsed, {
            repoName: 'github.com/gorilla/mux',
            revision: 'branch',
            rawRevision: 'branch',
            filePath: 'mux.go',
        })
    })
    test('should parse repo with revision and file', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/gorilla/mux@branch/-/blob/mux.go')
        assertDeepStrictEqual(parsed, {
            repoName: 'gorilla/mux',
            revision: 'branch',
            rawRevision: 'branch',
            filePath: 'mux.go',
        })
    })

    test('should parse github repo with revision and file and line', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/github.com/gorilla/mux@branch/-/blob/mux.go#L3')
        assertDeepStrictEqual(parsed, {
            repoName: 'github.com/gorilla/mux',
            revision: 'branch',
            rawRevision: 'branch',
            filePath: 'mux.go',
            position: {
                line: 3,
                character: 0,
            },
        })
    })
    test('should parse repo with revision and file and line', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/gorilla/mux@branch/-/blob/mux.go#L3')
        assertDeepStrictEqual(parsed, {
            repoName: 'gorilla/mux',
            revision: 'branch',
            rawRevision: 'branch',
            filePath: 'mux.go',
            position: {
                line: 3,
                character: 0,
            },
        })
    })

    test('should parse github repo with revision and file and position', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/github.com/gorilla/mux@branch/-/blob/mux.go#L3:5')
        assertDeepStrictEqual(parsed, {
            repoName: 'github.com/gorilla/mux',
            revision: 'branch',
            rawRevision: 'branch',
            filePath: 'mux.go',
            position: {
                line: 3,
                character: 5,
            },
        })
    })
    test('should parse repo with revision and file and position', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/gorilla/mux@branch/-/blob/mux.go#L3:5')
        assertDeepStrictEqual(parsed, {
            repoName: 'gorilla/mux',
            revision: 'branch',
            rawRevision: 'branch',
            filePath: 'mux.go',
            position: {
                line: 3,
                character: 5,
            },
        })
    })

    test('should parse repo with revisions containing @', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/github.com/emotion-js/emotion@@emotion/core@11.0.0')
        assertDeepStrictEqual(parsed, {
            repoName: 'github.com/emotion-js/emotion',
            revision: '@emotion/core@11.0.0',
            rawRevision: '@emotion/core@11.0.0',
        })
    })
})
