import type { SourcegraphContext } from '../jscontext'

/**
 * @returns whether the access request feature is allowed or not
 */
export function checkRequestAccessAllowed({
    allowSignup,
    authAccessRequest,
}: Pick<SourcegraphContext, 'allowSignup' | 'authAccessRequest'>): boolean {
    return !allowSignup && authAccessRequest?.enabled !== false
}
