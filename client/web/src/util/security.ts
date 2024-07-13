import { logger } from '@sourcegraph/common'

import type { SourcegraphContext } from '../jscontext'

export function validatePassword(
    context: Pick<SourcegraphContext, 'authProviders' | 'authPasswordPolicy' | 'authMinPasswordLength'>,
    password: string
): string | undefined {
    // minPasswordLen always has a value so we do it first
    if (password.length < context.authMinPasswordLength) {
        return 'Password must be at least ' + context.authMinPasswordLength.toString() + ' characters.'
    }

    const passwordPolicyReference = context.authPasswordPolicy

    if (passwordPolicyReference?.enabled) {
        if (
            passwordPolicyReference.numberOfSpecialCharacters &&
            passwordPolicyReference.numberOfSpecialCharacters > 0
        ) {
            const specialCharacters = /[!"#$%&'()*+,./:;<=>?@[\]^_`{|}~-]/
            // This must be kept in sync with the security.go checks
            const count = (password.match(specialCharacters) || []).length
            if (
                passwordPolicyReference.numberOfSpecialCharacters &&
                count < passwordPolicyReference.numberOfSpecialCharacters
            ) {
                return (
                    'Password must contain ' +
                    passwordPolicyReference.numberOfSpecialCharacters.toString() +
                    ' special character(s).'
                )
            }
        }

        if (passwordPolicyReference.requireAtLeastOneNumber) {
            const validRequireAtLeastOneNumber = /\d+/
            if (password.match(validRequireAtLeastOneNumber) === null) {
                return 'Password must contain at least one number.'
            }
        }

        if (passwordPolicyReference.requireUpperandLowerCase) {
            const validUseUpperCase = new RegExp('[A-Z]+')
            if (!validUseUpperCase.test(password)) {
                return 'Password must contain at least one uppercase letter.'
            }
        }

        return undefined
    }

    return undefined
}

export function getPasswordRequirements(
    context: Pick<SourcegraphContext, 'authProviders' | 'authPasswordPolicy' | 'authMinPasswordLength'>
): string {
    const passwordPolicyReference = context.authPasswordPolicy

    let requirements: string = 'At least ' + context.authMinPasswordLength.toString() + ' characters'

    if (passwordPolicyReference?.enabled) {
        logger.log('Using enhanced password policy.')

        if (
            passwordPolicyReference.numberOfSpecialCharacters &&
            passwordPolicyReference.numberOfSpecialCharacters > 0
        ) {
            requirements += ', ' + passwordPolicyReference.numberOfSpecialCharacters.toString() + ' special characters'
        }
        if (passwordPolicyReference.requireAtLeastOneNumber) {
            requirements += ', at least one number'
        }
        if (passwordPolicyReference.requireUpperandLowerCase) {
            requirements += ', at least one uppercase letter'
        }
    }

    return requirements
}

export const generateSecret = (): string => {
    let text = ''
    const possible = 'ABCDEF0123456789'
    for (let index = 0; index < 12; index++) {
        text += possible.charAt(Math.floor(Math.random() * possible.length))
    }
    return text
}
