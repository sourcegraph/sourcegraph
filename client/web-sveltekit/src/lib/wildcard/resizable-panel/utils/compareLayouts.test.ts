import { describe, it, expect } from 'vitest'

import { compareLayouts } from './compareLayouts'

describe('compareLayouts', () => {
    it('should work', () => {
        expect(compareLayouts([1, 2], [1])).toBe(false)
        expect(compareLayouts([1], [1, 2])).toBe(false)
        expect(compareLayouts([1, 2, 3], [1, 2, 3])).toBe(true)
    })
})
