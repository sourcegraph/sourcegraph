import { SourcegraphURL } from '@sourcegraph/common'
import type { Position } from '@sourcegraph/extension-api-types'
import { parseRepoRevision } from '@sourcegraph/shared/src/util/url'

export interface SourcegraphUriOptionals {
    revision?: string
    path?: string
    position?: Position
    isDirectory?: boolean
    isCommit?: boolean
    compareRange?: CompareRange
}

export interface CompareRange {
    base: string
    head: string
}

/**
 * SourcegraphUri encodes a URI like `sourcegraph://HOST/REPOSITORY@REVISION/-/blob/PATH?L1337`.
 *
 * This class is used in both webviews and extensions, so try to avoid state management in this class or module.
 */
export class SourcegraphUri {
    private constructor(
        public readonly uri: string,
        public readonly host: string,
        public readonly repositoryName: string,
        public readonly revision: string,
        public readonly path: string | undefined,
        public readonly position: Position | undefined,
        public readonly compareRange: CompareRange | undefined
    ) {}

    public withRevision(newRevision: string | undefined): SourcegraphUri {
        const newRevisionPath = newRevision ? `@${newRevision}` : ''
        return SourcegraphUri.parse(
            `sourcegraph://${this.host}/${this.repositoryName}${newRevisionPath}/-/blob/${
                this.path || ''
            }${this.positionSuffix()}`
        )
    }

    public with(optionals: SourcegraphUriOptionals): SourcegraphUri {
        return SourcegraphUri.fromParts(this.host, this.repositoryName, {
            path: this.path,
            revision: this.revision,
            compareRange: this.compareRange,
            position: this.position,
            ...optionals,
        })
    }

    public withPath(newPath: string): SourcegraphUri {
        return SourcegraphUri.parse(`${this.repositoryUri()}/-/blob/${newPath}${this.positionSuffix()}`)
    }

    public basename(): string {
        const parts = (this.path || '').split('/')
        return parts.at(-1)!
    }

    public dirname(): string {
        const parts = (this.path || '').split('/')
        return parts.slice(0, -1).join('/')
    }

    public parentUri(): string | undefined {
        if (typeof this.path === 'string') {
            const slash = this.uri.lastIndexOf('/')
            if (slash < 0 || !this.path.includes('/')) {
                return `sourcegraph://${this.host}/${this.repositoryName}${this.revisionPart()}`
            }
            const parent = this.uri.slice(0, slash).replace('/-/blob/', '/-/tree/')
            return parent
        }
        return undefined
    }

    public withIsDirectory(isDirectory: boolean): SourcegraphUri {
        return SourcegraphUri.fromParts(this.host, this.repositoryName, {
            isDirectory,
            path: this.path,
            revision: this.revision,
            position: this.position,
        })
    }

    public isCommit(): boolean {
        return this.uri.includes('/-/commit/')
    }

    public isCompare(): boolean {
        return this.uri.includes('/-/compare/') && this.compareRange !== undefined
    }

    public isDirectory(): boolean {
        return this.uri.includes('/-/tree/')
    }

    public isFile(): boolean {
        return this.uri.includes('/-/blob/')
    }

    public static fromParts(host: string, repositoryName: string, optional?: SourcegraphUriOptionals): SourcegraphUri {
        const revisionPart = optional?.revision ? `@${optional.revision}` : ''
        const directoryPart = optional?.isDirectory
            ? 'tree'
            : optional?.isCommit
            ? 'commit'
            : optional?.compareRange
            ? 'compare'
            : 'blob'
        const pathPart = optional?.compareRange
            ? `/-/compare/${optional.compareRange.base}...${optional.compareRange.head}`
            : optional?.isCommit && optional.revision
            ? `/-/commit/${optional.revision}`
            : optional?.path
            ? `/-/${directoryPart}/${optional?.path}`
            : ''
        const uri = `sourcegraph://${host}/${repositoryName}${revisionPart}${pathPart}`
        return new SourcegraphUri(
            uri,
            host,
            repositoryName,
            optional?.revision || '',
            optional?.path,
            optional?.position,
            optional?.compareRange
        )
    }

    public repositoryUri(): string {
        return `sourcegraph://${this.host}/${this.repositoryName}${this.revisionPart()}`
    }

    public treeItemLabel(parent?: SourcegraphUri): string {
        if (this.path) {
            if (parent?.path) {
                return this.path.slice(parent.path.length + 1)
            }
            return this.path
        }
        return `${this.repositoryName}`
    }

    public revisionPart(): string {
        return this.revision ? `@${this.revision}` : ''
    }

    public positionSuffix(): string {
        return this.position === undefined ? '' : `?L${this.position.line}:${this.position.character}`
    }

    // Debt: refactor and use shared functions. Below is based on parseBrowserRepoURL
    // https://sourcegraph.com/github.com/sourcegraph/sourcegraph@56dfaaa3e3172f9afd4a29a4780a7f1a34198238/-/blob/client/shared/src/util/url.ts?L287
    // In the browser, pass in window.URL. When we use the shared implementation, pass in the URL module from Node.
    public static parse(uri: string, URLModule = URL): SourcegraphUri {
        uri = uri.replace('https://', 'sourcegraph://')
        const url = new URLModule(uri.replace('sourcegraph://', 'https://'))
        let pathname = url.pathname.slice(1) // trim leading '/'
        if (pathname.endsWith('/')) {
            pathname = pathname.slice(0, -1) // trim trailing '/'
        }

        const indexOfSeparator = pathname.indexOf('/-/')

        // examples:
        // - 'github.com/gorilla/mux'
        // - 'github.com/gorilla/mux@revision'
        // - 'foo/bar' (from 'sourcegraph.mycompany.com/foo/bar')
        // - 'foo/bar@revision' (from 'sourcegraph.mycompany.com/foo/bar@revision')
        // - 'foobar' (from 'sourcegraph.mycompany.com/foobar')
        // - 'foobar@revision' (from 'sourcegraph.mycompany.com/foobar@revision')
        let repoRevision: string
        if (indexOfSeparator === -1) {
            repoRevision = pathname // the whole string
        } else {
            repoRevision = pathname.slice(0, indexOfSeparator) // the whole string leading up to the separator (allows revision to be multiple path parts)
        }
        let { repoName, revision } = parseRepoRevision(repoRevision)

        let path: string | undefined
        let compareRange: CompareRange | undefined
        const treeSeparator = pathname.indexOf('/-/tree/')
        const blobSeparator = pathname.indexOf('/-/blob/')
        const commitSeparator = pathname.indexOf('/-/commit/')
        const comparisonSeparator = pathname.indexOf('/-/compare/')
        if (treeSeparator !== -1) {
            path = decodeURIComponent(pathname.slice(treeSeparator + '/-/tree/'.length))
        }
        if (blobSeparator !== -1) {
            path = decodeURIComponent(pathname.slice(blobSeparator + '/-/blob/'.length))
        }
        if (commitSeparator !== -1) {
            path = decodeURIComponent(pathname.slice(commitSeparator + '/-/commit/'.length))
        }
        if (comparisonSeparator !== -1) {
            const range = pathname.slice(comparisonSeparator + '/-/compare/'.length)
            const parts = range.split('...')
            if (parts.length === 2) {
                const [base, head] = parts
                compareRange = { base, head }
            }
        }
        let position: Position | undefined

        const lineRange = SourcegraphURL.from(url.toString()).lineRange
        if (lineRange.line) {
            position = {
                line: lineRange.line,
                character: lineRange.character || 0,
            }
        }
        const isDirectory = uri.includes('/-/tree/')
        const isCommit = uri.includes('/-/commit/')
        if (isCommit) {
            revision = url.pathname.replace(new RegExp('.*/-/commit/([^/]+).*'), (_unused, oid: string) => oid)
            path = path?.slice(`${revision}/`.length)
        }
        return SourcegraphUri.fromParts(url.host, repoName, {
            revision,
            path,
            position,
            isDirectory,
            isCommit,
            compareRange,
        })
    }
}
