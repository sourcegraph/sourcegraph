import { SourcegraphUri } from './SourcegraphUri'

/**
 * Helper class to represent a flat list of relative file paths (type `string[]`) as a hierarchical file tree.
 */
export class FileTree {
    constructor(public readonly uri: SourcegraphUri, public readonly files: string[]) {
        files.sort()
    }

    public toString(): string {
        return `FileTree(${this.uri.uri}, files.length=${this.files.length})`
    }

    public directChildren(directory: string): string[] {
        return this.directChildrenInternal(directory, true)
    }

    private directChildrenInternal(directory: string, allowRecursion: boolean): string[] {
        const depth = this.depth(directory)
        const directFiles = new Set<string>()
        const directDirectories = new Set<string>()
        const isRoot = directory === ''
        if (!isRoot && !directory.endsWith('/')) {
            directory = directory + '/'
        }
        let index = this.binarySearchDirectoryStart(directory)
        while (index < this.files.length) {
            const startIndex = index
            const file = this.files[index]
            if (file === '') {
                index++
                continue
            }
            if (file.startsWith(directory)) {
                const fileDepth = this.depth(file)
                const isFile = isRoot ? fileDepth === 0 : fileDepth === depth + 1
                let path = isFile ? file : file.slice(0, file.indexOf('/', directory.length))
                let nestedChildren = allowRecursion && !isFile ? this.directChildrenInternal(path, false) : []
                while (allowRecursion && nestedChildren.length === 1) {
                    const child = SourcegraphUri.parse(nestedChildren[0])
                    if (child.isDirectory()) {
                        path = child.path || ''
                        nestedChildren = this.directChildrenInternal(path, false)
                    } else {
                        break
                    }
                }
                const uri = SourcegraphUri.fromParts(this.uri.host, this.uri.repositoryName, {
                    revision: this.uri.revision,
                    path,
                    isDirectory: !isFile,
                }).uri
                if (isFile) {
                    directFiles.add(uri)
                } else {
                    index = this.binarySearchDirectoryEnd(path + '/', index + 1)
                    directDirectories.add(uri)
                }
            }
            if (index === startIndex) {
                index++
            }
        }
        return [...directDirectories, ...directFiles]
    }

    private binarySearchDirectoryStart(directory: string): number {
        if (directory === '') {
            return 0
        }
        return this.binarySearch(
            { low: 0, high: this.files.length },
            midpoint => this.files[midpoint].localeCompare(directory) > 0
        )
    }

    private binarySearchDirectoryEnd(directory: string, low: number): number {
        while (low < this.files.length && this.files[low].localeCompare(directory) <= 0) {
            low++
        }
        return this.binarySearch(
            { low, high: this.files.length },
            midpoint => !this.files[midpoint].startsWith(directory)
        )
    }

    private binarySearch({ low, high }: SearchRange, isGreater: (midpoint: number) => boolean): number {
        while (low < high) {
            const midpoint = Math.floor(low + (high - low) / 2)
            if (isGreater(midpoint)) {
                high = midpoint
            } else {
                low = midpoint + 1
            }
        }
        return high
    }

    private depth(path: string): number {
        let result = 0
        for (const char of path) {
            if (char === '/') {
                result += 1
            }
        }
        return result
    }
}

interface SearchRange {
    low: number
    high: number
}
