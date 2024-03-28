export const isError = (value: unknown): value is Error => value instanceof Error

// Converts a git clone URL to the codebase name that includes the slash-separated code host, owner, and repository name
// This should captures:
// - "github:sourcegraph/sourcegraph" a common SSH host alias
// - "https://github.com/sourcegraph/deploy-sourcegraph-k8s.git"
// - "git@github.com:sourcegraph/sourcegraph.git"
export function convertGitCloneURLToCodebaseName(cloneURL: string): string | null {
    if (!cloneURL) {
        // eslint-disable-next-line no-console
        console.error(`Unable to determine the git clone URL for this workspace.\ngit output: ${cloneURL}`)
        return null
    }
    try {
        const uri = new URL(cloneURL.replace('git@', ''))
        // Handle common Git SSH URL format
        const match = cloneURL.match(/git@([^:]+):([\w-]+)\/([\w-]+)(\.git)?/)
        if (cloneURL.startsWith('git@') && match) {
            const host = match[1]
            const owner = match[2]
            const repo = match[3]
            return `${host}/${owner}/${repo}`
        }
        // Handle GitHub URLs
        if (uri.protocol.startsWith('github') || uri.href.startsWith('github')) {
            return `github.com/${uri.pathname.replace('.git', '')}`
        }
        // Handle GitLab URLs
        if (uri.protocol.startsWith('gitlab') || uri.href.startsWith('gitlab')) {
            return `gitlab.com/${uri.pathname.replace('.git', '')}`
        }
        // Handle HTTPS URLs
        if (uri.protocol.startsWith('http') && uri.hostname && uri.pathname) {
            return `${uri.hostname}${uri.pathname.replace('.git', '')}`
        }
        // Generic URL
        if (uri.hostname && uri.pathname) {
            return `${uri.hostname}${uri.pathname.replace('.git', '')}`
        }
        return null
    } catch (error) {
        // eslint-disable-next-line no-console
        console.error(`Cody could not extract repo name from clone URL ${cloneURL}:`, error)
        return null
    }
}
