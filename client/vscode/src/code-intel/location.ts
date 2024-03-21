import type { Range } from '@sourcegraph/extension-api-types'

export interface LocationNode {
    resource: {
        path: string
        repositoryName: string
        revision: string
    }
    range?: Range
}
