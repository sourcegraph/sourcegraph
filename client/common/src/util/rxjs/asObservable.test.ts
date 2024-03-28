import assert from 'assert'

import { lastValueFrom, of } from 'rxjs'
import { describe, it } from 'vitest'

import { asObservable } from './asObservable'

describe('asObservable', () => {
    it('accepts an Observable', async () => {
        assert.equal(await lastValueFrom(asObservable(() => of(1))), 1)
    })
    it('accepts a sync value', async () => {
        assert.equal(await lastValueFrom(asObservable(() => 1)), 1)
    })
    it('catches errors', async () => {
        await assert.rejects(
            () =>
                lastValueFrom(
                    asObservable(() => {
                        throw new Error('test')
                    })
                ),
            /test/
        )
    })
})
