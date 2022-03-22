import { InsightFilters } from '../../../../../../core/types'

import { DrillDownFiltersFormValues } from './components/drill-down-filters-form/DrillDownFiltersForm'

export const EMPTY_DRILLDOWN_FILTERS: InsightFilters = {
    includeRepoRegexp: '',
    excludeRepoRegexp: '',
}

/**
 * Selector function from insight model filters to filter form values.
 */
export function getDrillDownFormValues(filters: InsightFilters): DrillDownFiltersFormValues {
    // Currently, we support only regexp filters so extract them in a separate object
    // to pass further in the filters form component.
    return {
        excludeRepoRegexp: filters.excludeRepoRegexp,
        includeRepoRegexp: filters.includeRepoRegexp,
    }
}
