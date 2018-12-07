import assert from 'assert'
import { parseBrowserRepoURL, toTreeURL } from './url'

/**
 * Asserts deep object equality using node's assert.deepStrictEqual, except it (1) ignores differences in the
 * prototype (because that causes 2 object literals to fail the test) and (2) treats undefined properties as
 * missing.
 */
function assertDeepStrictEqual(actual: any, expected: any, message?: string): void {
    actual = JSON.parse(JSON.stringify(actual))
    expected = JSON.parse(JSON.stringify(expected))
    assert.deepStrictEqual(actual, expected, message)
}

const ctx = {
    repoPath: 'github.com/gorilla/mux',
    rev: '',
    commitID: '24fca303ac6da784b9e8269f724ddeb0b2eea5e7',
    filePath: 'mux.go',
}

describe('toTreeURL', () => {
    it('formats url', () => {
        assert.strictEqual(
            toTreeURL(ctx),
            '/github.com/gorilla/mux@24fca303ac6da784b9e8269f724ddeb0b2eea5e7/-/tree/mux.go'
        )
    })

    // other cases are gratuitous given tests for other URL functions
})

describe('parseBrowserRepoURL', () => {
    it('should parse github repo', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/github.com/gorilla/mux')
        assertDeepStrictEqual(parsed, {
            repoPath: 'github.com/gorilla/mux',
        })
    })
    it('should parse repo', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/gorilla/mux')
        assertDeepStrictEqual(parsed, {
            repoPath: 'gorilla/mux',
        })
    })

    it('should parse github repo with rev', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/github.com/gorilla/mux@branch')
        assertDeepStrictEqual(parsed, {
            repoPath: 'github.com/gorilla/mux',
            rev: 'branch',
        })
    })
    it('should parse repo with rev', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/gorilla/mux@branch')
        assertDeepStrictEqual(parsed, {
            repoPath: 'gorilla/mux',
            rev: 'branch',
        })
    })

    it('should parse github repo with multi-path-part rev', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/github.com/gorilla/mux@foo/baz/bar')
        assertDeepStrictEqual(parsed, {
            repoPath: 'github.com/gorilla/mux',
            rev: 'foo/baz/bar',
        })
        const parsed2 = parseBrowserRepoURL('https://sourcegraph.com/github.com/gorilla/mux@foo/baz/bar/-/blob/mux.go')
        assertDeepStrictEqual(parsed2, {
            repoPath: 'github.com/gorilla/mux',
            rev: 'foo/baz/bar',
            filePath: 'mux.go',
        })
    })
    it('should parse repo with multi-path-part rev', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/gorilla/mux@foo/baz/bar')
        assertDeepStrictEqual(parsed, {
            repoPath: 'gorilla/mux',
            rev: 'foo/baz/bar',
        })
        const parsed2 = parseBrowserRepoURL('https://sourcegraph.com/gorilla/mux@foo/baz/bar/-/blob/mux.go')
        assertDeepStrictEqual(parsed2, {
            repoPath: 'gorilla/mux',
            rev: 'foo/baz/bar',
            filePath: 'mux.go',
        })
    })

    it('should parse github repo with commitID', () => {
        const parsed = parseBrowserRepoURL(
            'https://sourcegraph.com/github.com/gorilla/mux@24fca303ac6da784b9e8269f724ddeb0b2eea5e7'
        )
        assertDeepStrictEqual(parsed, {
            repoPath: 'github.com/gorilla/mux',
            rev: '24fca303ac6da784b9e8269f724ddeb0b2eea5e7',
            commitID: '24fca303ac6da784b9e8269f724ddeb0b2eea5e7',
        })
    })
    it('should parse repo with commitID', () => {
        const parsed = parseBrowserRepoURL(
            'https://sourcegraph.com/gorilla/mux@24fca303ac6da784b9e8269f724ddeb0b2eea5e7'
        )
        assertDeepStrictEqual(parsed, {
            repoPath: 'gorilla/mux',
            rev: '24fca303ac6da784b9e8269f724ddeb0b2eea5e7',
            commitID: '24fca303ac6da784b9e8269f724ddeb0b2eea5e7',
        })
    })

    it('should parse github repo with rev and file', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/github.com/gorilla/mux@branch/-/blob/mux.go')
        assertDeepStrictEqual(parsed, {
            repoPath: 'github.com/gorilla/mux',
            rev: 'branch',
            filePath: 'mux.go',
        })
    })
    it('should parse repo with rev and file', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/gorilla/mux@branch/-/blob/mux.go')
        assertDeepStrictEqual(parsed, {
            repoPath: 'gorilla/mux',
            rev: 'branch',
            filePath: 'mux.go',
        })
    })

    it('should parse github repo with rev and file and line', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/github.com/gorilla/mux@branch/-/blob/mux.go#L3')
        assertDeepStrictEqual(parsed, {
            repoPath: 'github.com/gorilla/mux',
            rev: 'branch',
            filePath: 'mux.go',
            position: {
                line: 3,
                character: 0,
            },
        })
    })
    it('should parse repo with rev and file and line', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/gorilla/mux@branch/-/blob/mux.go#L3')
        assertDeepStrictEqual(parsed, {
            repoPath: 'gorilla/mux',
            rev: 'branch',
            filePath: 'mux.go',
            position: {
                line: 3,
                character: 0,
            },
        })
    })

    it('should parse github repo with rev and file and position', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/github.com/gorilla/mux@branch/-/blob/mux.go#L3:5')
        assertDeepStrictEqual(parsed, {
            repoPath: 'github.com/gorilla/mux',
            rev: 'branch',
            filePath: 'mux.go',
            position: {
                line: 3,
                character: 5,
            },
        })
    })
    it('should parse repo with rev and file and position', () => {
        const parsed = parseBrowserRepoURL('https://sourcegraph.com/gorilla/mux@branch/-/blob/mux.go#L3:5')
        assertDeepStrictEqual(parsed, {
            repoPath: 'gorilla/mux',
            rev: 'branch',
            filePath: 'mux.go',
            position: {
                line: 3,
                character: 5,
            },
        })
    })
})
