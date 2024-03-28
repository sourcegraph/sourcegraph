/* eslint-disable no-sync */
import fs from 'fs'
import path from 'path'

// NOTE: use fs.realpathSync() in addition to path.resolve() to resolve
// symlinks to the real path. This is required when the file path contains symlinks such
// as when using pnpm.
export function resolveWithSymlink(...args: string[]): string {
    const resolvedPath = path.resolve(...args)

    try {
        return fs.realpathSync(resolvedPath)
    } catch {
        return resolvedPath
    }
}

export const ROOT_PATH = resolveWithSymlink(__dirname, '../../../../')
export const NODE_MODULES_PATH = resolveWithSymlink(ROOT_PATH, 'node_modules')
