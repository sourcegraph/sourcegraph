import React, { useCallback } from 'react'

import { Select, Text } from '@sourcegraph/wildcard'

import { RadioButtons } from '../RadioButtons'

import styles from './FilterControl.module.scss'

/**
 * A filter to display next to the search input field.
 */
export interface Filter {
    /** The UI label for the filter. */
    label: string

    /** The UI form control to use when displaying this filter. */
    type: 'radio' | 'select'

    /**
     * The URL query parameter name for this filter (conventionally the label, lowercased and
     * without spaces and punctuation).
     */
    id: string

    /** An optional tooltip to display for this filter. */
    tooltip?: string

    /**
     * All of the possible values for this filter that the user can select.
     */
    options: FilterOption[]
}

/**
 * An option that the user can select for a filter ({@link Filter}).
 */
export interface FilterOption {
    /**
     * The value (corresponding to the key in {@link Filter.id}) if this option is chosen. For
     * example, if a filter has {@link Filter.id} of `sort` and the user selects a
     * {@link FilterOption} with {@link FilterOption.value} of `asc`, then the URL query string
     * would be `sort=asc`.
     */
    value: string
    label: string
    tooltip?: string
    args: { [name: string]: string | number | boolean }
}

/**
 * The values of all filters, keyed by the filter ID ({@link Filter.id}).
 */
export interface FilterValues extends Record<string, FilterOption['value'] | null> {}

interface FilterControlProps {
    /** All filters. */
    filters: Filter[]

    /** Called when a filter is selected. */
    onValueSelect: (filter: Filter, value: FilterOption['value']) => void

    values: FilterValues
}

export const FilterControl: React.FunctionComponent<React.PropsWithChildren<FilterControlProps>> = ({
    filters,
    values,
    onValueSelect,
    children,
}) => {
    const onChange = useCallback(
        (filter: Filter, id: string) => {
            const value = filter.options.find(opt => opt.value === id)
            if (value === undefined) {
                return
            }
            onValueSelect(filter, value.value)
        },
        [onValueSelect]
    )

    return (
        <div className={styles.filterControl}>
            {filters.map(filter => {
                if (filter.type === 'radio') {
                    return (
                        <RadioButtons
                            key={filter.id}
                            name={filter.id}
                            className="d-inline-flex flex-row"
                            selected={values[filter.id] ?? undefined}
                            nodes={filter.options.map(({ value, label, tooltip }) => ({
                                tooltip,
                                label,
                                id: value,
                            }))}
                            onChange={event => onChange(filter, event.currentTarget.value)}
                        />
                    )
                }

                if (filter.type === 'select') {
                    const filterLabelId = `filtered-select-label-${filter.id}`
                    return (
                        <div key={filter.id} className="d-inline-flex flex-row align-center flex-wrap">
                            <div className="d-inline-flex flex-row align-items-baseline">
                                <Text className="text-xl-center text-nowrap mr-2 mb-0" id={filterLabelId}>
                                    {filter.label}:
                                </Text>
                                <Select
                                    aria-labelledby={filterLabelId}
                                    id=""
                                    name={filter.id}
                                    onChange={event => onChange(filter, event.currentTarget.value)}
                                    value={values[filter.id] ?? undefined}
                                    className="mb-0"
                                    isCustomStyle={true}
                                >
                                    {filter.options.map(opt => (
                                        <option key={opt.value} value={opt.value} label={opt.label} />
                                    ))}
                                </Select>
                            </div>
                        </div>
                    )
                }

                return null
            })}
            {children}
        </div>
    )
}
