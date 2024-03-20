import { parseRepoGitURI } from '../../../util/url'

/**
 * Extracts the components of a text document URI.
 *
 * @param uri The text document URL.
 */
export function parseGitURI(uri: string): { repo: string; commit: string; path: string } {
    const result = parseRepoGitURI(uri)
    return {
        repo: result.repoName,
        commit: result.revision ?? '',
        path: result.filePath ?? '',
    }
}
