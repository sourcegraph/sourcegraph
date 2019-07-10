import express from 'express'
import asyncHandler from 'express-async-handler'
import bodyParser from 'body-parser'
import { JsonDatabase } from './json'
import { Database, noopTransformer } from './database'
import { fs } from 'mz'
import * as path from 'path'
import LRU from 'lru-cache'
import { withFile } from 'tmp-promise'

/**
 * Reads an integer from an environment variable or defaults to the given value.
 */
function readEnvInt({ key, def }: { key: string; def: number }): number {
    const value = process.env[key]
    if (!value) {
        return def
    }
    const n = parseInt(value)
    if (isNaN(n)) {
        return def
    }
    return n
}

/**
 * Where on the file system to store LSIF files.
 */
const storageRoot = process.env['SRC_LSIF_STORAGE_ROOT'] || 'lsif'

/**
 * Soft limit on the amount of storage used by LSIF files. Storage can exceed
 * this limit if a single LSIF file is larger than this, otherwise storage will
 * be kept under this limit.
 */
const softMaxStorage = readEnvInt({ key: 'SRC_LSIF_SOFT_MAX_STORAGE', def: 100 * 1024 * 1024 * 1024 })

/**
 * Limit on the file size accepted by the /upload endpoint.
 */
const maxFileSize = readEnvInt({ key: 'SRC_LSIF_MAX_FILE_SIZE', def: 100 * 1024 * 1024 })

/**
 * Soft limit on the total amount of storage occupied by LSIF data loaded in
 * memory. The actual amount can exceed this if a single LSIF file is larger
 * than this limit, otherwise memory will be kept under this limit.
 *
 * Empirically based on github.com/sourcegraph/codeintellify, each byte of
 * storage (uncompressed newline-delimited JSON) expands to 3 bytes in memory.
 */
const softMaxStorageInMemory = readEnvInt({ key: 'SRC_LSIF_SOFT_MAX_STORAGE_IN_MEMORY', def: 100 * 1024 * 1024 })

/**
 * Which port to run the LSIF server on.
 */
const port = readEnvInt({ key: 'SRC_LSIF_HTTP_PORT', def: 3185 })

/**
 * An opaque repository ID.
 */
interface Repository {
    repository: string
}

/**
 * A 40-character commit hash.
 */
interface Commit {
    commit: string
}

/**
 * Combines `Repository` and `Commit`.
 */
interface RepositoryCommit extends Repository, Commit {}

/**
 * Deletes old files (sorted by last modified time) to keep the disk usage below
 * the given `max`.
 */
async function enforceMaxDiskUsage({
    flatDirectory,
    max,
    onBeforeDelete,
}: {
    flatDirectory: string
    max: number
    onBeforeDelete: (filePath: string) => void
}): Promise<void> {
    if (!(await fs.exists(flatDirectory))) {
        return
    }
    const files = await Promise.all(
        (await fs.readdir(flatDirectory)).map(async f => ({
            path: path.join(flatDirectory, f),
            stat: await fs.stat(path.join(flatDirectory, f)),
        }))
    )
    let totalSize = files.reduce((subtotal, f) => subtotal + f.stat.size, 0)
    for (const f of files.sort((a, b) => a.stat.mtimeMs - b.stat.mtimeMs)) {
        if (totalSize <= max) {
            break
        }
        onBeforeDelete(f.path)
        await fs.unlink(f.path)
        totalSize = totalSize - f.stat.size
    }
}

/**
 * Computes the filename that contains LSIF data for the given repository@commit.
 */
function diskKey({ repository, commit }: RepositoryCommit): string {
    const base64Repository = Buffer.from(repository).toString('base64')
    return path.join(storageRoot, `base64repository:${base64Repository},commit:${commit}.lsif`)
}

/**
 * Loads LSIF data from disk and returns a promise to the resulting `Database`.
 * Throws ENOENT when there is no LSIF data for the given repository@commit.
 */
async function createDB(repositoryCommit: RepositoryCommit): Promise<Database> {
    const db = new JsonDatabase()
    await db.load(diskKey(repositoryCommit), projectRoot => ({
        toDatabase: path_ => projectRoot + '/' + path_,
        fromDatabase: path_ => (path_.startsWith(projectRoot) ? path_.slice(`${projectRoot}/`.length) : path_),
    }))
    return db
}

/**
 * List of supported `Database` methods.
 */
const supportedMethods = ['hover', 'definitions', 'references']

/**
 * Throws an error with status 400 if the repository is invalid.
 */
function checkRepository(repository: any): void {
    if (typeof repository !== 'string') {
        throw Object.assign(new Error('must specify the repository (usually of the form github.com/user/repo)'), {
            status: 400,
        })
    }
}

/**
 * Throws an error with status 400 if the commit is invalid.
 */
function checkCommit(commit: any): void {
    if (typeof commit !== 'string' || commit.length !== 40 || !/^[0-9a-f]+$/.test(commit)) {
        throw Object.assign(new Error('must specify the commit as a 40 character hash ' + commit), { status: 400 })
    }
}

/**
 * A `Database`, the size of the LSIF file it was loaded from, and a callback to
 * dispose of it when evicted from the cache.
 */
interface LRUDBEntry {
    dbPromise: Promise<Database>
    length: number
    dispose: () => void
}

/**
 * An LRU cache mapping `repository@commit`s to in-memory `Database`s. Old
 * `Database`s are evicted from the cache to prevent OOM errors.
 */
const dbLRU = new LRU<String, LRUDBEntry>({
    max: softMaxStorageInMemory,
    length: (entry, key) => entry.length,
    dispose: (key, entry) => entry.dispose(),
})

/**
 * Runs the given `action` with the `Database` associated with the given
 * repository@commit. Internally, it either gets the `Database` from the LRU
 * cache or loads it from storage.
 */
async function withDB(repositoryCommit: RepositoryCommit, action: (db: Database) => Promise<void>): Promise<void> {
    const entry = dbLRU.get(diskKey(repositoryCommit))
    if (entry) {
        await action(await entry.dbPromise)
    } else {
        const length = (await fs.stat(diskKey(repositoryCommit))).size
        const dbPromise = createDB(repositoryCommit)
        dbLRU.set(diskKey(repositoryCommit), {
            dbPromise: dbPromise,
            length: length,
            dispose: () => dbPromise.then(db => db.close()),
        })
        await action(await dbPromise)
    }
}

/**
 * Runs the HTTP server which accepts LSIF file uploads and responds to
 * hover/defs/refs requests.
 */
function main() {
    const app = express()

    // This limit only applies to JSON requests (i.e. the /request endpoint).
    app.use(bodyParser.json({ limit: '1mb' }))

    app.get('/ping', (req, res) => {
        res.send({ pong: 'pong' })
    })

    app.post(
        '/request',
        asyncHandler(async (req, res) => {
            const { repository, commit } = req.query
            const { method, params } = req.body

            checkRepository(repository)
            checkCommit(commit)
            if (!supportedMethods.includes(method)) {
                throw Object.assign(new Error('method must be one of ' + supportedMethods), { status: 400 })
            }

            try {
                await withDB({ repository, commit }, async db => {
                    let result: any
                    switch (method) {
                        case 'hover':
                            result = db.hover(params[0], params[1])
                            break
                        case 'definitions':
                            result = db.definitions(params[0], params[1])
                            break
                        case 'references':
                            result = db.references(params[0], params[1], { includeDeclaration: false })
                            break
                        default:
                            res.status(500).send({ error: `Unknown method ${method}` })
                            return
                    }
                    res.send(result || { error: 'No result found' })
                })
            } catch (e) {
                if ('code' in e && e.code === 'ENOENT') {
                    res.send({ error: `No LSIF data available for ${repository}@${commit}.` })
                    return
                }
                throw e
            }
        })
    )

    app.get(
        '/exists',
        asyncHandler(async (req, res) => {
            const { repository, commit, file } = req.query

            checkRepository(repository)
            checkCommit(commit)

            if (!file) {
                res.send(await fs.exists(diskKey({ repository, commit })))
                return
            }

            if (typeof file !== 'string') {
                throw Object.assign(new Error('file must be a string'), { status: 400 })
            }

            res.send(Boolean((await createDB({ repository, commit })).stat(file)))
        })
    )

    app.post(
        '/upload',
        asyncHandler(async (req, res) => {
            const { repository, commit } = req.query

            checkRepository(repository)
            checkCommit(commit)

            const contentLength = parseInt(req.header('Content-Length') || '') || 0
            if (contentLength > maxFileSize) {
                throw Object.assign(
                    new Error(
                        `The size of the given LSIF file (${contentLength} bytes) exceeds the max of ${maxFileSize}`
                    ),
                    { status: 400 }
                )
            }

            // TODO enforce max disk usage per-repository. Currently, a
            // misbehaving client could upload a bunch of LSIF files for one
            // repository and take up all of the disk space, causing all other
            // LSIF files to get deleted to make room for the new files.
            await enforceMaxDiskUsage({
                flatDirectory: storageRoot,
                max: Math.max(0, softMaxStorage - contentLength),
                onBeforeDelete: filePath =>
                    console.log(`Deleting ${filePath} to help keep disk usage under ${softMaxStorage}.`),
            })

            await withFile(async tempFile => {
                // TODO bail early if the request body contains more data than was
                // specified in the Content-Length header. Currently, if a client
                // lies about the Content-Length, they can send a huge body that
                // fills up the disk.

                // Pipe the given LSIF data to a temp file.
                const stream = req.pipe(fs.createWriteStream(tempFile.path))
                await new Promise((resolve, reject) => {
                    stream.on('close', async () => {
                        if ((await fs.stat(tempFile.path)).size > contentLength) {
                            reject(
                                Object.assign(
                                    new Error(
                                        `The size of the given LSIF file (${contentLength} bytes) exceeds the specified Content-Length ${contentLength}`
                                    ),
                                    { status: 400 }
                                )
                            )
                            return
                        }
                        resolve()
                    })

                    stream.on('error', error => {
                        reject(error)
                    })
                })

                // Load a `Database` from the file to check that it's valid.
                await new JsonDatabase().load(tempFile.path, () => noopTransformer)

                // Replace the old LSIF file with the new file.
                if (!(await fs.exists(storageRoot))) {
                    await fs.mkdir(storageRoot)
                }
                await fs.rename(tempFile.path, diskKey({ repository, commit }))

                // Evict the old `Database` from the LRU cache to cause it to pick up the new LSIF data from disk.
                dbLRU.del(diskKey({ repository, commit }))

                res.send('Upload successful.')
            })
        })
    )

    app.listen(port, () => {
        console.log(`Listening for HTTP requests on port ${port}`)
    })
}

main()
