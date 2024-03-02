import { AuthenticatedUser } from '../auth'

import { LicenseManagerReadPermission, LicenseManagerWritePermission } from './constants'

export const canReadLicenseManagement = (user: Pick<AuthenticatedUser, 'permissions'> | null): boolean =>
    !!user &&
    user.permissions.nodes.find(permission => permission.displayName === LicenseManagerReadPermission) !== undefined

export const canWriteLicenseManagement = (user: Pick<AuthenticatedUser, 'permissions'> | null): boolean =>
    !!user &&
    user.permissions.nodes.find(permission => permission.displayName === LicenseManagerWritePermission) !== undefined
