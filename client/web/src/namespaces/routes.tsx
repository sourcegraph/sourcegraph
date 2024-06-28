import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'

import type { NamespaceAreaContext, NamespaceAreaRoute } from './NamespaceArea'

const SavedSearchArea = lazyComponent<NamespaceAreaContext, 'SavedSearchArea'>(
    () => import('../savedSearches/SavedSearchArea'),
    'SavedSearchArea'
)

export const namespaceAreaRoutes: readonly NamespaceAreaRoute[] = [
    {
        path: 'searches/*',
        render: props => <SavedSearchArea {...props} />,
        condition: () => window.context?.codeSearchEnabledOnInstance,
    },
]
