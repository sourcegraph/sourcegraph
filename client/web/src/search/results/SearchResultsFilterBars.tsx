import classNames from 'classnames'
import { debounce } from 'lodash'
import MenuLeftIcon from 'mdi-react/MenuLeftIcon'
import MenuRightIcon from 'mdi-react/MenuRightIcon'
import React, { useCallback, useLayoutEffect, useRef, useState } from 'react'
import { SearchFilters } from '../../../../shared/src/api/protocol'
import { QuickLink } from '../../schema/settings.schema'
import { FilterChip } from '../FilterChip'
import { QuickLinks } from '../QuickLinks'
import { escapeSpaces } from '../../../../shared/src/search/query/filters'

export interface DynamicSearchFilter {
    name?: string

    value: string

    count?: number
    limitHit?: boolean
}

export interface SearchResultsFilterBarsProps {
    navbarSearchQuery: string
    searchSucceeded: boolean
    resultsLimitHit: boolean
    genericFilters: DynamicSearchFilter[]
    extensionFilters: SearchFilters[] | undefined
    repoFilters?: DynamicSearchFilter[] | undefined
    quickLinks?: QuickLink[] | undefined
    onFilterClick: (value: string) => void
    onShowMoreResultsClick: (value: string) => void
    calculateShowMoreResultsCount: () => number
}

const FilterCarousel: React.FunctionComponent<{ children: JSX.Element | JSX.Element[] }> = ({ children }) => {
    const amountToScroll = 0.9 // Scroll 90% with every button press

    const filtersReference = useRef<HTMLDivElement | null>(null)

    const computeCanScrollBack = (): boolean =>
        filtersReference.current ? filtersReference.current.scrollLeft > 0 : false
    const computeCanScrollForward = (): boolean =>
        filtersReference.current
            ? filtersReference.current.scrollLeft + filtersReference.current.clientWidth <
              filtersReference.current.scrollWidth
            : false

    const [canScrollBack, setCanScrollBack] = useState(false)
    const [canScrollForward, setCanScrollForward] = useState(false)

    // WARNING for i18n/l10n: If/when we end up supporting right-to-left (eg. Arabic, Hebrew),
    // this logic has to be reversed in RTL mode, as scrollLeft will be increasingly **negative**.
    const onBackClicked = useCallback(() => {
        if (canScrollBack && filtersReference.current) {
            const width = filtersReference.current.clientWidth
            const offset = filtersReference.current.scrollLeft
            filtersReference.current.scrollTo({
                top: 0,
                left: Math.max(offset - width * amountToScroll, 0),
                behavior: 'smooth',
            })
        }
    }, [canScrollBack])
    const onForwardClicked = useCallback(() => {
        if (canScrollForward && filtersReference.current) {
            const width = filtersReference.current.clientWidth
            const offset = filtersReference.current.scrollLeft
            filtersReference.current.scrollTo({
                top: 0,
                left: Math.max(offset + width * amountToScroll, 0),
                behavior: 'smooth',
            })
        }
    }, [canScrollForward])

    useLayoutEffect(() => {
        const updateCanScroll = debounce((): void => {
            setCanScrollBack(computeCanScrollBack)
            setCanScrollForward(computeCanScrollForward)
        }, 50)

        updateCanScroll()
        const current = filtersReference.current

        current?.addEventListener('scroll', updateCanScroll)
        window.addEventListener('resize', updateCanScroll)

        return () => {
            current?.removeEventListener('scroll', updateCanScroll)
            window.removeEventListener('resize', updateCanScroll)
        }
    }, [])

    return (
        <div className="d-flex search-results-filter-bars__carousel">
            <button
                type="button"
                className={classNames('btn', 'btn-link', 'search-results-filter-bars__scroll', {
                    'search-results-filter-bars__scroll--disabled': !canScrollBack,
                })}
                onClick={onBackClicked}
            >
                <span className="sr-only">Back</span>
                <MenuLeftIcon />
            </button>
            <div className="search-results-filter-bars__filters" ref={filtersReference}>
                {children}
            </div>
            <button
                type="button"
                className={classNames('btn', 'btn-link', 'search-results-filter-bars__scroll', {
                    'search-results-filter-bars__scroll--disabled': !canScrollForward,
                })}
                onClick={onForwardClicked}
            >
                <span className="sr-only">Forward</span>
                <MenuRightIcon />
            </button>
        </div>
    )
}

export const SearchResultsFilterBars: React.FunctionComponent<SearchResultsFilterBarsProps> = ({
    navbarSearchQuery,
    searchSucceeded,
    resultsLimitHit,
    genericFilters,
    extensionFilters,
    repoFilters,
    quickLinks,
    onFilterClick,
    onShowMoreResultsClick,
    calculateShowMoreResultsCount,
}) => (
    <div className="search-results-filter-bars">
        <div className="search-results-filter-bars__row" data-testid="filters-bar">
            <span
                className={classNames('my-1', {
                    'search-results-filter-bars__row-label--empty': !(
                        (searchSucceeded && genericFilters.length > 0) ||
                        (extensionFilters && extensionFilters.length > 0)
                    ),
                })}
            >
                Filters:
            </span>
            <FilterCarousel>
                <>
                    {extensionFilters
                        ?.filter(filter => filter.value !== '')
                        .map(filter => (
                            <FilterChip
                                query={navbarSearchQuery}
                                onFilterChosen={onFilterClick}
                                key={filter.name + filter.value}
                                value={filter.value}
                                name={filter.name}
                            />
                        ))}
                    {genericFilters
                        .filter(filter => filter.value !== '')
                        .map(filter => (
                            <FilterChip
                                query={navbarSearchQuery}
                                onFilterChosen={onFilterClick}
                                key={String(filter.name) + filter.value}
                                value={filter.value}
                                name={filter.name}
                                count={filter.count}
                                limitHit={filter.limitHit}
                            />
                        ))}
                </>
            </FilterCarousel>
        </div>
        <div className="search-results-filter-bars__row" data-testid="repo-filters-bar">
            <span
                className={classNames('my-1', {
                    'search-results-filter-bars__row-label--empty': !(
                        searchSucceeded &&
                        repoFilters &&
                        repoFilters.length > 0
                    ),
                })}
            >
                Repositories:
            </span>
            <FilterCarousel>
                <>
                    {repoFilters?.map(filter => (
                        <FilterChip
                            name={filter.name}
                            query={navbarSearchQuery}
                            onFilterChosen={onFilterClick}
                            key={filter.value}
                            value={escapeSpaces(filter.value)}
                            count={filter.count}
                            limitHit={filter.limitHit}
                        />
                    ))}
                    {resultsLimitHit && !/\brepo:/.test(navbarSearchQuery) && (
                        <FilterChip
                            name="Show more"
                            query={navbarSearchQuery}
                            onFilterChosen={onShowMoreResultsClick}
                            key={`count:${calculateShowMoreResultsCount()}`}
                            value={`count:${calculateShowMoreResultsCount()}`}
                            showMore={true}
                        />
                    )}
                </>
            </FilterCarousel>
        </div>
        <QuickLinks
            quickLinks={quickLinks}
            className="search-results-filter-bars__row search-results-filter-bars__quicklinks"
        />
    </div>
)
