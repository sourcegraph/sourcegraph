import type { Decorator, Meta } from '@storybook/react'

import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { BrandedStory } from '@sourcegraph/wildcard/src/stories'

import { NewSearchFilters } from './NewSearchFilters'

const decorator: Decorator = story => <BrandedStory>{props => story()}</BrandedStory>

const config: Meta = {
    title: 'branded/search-ui/filters',
    decorators: [decorator],
    parameters: {
        chromatic: { viewports: [575, 700] },
    },
}

export default config

export const FiltersStore = () => (
    <NewSearchFilters
        query=""
        filters={[]}
        onQueryChange={() => {}}
        withCountAllFilter={false}
        isFilterLoadingComplete={false}
        telemetryService={NOOP_TELEMETRY_SERVICE}
    />
)
