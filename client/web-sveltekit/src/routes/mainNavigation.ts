import { mdiChartBar, mdiMagnify } from '@mdi/js'
import type { Page } from '@sveltejs/kit'
import type { ComponentType } from 'svelte'

import BatchChangesIcon from '$lib/icons/BatchChanges.svelte'
import CodyIcon from '$lib/icons/Cody.svelte'
import { isRepoRoute } from '$lib/navigation'

/**
 * Indiciates to the UI to show a status badge next to the navigation entry.
 */
export enum Status {
    BETA = 1,
}

/**
 * A navigation entry is a single item in the navigation menu
 * It is a link to a specific page in the application.
 */
export interface NavigationEntry {
    /**
     * The label of the navigation entry.
     */
    label: string
    /**
     * The target URL to navigate to.
     */
    href: string
    /**
     * An optional icon to display next to the label.
     */
    icon?: string | ComponentType
    /**
     * An optional status to display next to the label. See {@link Status}.
     */
    status?: Status
}

/**
 * A navigation menu is a collection of navigation entries.
 * Currently it will be rendered as a dropdown in the navigation bar.
 */
export interface NavigationMenu {
    /**
     * The label of the navigation menu.
     */
    label: string
    /**
     * The navigation entries that are part of the menu.
     */
    children: NavigationEntry[]
    /**
     * An optional icon to display next to the label.
     */
    icon?: string | ComponentType
    /**
     * A function to determine if current page is part of the menu.
     * This is used to mark the menu as "current" in the UI.
     */
    isCurrent(page: Page): boolean
}

/**
 * A function to determine if a navigation entry is asoociated with the current page,
 * by means of comparing the entry's href with the current page's URL.
 */
export function isCurrent(entry: NavigationEntry, page: Page): boolean {
    return page.url.pathname.startsWith(entry.href)
}

/**
 * The main navigation of the application.
 */
export const mainNavigation: (NavigationMenu | NavigationEntry)[] = [
    {
        label: 'Code Search',
        icon: mdiMagnify,
        children: [
            {
                label: 'Search Home',
                href: '/search',
            },
            {
                label: 'Contexts',
                href: '/contexts',
            },
            {
                label: 'Notebooks',
                href: '/notebooks',
            },
            {
                label: 'Monitoring',
                href: '/code-monitoring',
            },
            {
                label: 'Code Ownership',
                href: '/own',
            },
            {
                label: 'Search Jobs',
                href: '/search-jobs',
                status: Status.BETA,
            },
        ],
        isCurrent(this: NavigationMenu, page) {
            // This is a special case of the code search menu: It is marked as "current" if the
            // current page is a repository route.
            return isRepoRoute(page.route?.id) || this.children.some(entry => isCurrent(entry, page))
        },
    },
    {
        label: 'Cody AI',
        icon: CodyIcon,
        isCurrent(this: NavigationMenu, page) {
            return this.children.some(entry => isCurrent(entry, page))
        },
        children: [
            {
                label: 'Dashboard',
                href: '/cody',
            },
            {
                label: 'Web Chat',
                href: '/cody/chat',
            },
        ],
    },
    {
        label: 'Batch Changes',
        icon: BatchChangesIcon,
        href: '/batch-changes',
    },
    {
        label: 'Insights',
        icon: mdiChartBar,
        href: '/insights',
    },
]
