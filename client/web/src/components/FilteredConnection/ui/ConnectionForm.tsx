import React, { useCallback, useRef } from 'react'

import classNames from 'classnames'
import { useMergeRefs } from 'use-callback-ref'

import { Form, Input, useAutoFocus } from '@sourcegraph/wildcard'

import { FilterControl, type FilteredConnectionFilter, type FilteredConnectionFilterValue } from '../FilterControl'

import styles from './ConnectionForm.module.scss'

export interface ConnectionFormProps {
    /** Hides the search input field. */
    hideSearch?: boolean

    /** Shows the search input field before the filter controls */
    showSearchFirst?: boolean

    /** CSS class name for the <input> element */
    inputClassName?: string

    /** CSS class name for the <form> element */
    formClassName?: string

    /** Placeholder text for the <input> element */
    inputPlaceholder?: string

    /** Value of the <input> element */
    inputValue?: string

    /** aria-label for the <input> element */
    inputAriaLabel?: string

    /** Called when the <input> element value changes */
    onInputChange?: React.ChangeEventHandler<HTMLInputElement>

    /** Autofocuses the filter input field. */
    autoFocus?: boolean

    /**
     * Filters to display next to the filter input field.
     *
     * Filters are mutually exclusive.
     */
    filters?: FilteredConnectionFilter[]

    onFilterSelect?: (filter: FilteredConnectionFilter, value: FilteredConnectionFilterValue['value'] | null) => void

    /** An element rendered as a sibling of the filters. */
    additionalFilterElement?: React.ReactElement

    filterValues?: Record<string, FilteredConnectionFilterValue['value'] | null>

    compact?: boolean
}

/**
 * FilteredConnection form input.
 * Supports <input> for querying and <select>/<radio> controls for filtering
 */
export const ConnectionForm = React.forwardRef<HTMLInputElement, ConnectionFormProps>(
    (
        {
            hideSearch,
            showSearchFirst,
            formClassName,
            inputClassName,
            inputPlaceholder,
            inputAriaLabel,
            inputValue,
            onInputChange,
            autoFocus,
            filters,
            onFilterSelect,
            filterValues,
            additionalFilterElement,
            compact,
        },
        reference
    ) => {
        const localReference = useRef<HTMLInputElement>(null)
        const mergedReference = useMergeRefs([localReference, reference])
        const handleSubmit = useCallback<React.FormEventHandler<HTMLFormElement>>(event => {
            // Do nothing. The <input onChange> handler will pick up any changes shortly.
            event.preventDefault()
        }, [])

        useAutoFocus({ autoFocus, reference: localReference })

        const searchControl = !hideSearch && (
            <Input
                className={classNames(styles.input, inputClassName)}
                type="search"
                placeholder={inputPlaceholder}
                name="query"
                value={inputValue}
                onChange={onInputChange}
                autoFocus={autoFocus}
                autoComplete="off"
                autoCorrect="off"
                autoCapitalize="off"
                ref={mergedReference}
                spellCheck={false}
                aria-label={inputAriaLabel}
                variant={compact ? 'small' : 'regular'}
            />
        )

        return (
            <Form
                className={classNames(styles.form, !compact && styles.noncompact, formClassName)}
                onSubmit={handleSubmit}
            >
                {showSearchFirst && searchControl}
                {filters && onFilterSelect && filterValues && (
                    <FilterControl filters={filters} onValueSelect={onFilterSelect} values={filterValues}>
                        {additionalFilterElement}
                    </FilterControl>
                )}
                {!showSearchFirst && searchControl}
            </Form>
        )
    }
)
ConnectionForm.displayName = 'ConnectionForm'
