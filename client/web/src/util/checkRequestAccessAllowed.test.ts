import { describe, expect, it } from 'vitest'

import { checkRequestAccessAllowed } from './checkRequestAccessAllowed'

describe('checkRequestAccessAllowed', () => {
    const defaultContext = {
        sourcegraphDotComMode: false,
        allowSignup: false,
    }

    it('should return false if builtin signup enabled', () => {
        expect(checkRequestAccessAllowed({ ...defaultContext, allowSignup: true })).toBe(false)
    })

    it('should return false if explicitly set enabled=false', () => {
        expect(checkRequestAccessAllowed({ ...defaultContext, authAccessRequest: { enabled: false } })).toBe(false)
    })

    it('should return true if all conditions are met', () => {
        expect(checkRequestAccessAllowed(defaultContext)).toBe(true)
    })

    it('should return true if explicitly set enabled=true', () => {
        expect(checkRequestAccessAllowed({ ...defaultContext, authAccessRequest: { enabled: true } })).toBe(true)
    })
})
