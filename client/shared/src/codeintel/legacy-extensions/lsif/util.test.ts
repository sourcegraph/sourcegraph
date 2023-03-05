import sinon from 'sinon'

import * as scip from '../../scip'
import * as sourcegraph from '../api'

import { GenericLSIFResponse } from './api'

export const document: sourcegraph.TextDocument = {
    uri: 'git://repo?rev#foo.ts',
    languageId: 'typescript',
    text: undefined,
}

export const makeResource = (
    name: string,
    oid: string,
    path: string
): { repository: { name: string }; commit: { oid: string }; path: string } => ({
    repository: { name },
    commit: { oid },
    path,
})

export const position = new scip.Position(5, 10)
export const range1 = scip.Range.fromNumbers(1, 2, 3, 4)
export const range2 = scip.Range.fromNumbers(2, 3, 4, 5)
export const range3 = scip.Range.fromNumbers(3, 4, 5, 6)
export const range4 = scip.Range.fromNumbers(4, 5, 6, 7)
export const range5 = scip.Range.fromNumbers(5, 6, 7, 8)
export const range6 = scip.Range.fromNumbers(6, 7, 8, 9)

// eslint-disable-next-line import/no-named-as-default-member
export const stencil1 = sinon.stub().callsFake(() =>
    makeEnvelope({
        stencil: [new scip.Range(position, new scip.Position(position.line, position.character + 1))],
    })
)

export const resource0 = makeResource('repo', 'rev', 'foo.ts')
export const resource1 = makeResource('repo1', 'deadbeef1', 'a.ts')
export const resource2 = makeResource('repo2', 'deadbeef2', 'b.ts')
export const resource3 = makeResource('repo3', 'deadbeef3', 'c.ts')

export const makeEnvelope = <R>(value: R | null = null): Promise<GenericLSIFResponse<R | null>> =>
    Promise.resolve({
        repository: {
            commit: {
                blob: {
                    lsif: value,
                },
            },
        },
    })

export async function gatherValues<T>(generator: AsyncGenerator<T>): Promise<T[]> {
    const values: T[] = []
    for await (const value of generator) {
        values.push(value)
    }
    return values
}

test('util.test.ts', () => {})
