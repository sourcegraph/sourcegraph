import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'

import type { NamespaceAreaRoute } from './NamespaceArea'

const SavedSearchArea = lazyComponent(() => import('../savedSearches/Area'), 'Area')

export const namespaceAreaRoutes: readonly NamespaceAreaRoute[] = [
    {
        path: 'searches/*',
        render: props => <SavedSearchArea {...props} />,
        condition: () => window.context?.codeSearchEnabledOnInstance,
    },
]
