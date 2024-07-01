import { lazyComponent } from '@sourcegraph/shared/src/util/lazyComponent'

import type { NamespaceAreaRoute } from './NamespaceArea'

const SavedSearchArea = lazyComponent(() => import('../savedSearches/Area'), 'Area')
const WorkflowArea = lazyComponent(() => import('../workflows/WorkflowArea'), 'WorkflowArea')

export const namespaceAreaRoutes: readonly NamespaceAreaRoute[] = [
    {
        path: 'searches/*',
        render: props => <SavedSearchArea {...props} />,
        condition: () => window.context?.codeSearchEnabledOnInstance,
    },
    {
        path: 'workflows/*',
        render: props => <WorkflowArea {...props} />,
        condition: () => window.context?.codyEnabledForCurrentUser,
    },
]
