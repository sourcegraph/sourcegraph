import { mkdirSync, type WriteStream, createWriteStream } from 'fs'
import path from 'path'
import type { Readable } from 'stream'

import { type Entry, open as _openZip, type ZipFile } from 'yauzl'

export function installExtension(extensionPath: string, extensionDirectory: string): Promise<void> {
    return openZip(extensionPath, true).then(zipfile => extractZip(zipfile, extensionDirectory))
}

function openZip(zipFile: string, lazy: boolean = false): Promise<ZipFile> {
    return new Promise<ZipFile>((resolve, reject) => {
        _openZip(zipFile, lazy ? { lazyEntries: true } : {}, (error?: Error | null, zipfile?: ZipFile) => {
            if (error || zipfile === undefined) {
                reject(error)
            } else {
                resolve(zipfile)
            }
        })
    })
}

function openZipStream(zipFile: ZipFile, entry: Entry): Promise<Readable> {
    return new Promise<Readable>((resolve, reject) => {
        zipFile.openReadStream(entry, (error?: Error | null, stream?: Readable) => {
            if (error || stream === undefined) {
                reject(error)
            } else {
                resolve(stream)
            }
        })
    })
}

function extractZip(zipfile: ZipFile, targetPath: string): Promise<void> {
    let extractedEntriesCount = 0

    return new Promise((resolve, reject) => {
        const readNextEntry = (): void => {
            extractedEntriesCount++
            zipfile.readEntry()
        }

        zipfile.once('error', reject)
        zipfile.once('close', () => {
            if (zipfile.entryCount === extractedEntriesCount) {
                resolve()
            } else {
                reject(new Error('Incomplete extraction.'))
            }
        })
        zipfile.readEntry()

        zipfile.on('entry', async (entry: Entry) => {
            const fileName = entry.fileName // .replace(options.sourcePathRegex, '')

            // directory file names end with '/'
            if (fileName.endsWith('/')) {
                const targetFileName = path.join(targetPath, fileName)
                mkdirSync(targetFileName, { recursive: true })
                readNextEntry()
                return
            }

            const stream = openZipStream(zipfile, entry)
            const mode = modeFromEntry(entry)

            await extractEntry(await stream, fileName, mode, targetPath)
            readNextEntry()
        })
    })
}

function extractEntry(stream: Readable, fileName: string, mode: number, targetPath: string): Promise<void> {
    const directoryName = path.dirname(fileName)
    const targetDirectoryName = path.join(targetPath, directoryName)
    if (!targetDirectoryName.startsWith(targetPath)) {
        return Promise.reject(new Error('invalid file'))
    }

    const targetFileName = path.join(targetPath, fileName)

    let istream: WriteStream

    mkdirSync(targetDirectoryName, { recursive: true })

    return new Promise<void>((resolve, reject) => {
        try {
            istream = createWriteStream(targetFileName, { mode })
            istream.once('close', () => resolve())
            istream.once('error', reject)
            stream.once('error', reject)
            stream.pipe(istream)
        } catch {
            reject(new Error('Unknown error'))
        }
    })
}

function modeFromEntry(entry: Entry): number {
    const attribute = entry.externalFileAttributes >> 16 || 33188

    return [448 /* S_IRWXU */, 56 /* S_IRWXG */, 7 /* S_IRWXO */]
        .map(mask => attribute & mask)
        .reduce((a, b) => a + b, attribute & 61440 /* S_IFMT */)
}
