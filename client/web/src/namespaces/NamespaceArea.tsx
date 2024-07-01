import type { PlatformContextProps } from '@sourcegraph/shared/src/platform/context'

import type { AuthenticatedUser } from '../auth'
import type { BatchChangesProps } from '../batches'
import type { NavItemWithIconDescriptor, RouteV6Descriptor } from '../util/contributions'

import type { PartialNamespace } from '.'

/**
 * Properties passed to all page components in the namespace area.
 */
export interface NamespaceAreaContext extends PlatformContextProps {
    namespace: PartialNamespace

    authenticatedUser: AuthenticatedUser | null
    isSourcegraphDotCom: boolean
}

export interface NamespaceAreaRoute extends RouteV6Descriptor<NamespaceAreaContext> {}

interface NavItemDescriptorContext extends BatchChangesProps {
    isSourcegraphDotCom: boolean
}

export interface NamespaceAreaNavItem extends NavItemWithIconDescriptor<NavItemDescriptorContext> {}
