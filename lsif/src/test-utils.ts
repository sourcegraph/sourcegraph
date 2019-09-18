import winston from 'winston'
import { lsp } from 'lsif-protocol'

export function createSilentLogger(): winston.Logger {
    const transport = new winston.transports.Console({})
    transport.silent = true
    return winston.createLogger({ level: 'debug', transports: [transport] })
}

export function createLocation(
    uri: string,
    startLine: number,
    startCharacter: number,
    endLine: number,
    endCharacter: number
): lsp.Location {
    return lsp.Location.create(uri, {
        start: { line: startLine, character: startCharacter },
        end: { line: endLine, character: endCharacter },
    })
}

export function createRemoteLocation(
    repository: string,
    path: string,
    startLine: number,
    startCharacter: number,
    endLine: number,
    endCharacter: number
): lsp.Location {
    const url = new URL(`git://${repository}`)
    url.search = createCommit(repository)
    url.hash = path

    return createLocation(url.href, startLine, startCharacter, endLine, endCharacter)
}

export function createCommit(repository: string): string {
    return repository.repeat(40).substring(0, 40)
}
