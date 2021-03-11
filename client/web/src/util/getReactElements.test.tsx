import React from 'react'
import { getReactElements } from './getReactElements'

describe('util/getReactElements', () => {
    it('returns only React-Elements from the provided array', () => {
        const reactElement1 = <div>element1</div>
        const reactElement2 = <div>element2</div>

        const reactElements = getReactElements([
            reactElement1,
            reactElement2,
            null,
            {},
            undefined,
            false && reactElement2,
        ])

        expect(reactElements).toEqual([reactElement1, reactElement2])
    })
})
