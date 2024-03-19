import type { FC } from 'react'

import create from 'zustand'

import { NewSearchFilters, useUrlFilters } from '@sourcegraph/branded'
import { DeleteIcon } from '@sourcegraph/branded/src/search-ui/results/filters/components/Icons'
import type { Filter } from '@sourcegraph/shared/src/search/stream'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Badge, Button, Icon, Modal, Panel, useWindowSize } from '@sourcegraph/wildcard'

import styles from './SearchFiltersPanel.module.scss'

interface SearchFiltersStore {
    isOpen: boolean
    setFiltersPanel: (open: boolean) => void
}

export const useSearchFiltersStore = create<SearchFiltersStore>(set => ({
    isOpen: false,
    setFiltersPanel: (open: boolean) => set({ isOpen: open }),
}))

export interface SearchFiltersPanelProps extends TelemetryProps {
    query: string
    filters: Filter[] | undefined
    withCountAllFilter: boolean
    isFilterLoadingComplete: boolean
    className?: string
    onQueryChange: (nextQuery: string, updatedSearchURLQuery?: string) => void
}

/**
 * Search result page filters sidebar, this components renders
 * filters panel in different UI mode, sidebar for desktop and modal-like
 * UI for tablets and mobile layout.
 *
 * NOTE: This is a specific component to search result page, do not reuse it
 * as it is, use consumer agnostic NewSearchFilters component instead.
 */
export const SearchFiltersPanel: FC<SearchFiltersPanelProps> = props => {
    const { query, filters, withCountAllFilter, isFilterLoadingComplete, className, onQueryChange, telemetryService } =
        props

    const { isOpen, setFiltersPanel } = useSearchFiltersStore()
    const uiMode = useSearchFiltersPanelUIMode()

    if (uiMode === SearchFiltersPanelUIMode.Sidebar) {
        return (
            <Panel
                defaultSize={300}
                minSize={240}
                position="left"
                storageKey="filter-sidebar"
                ariaLabel="Filters sidebar"
                className={className}
            >
                <NewSearchFilters
                    query={query}
                    filters={filters}
                    withCountAllFilter={withCountAllFilter}
                    isFilterLoadingComplete={isFilterLoadingComplete}
                    onQueryChange={onQueryChange}
                    telemetryService={telemetryService}
                />
            </Panel>
        )
    }

    return (
        <Modal
            isOpen={isOpen}
            aria-label="Filters modal"
            className={styles.modal}
            onDismiss={() => setFiltersPanel(false)}
        >
            <NewSearchFilters
                query={query}
                filters={filters}
                withCountAllFilter={withCountAllFilter}
                isFilterLoadingComplete={isFilterLoadingComplete}
                onQueryChange={onQueryChange}
                telemetryService={telemetryService}
            >
                <Button variant="secondary" outline={true} onClick={() => setFiltersPanel(false)}>
                    <Icon as={DeleteIcon} width={14} height={14} aria-hidden={true} className={styles.closeIcon} />{' '}
                    Close filters
                </Button>
            </NewSearchFilters>
        </Modal>
    )
}

export enum SearchFiltersPanelUIMode {
    Sidebar = 'sidebar',
    Modal = 'modal',
}

export function useSearchFiltersPanelUIMode(): SearchFiltersPanelUIMode {
    const { width } = useWindowSize()

    // Hardcoded media query value in order to switch between desktop and mobile
    // filter UI versions
    const hasTabletLayout = width <= 992

    return hasTabletLayout ? SearchFiltersPanelUIMode.Modal : SearchFiltersPanelUIMode.Sidebar
}

export const SearchFiltersTabletButton: FC = props => {
    const mode = useSearchFiltersPanelUIMode()
    const [urlFilters] = useUrlFilters()
    const { setFiltersPanel } = useSearchFiltersStore()

    // There is no point to render action filter button when we're in
    // sidebar mode since filters are always visible in this mode
    // Render it only when we're in tablet/mobile layout
    if (mode === SearchFiltersPanelUIMode.Sidebar) {
        return null
    }

    return (
        <Button variant="secondary" outline={true} size="sm" onClick={() => setFiltersPanel(true)}>
            Filters{' '}
            {urlFilters.length > 0 && (
                <Badge small={true} variant="primary" className="ml-1">
                    {urlFilters.length}
                </Badge>
            )}
        </Button>
    )
}
