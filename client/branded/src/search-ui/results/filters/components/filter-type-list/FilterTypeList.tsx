import type { FC } from 'react'

import {
    mdiSourceFork,
    mdiCodeBraces,
    mdiFileOutline,
    mdiPlusMinus,
    mdiFunction,
    mdiSourceCommit,
    mdiClose,
} from '@mdi/js'
import classNames from 'classnames'

import type { Filter } from '@sourcegraph/shared/src/search/stream'
import { Button, Icon, H4, H2 } from '@sourcegraph/wildcard'

import type { URLQueryFilter } from '../../hooks'
import { FilterKind } from '../../types'
import { DynamicFilterBadge } from '../DynamicFilterBadge'

import styles from './FilterTypeList.module.scss'

interface SearchFilterTypesProps {
    backendFilters: Filter[]
    selectedFilters: URLQueryFilter[]
    onClick: (filter: URLQueryFilter, remove: boolean) => void
}

export const FilterTypeList: FC<SearchFilterTypesProps> = props => {
    const { backendFilters, selectedFilters, onClick } = props

    const defaultExhaustive = backendFilters.every(filter => filter.exhaustive)

    const mergedFilters = STATIC_TYPE_FILTERS.map(staticFilter => {
        const backendFilter = backendFilters.find(
            filter => filter.kind === FilterKind.Type && filter.label === staticFilter.label
        )
        const selectedFilter = selectedFilters.find(
            filter => filter.kind === FilterKind.Type && filter.label === staticFilter.label
        )
        const filter: Filter = {
            value: staticFilter.value,
            label: staticFilter.label,
            count: backendFilter?.count ?? 0,
            exhaustive: backendFilter ? backendFilter.exhaustive : defaultExhaustive,
            kind: staticFilter.kind,
        }
        return {
            filter,
            forceCount: selectedFilters.length === 0 && DEFAULT_SEARCH_TYPES.has(staticFilter.label),
            selected: selectedFilter !== undefined,
        }
    })

    return (
        <div className={styles.typeListContainer}>
            <H4 as={H2} className={styles.heading}>
                By type
            </H4>
            <ul className={styles.typeList}>
                {mergedFilters.map(({ filter, selected, forceCount }) => (
                    <li key={filter.value}>
                        <FilterTypeButton
                            filter={filter}
                            selected={selected}
                            onClick={onClick}
                            forceCount={forceCount}
                        />
                    </li>
                ))}
            </ul>
        </div>
    )
}

enum SearchTypeLabel {
    Code = 'Code',
    Repositories = 'Repositories',
    Paths = 'Paths',
    Symbols = 'Symbols',
    Commits = 'Commits',
    Diffs = 'Diffs',
}

const DEFAULT_SEARCH_TYPES: Set<string> = new Set([
    SearchTypeLabel.Code,
    SearchTypeLabel.Repositories,
    SearchTypeLabel.Paths,
])

export const STATIC_TYPE_FILTERS: URLQueryFilter[] = [
    { kind: 'type', label: SearchTypeLabel.Code, value: 'type:file' },
    { kind: 'type', label: SearchTypeLabel.Repositories, value: 'type:repo' },
    { kind: 'type', label: SearchTypeLabel.Paths, value: 'type:path' },
    { kind: 'type', label: SearchTypeLabel.Symbols, value: 'type:symbol' },
    { kind: 'type', label: SearchTypeLabel.Commits, value: 'type:commit' },
    { kind: 'type', label: SearchTypeLabel.Diffs, value: 'type:diff' },
]

const FILTER_TYPE_ICONS: { [key: string]: any } = {
    [SearchTypeLabel.Code]: mdiCodeBraces,
    [SearchTypeLabel.Repositories]: mdiSourceFork,
    [SearchTypeLabel.Paths]: mdiFileOutline,
    [SearchTypeLabel.Symbols]: mdiFunction,
    [SearchTypeLabel.Commits]: mdiSourceCommit,
    [SearchTypeLabel.Diffs]: mdiPlusMinus,
}

interface FilterTypeButtonProps {
    filter: Filter
    selected: boolean
    forceCount: boolean
    onClick: (filter: URLQueryFilter, remove: boolean) => void
}

const FilterTypeButton: FC<FilterTypeButtonProps> = props => {
    const { filter, selected, forceCount, onClick } = props

    return (
        <Button
            variant={selected ? 'primary' : 'secondary'}
            outline={!selected}
            className={classNames(styles.typeListItem, {
                [styles.typeListItemSelected]: selected,
            })}
            onClick={() => onClick(filter, selected)}
        >
            <Icon svgPath={FILTER_TYPE_ICONS[filter.label]} aria-hidden={true} />
            <span className={styles.typeListItemText}>{filter.label}</span>
            {(filter.count > 0 || forceCount) && (
                <DynamicFilterBadge exhaustive={filter.exhaustive} count={filter.count} />
            )}
            {selected && <Icon svgPath={mdiClose} aria-hidden={true} className="ml-1 flex-shrink-0" />}
        </Button>
    )
}
