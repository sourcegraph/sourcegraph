import * as zlib from 'mz/zlib'
import { Edge, Vertex } from 'lsif-protocol'
import { LineStream } from 'byline'
import { Readable, Transform, Writable } from 'stream'
import { ValidateFunction } from 'ajv'

/**
 * Pipes the input data into the output stream. All content of the input
 * stream will be written to the output stream if there are no validation
 * errors. The input is assumed to be gzipped. If the validator is
 * undefined, no validation will occur.
 *
 * @param input The input stream.
 * @param output The output stream.
 * @param validator The JSON schema validation function to apply.
 */
export function validateLsifInput(
    input: Readable,
    output: Writable,
    validator: ValidateFunction | undefined
): Promise<void> {
    if (!validator) {
        // If we're not validating, just pipe input to the temp file
        return new Promise((resolve, reject) => {
            input
                .pipe(output)
                .on('error', reject)
                .on('finish', resolve)
        })
    }

    let line = 0
    const transform = new Transform({
        objectMode: true,
        transform: (data: Buffer, _: string, cb: (error: Error | null, data?: string) => void): void => {
            line++
            if (data.length === 0) {
                return cb(null, '\n')
            }

            try {
                if (!validator(JSON.parse(data.toString())) && validator.errors) {
                    throw new Error(validator.errors.map(e => e.message).join(', '))
                }
            } catch (e) {
                return cb(new Error(`Failed to validate line #${line} (${data}): ${e && e.message}`))
            }

            return cb(null, `${data}\n`)
        },
    })

    return new Promise((resolve, reject) => {
        const lineStream = new LineStream({
            keepEmptyLines: true,
        })

        input
            .pipe(zlib.createGunzip())
            .on('error', reject)
            .pipe(lineStream)

        lineStream
            .pipe(transform)
            .on('error', reject)
            .pipe(zlib.createGzip())
            .on('error', reject)
            .pipe(output)
            .on('error', reject)
            .on('finish', resolve)
    })
}

/**
 * Apply a function to each element of the input stream. The input is
 * assumed to be gzipped.
 *
 * @param input The input stream.
 * @param process The function to apply to element of the input stream.
 */
export function processLsifInput(input: Readable, process: (element: Vertex | Edge) => void): Promise<void> {
    let line = 0
    const output = new Writable({
        objectMode: true,
        write: (data: Buffer, _: string, cb: (error?: Error | null) => void): void => {
            line++
            if (data.length === 0) {
                return cb()
            }

            try {
                process(JSON.parse(data.toString()))
            } catch (e) {
                return cb(new Error(`Failed to process line #${line} (${data}): ${e && e.message} `))
            }

            return cb()
        },
    })

    return new Promise((resolve, reject) => {
        const lineStream = new LineStream({
            keepEmptyLines: true,
        })

        input
            .pipe(zlib.createGunzip())
            .on('error', reject)
            .pipe(lineStream)

        lineStream
            .pipe(output)
            .on('error', reject)
            .on('finish', resolve)
    })
}
