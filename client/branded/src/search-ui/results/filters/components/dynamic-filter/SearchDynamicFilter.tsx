import { type FC, type ReactNode, useMemo, useRef, useState } from 'react'

import { mdiClose, mdiSourceRepository } from '@mdi/js'
import classNames from 'classnames'

import { displayRepoName } from '@sourcegraph/shared/src/components/RepoLink'
import { UserAvatar } from '@sourcegraph/shared/src/components/UserAvatar'
import type { Filter } from '@sourcegraph/shared/src/search/stream'
import { useExperimentalFeatures } from '@sourcegraph/shared/src/settings/settings'
import { SymbolKind } from '@sourcegraph/shared/src/symbols/SymbolKind'
import { Button, Icon, H2, H4, Input, LanguageIcon, Code, Tooltip } from '@sourcegraph/wildcard'

import { codeHostIcon } from '../../../../components'
import { SyntaxHighlightedSearchQuery } from '../../../../components/SyntaxHighlightedSearchQuery'
import type { URLQueryFilter } from '../../hooks'
import { FilterKind } from '../../types'
import { DynamicFilterBadge } from '../DynamicFilterBadge'

import styles from './SearchDynamicFilter.module.scss'

const DEFAULT_FILTERS_NUMBER = 5
const MAX_FILTERS_NUMBER = 10

interface SearchDynamicFilterProps {
    /** Name title of the filter section */
    title?: string

    /**
     * Specifies which type filter we want to render in this particular
     * filter section, it could be lang filter, repo filter, or file filters.
     */
    filterKind: FilterKind

    /**
     * The set of filters that are selected. This is the state that is stored
     * in the URL.
     */
    selectedFilters: URLQueryFilter[]

    /**
     * List of streamed filters from search stream API
     */
    filters?: Filter[]

    /** Exposes render API to render some custom filter item in the list */
    renderItem?: (filter: Filter, selected: boolean) => ReactNode

    /**
     * It's called whenever user changes (pick/reset) any filters in the filter panel.
     * @param nextQuery
     */
    onSelectedFilterChange: (filterKind: FilterKind, filters: URLQueryFilter[]) => void

    onAddFilterToQuery: (newFilter: string) => void
}

/**
 * Dynamic filter panel section. It renders dynamically generated filters which
 * come from the search stream API.
 */
export const SearchDynamicFilter: FC<SearchDynamicFilterProps> = ({
    title,
    filters,
    filterKind,
    selectedFilters,
    renderItem,
    onSelectedFilterChange,
    onAddFilterToQuery,
}) => {
    const inputRef = useRef<HTMLInputElement>(null)

    const [searchTerm, setSearchTerm] = useState<string>('')
    const [showMoreFilters, setShowMoreFilters] = useState<boolean>(false)

    const relevantFilters = filters?.filter(f => f.kind === filterKind) ?? []
    const relevantSelectedFilters = selectedFilters.filter(sf => sf.kind === filterKind)

    const isSelected = (filter: Filter): boolean =>
        relevantSelectedFilters.find(sf => filtersEqual(filter, sf)) !== undefined

    const mergedFilters = [
        // Selected filters come first, but we want to map them to the backend filters
        // to get the relevant count and exhaustiveness
        ...relevantSelectedFilters.map(
            sf => filters?.find(f => filtersEqual(f, sf)) ?? { ...sf, count: 0, exhaustive: true }
        ),
        // Followed by filters from the backend, but excluding the ones we
        // already listed
        ...relevantFilters.filter(f => relevantSelectedFilters.find(sf => filtersEqual(f, sf)) === undefined),
    ]

    const handleFilterClick = (filter: URLQueryFilter, remove?: boolean): void => {
        if (remove) {
            onSelectedFilterChange(
                filterKind,
                selectedFilters.filter(f => !filtersEqual(f, filter))
            )
        } else {
            onSelectedFilterChange(filterKind, [...selectedFilters, filter])
        }
    }

    const handleZeroStateButtonClick = (): void => {
        inputRef.current?.focus()
    }

    if (mergedFilters.length === 0) {
        return null
    }

    const lowerSearchTerm = searchTerm.toLowerCase()
    const filteredFilters = mergedFilters.filter(filter => filter.label.toLowerCase().includes(lowerSearchTerm))
    const filtersToShow = showMoreFilters
        ? filteredFilters.slice(0, MAX_FILTERS_NUMBER)
        : filteredFilters.slice(0, DEFAULT_FILTERS_NUMBER)

    // HACK(camdencheek): we limit the number of filters of each type to 1000, so if we get
    // exactly 1000 filters, assume that we hit that limit. Ideally, we wouldn't hard-code this
    // and the backend would tell us whether we hit that limit.
    const limitHit = filters?.some(filter => !filter.exhaustive) || filters?.length === 1000
    const suggestedQueryFilter = filterForSearchTerm(searchTerm, filterKind)

    return (
        <div className={styles.root}>
            {title && (
                <H4 as={H2} className={styles.heading}>
                    {title}
                </H4>
            )}

            {mergedFilters.length > DEFAULT_FILTERS_NUMBER && (
                <Input
                    ref={inputRef}
                    value={searchTerm}
                    placeholder={`Filter ${filterKind}`}
                    onChange={event => setSearchTerm(event.target.value)}
                />
            )}

            <ul className={styles.list}>
                {filtersToShow.map(filter => (
                    <DynamicFilterItem
                        key={filter.value}
                        filter={filter}
                        selected={isSelected(filter)}
                        renderItem={renderItem}
                        onClick={handleFilterClick}
                    />
                ))}

                {filtersToShow.length === 0 && (
                    <small className={styles.description}>
                        <div className={styles.descriptionHeader}>No matches in search results.</div>
                        {limitHit && suggestedQueryFilter ? (
                            <>
                                Try adding{' '}
                                <Button
                                    onClick={() => onAddFilterToQuery(suggestedQueryFilter)}
                                    className={styles.zeroStateQueryButton}
                                >
                                    <SyntaxHighlightedSearchQuery query={suggestedQueryFilter} />
                                </Button>{' '}
                                to your original search query to narrow results to that repo.
                            </>
                        ) : (
                            <>
                                Try expanding your search using the{' '}
                                <Button
                                    variant="link"
                                    onClick={handleZeroStateButtonClick}
                                    className={styles.zeroStateSearchButton}
                                >
                                    search bar
                                </Button>{' '}
                                above.
                            </>
                        )}
                    </small>
                )}
            </ul>
            {filteredFilters.length > DEFAULT_FILTERS_NUMBER && (
                <>
                    {showMoreFilters && filteredFilters.length > MAX_FILTERS_NUMBER && (
                        <small className={styles.description}>
                            There are {filteredFilters.length - MAX_FILTERS_NUMBER} other filters, use search to see
                            more
                        </small>
                    )}
                    <Button variant="link" size="sm" onClick={() => setShowMoreFilters(!showMoreFilters)}>
                        {showMoreFilters ? `Show less ${filterKind}s` : `Show more ${filterKind}s`}
                    </Button>
                </>
            )}
        </div>
    )
}

interface DynamicFilterItemProps {
    filter: Filter
    selected: boolean
    renderItem?: (filter: Filter, selected: boolean) => ReactNode
    onClick: (filter: URLQueryFilter, remove?: boolean) => void
}

const DynamicFilterItem: FC<DynamicFilterItemProps> = props => {
    const { filter, selected, renderItem, onClick } = props

    return (
        <li key={filter.value}>
            <Button
                variant={selected ? 'primary' : 'secondary'}
                outline={!selected}
                className={classNames(styles.item, { [styles.itemSelected]: selected })}
                onClick={() => onClick(filter, selected)}
            >
                <span className={styles.itemText}>{renderItem ? renderItem(filter, selected) : filter.label}</span>
                {/* NOTE: filter.count should _only_ be zero for the synthetic count filter. */}
                {filter.count > 0 && <DynamicFilterBadge exhaustive={filter.exhaustive} count={filter.count} />}
                {selected && <Icon svgPath={mdiClose} aria-hidden={true} className="ml-1 flex-shrink-0" />}
            </Button>
        </li>
    )
}

function filterForSearchTerm(input: string, filterKind: FilterKind): string | null {
    switch (filterKind) {
        case 'repo': {
            return `repo:${maybeQuoteString(input)}`
        }
        case 'author': {
            return `author:${maybeQuoteString(input)}`
        }
        default: {
            return null
        }
    }
}

function maybeQuoteString(input: string): string {
    if (input.match(/\s/)) {
        return `"${input.replaceAll('"', '\\"')}"`
    }
    return input
}

function filtersEqual(a: URLQueryFilter, b: URLQueryFilter): boolean {
    return a.kind === b.kind && a.label === b.label && a.value === b.value
}

export const languageFilter = (filter: Filter): ReactNode => (
    <>
        <LanguageIcon language={filter.label} className={styles.icon} />
        {filter.label}
    </>
)

export const repoFilter = (filter: Filter): ReactNode => {
    const { svgPath } = codeHostIcon(filter.label)

    return (
        <Tooltip content={filter.label} placement="right">
            <span>
                <Icon aria-hidden={true} svgPath={svgPath ?? mdiSourceRepository} /> {displayRepoName(filter.label)}
            </span>
        </Tooltip>
    )
}

export const commitDateFilter = (filter: Filter, selected: boolean): ReactNode => (
    <span className={styles.commitDate}>
        {filter.label}
        <Code className={!selected ? 'text-muted' : ''}>{filter.value}</Code>
    </span>
)

export const countAllFilter = (filter: Filter, selected: boolean): ReactNode => (
    <span className={styles.commitDate}>
        {filter.label}
        <Code className={!selected ? 'text-muted' : ''}>{filter.value}</Code>
    </span>
)

export const symbolFilter = (filter: Filter): ReactNode => {
    // eslint-disable-next-line react-hooks/rules-of-hooks
    const symbolKindTags = useExperimentalFeatures(features => features.symbolKindTags)

    // eslint-disable-next-line react-hooks/rules-of-hooks
    const symbolType = useMemo(() => {
        const parts = filter.value.split('.')
        return parts.at(-1) ?? ''
    }, [filter])

    return (
        <>
            <SymbolKind
                kind={symbolType.toUpperCase() as any}
                className={styles.icon}
                symbolKindTags={symbolKindTags}
            />
            {filter.label}
        </>
    )
}

export const utilityFilter = (filter: Filter): string => (filter.count === 0 ? filter.value : filter.label)

export const authorFilter = (filter: Filter): ReactNode => (
    <>
        <UserAvatar size={14} user={{ avatarURL: null, displayName: filter.label }} className={styles.avatar} />
        {filter.label}
    </>
)
