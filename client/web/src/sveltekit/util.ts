import { memoize } from 'lodash'
import { matchPath } from 'react-router-dom'

import { PageRoutes, CommunityPageRoutes } from '../routes.constants'

const allRoutes: string[] = (Object.values(PageRoutes) as string[]).concat(Object.values(CommunityPageRoutes)).filter(
    route =>
        // Remove the repository catch-all route because it matches everything
        route !== PageRoutes.RepoContainer &&
        // Remove index route because it will be a prefix of every pathname
        route !== PageRoutes.Index
)

// All the routes that are supported by SvelteKit.
// Also see `SVELTEKIT_SUPPORTED_REPO_PATHS` for the list of supported repo sub-pages.
const supportedRoutes = new Set<string>([PageRoutes.Search, PageRoutes.RepoContainer])

// All the routes that are enabled for SvelteKit. This is used for a gradual rollout of SvelteKit.
// Should be a subset of `supportedRoutes`.
// Keep in sync with 'cmd/frontend/internal/app/ui/sveltekit.go' and 'client/web-sveltekit/src/lib/navigation.ts'
const rolledoutRoutes = new Set<string>([PageRoutes.Search])

const SVELTEKIT_SUPPORTED_REPO_PATHS = /^\/.*?\/-\/(blob\/|tree\/|branches$|commit\/|commits$|stats$|tags$)/

function isRepoSubPage(href: string): boolean {
    return href.includes('/-/')
}

const getSvelteKitSupportedRoute = memoize((pathname: string): string | null => {
    if (!pathname) {
        return null
    }
    const route = allRoutes.find(
        route =>
            // Some routes in PageRoutes are not actually the exact paths passed to react router. Some are "extended"
            // in routes.tsx. For example, PageRoutes.CodyChat is used as PageRoutes.CodyChat + '/*' in routes.tsx.
            // But we cannot use routes.tsx directly here because it causes import ordering issues, specifically
            // for CSS.
            pathname.startsWith(route) || matchPath(route, pathname)
    )
    if (route && supportedRoutes.has(route)) {
        return route
    }
    // At this point we have to assume pathname is interprted as the repo container route
    // because that is the catch-all route /*.
    if (!route && (!isRepoSubPage(pathname) || SVELTEKIT_SUPPORTED_REPO_PATHS.test(pathname))) {
        return PageRoutes.RepoContainer
    }
    return null
})

/**
 * Returns true if the current route is supported (i.e. implemented) by SvelteKit.
 */
export function isSupportedRoute(pathname: string): boolean {
    return getSvelteKitSupportedRoute(pathname) !== null
}

/**
 * Returns true if the current route is enabled for SvelteKit.
 * This is used for a gradual rollout of SvelteKit.
 * This should be used togehther with the `web-next-enabled` feature flag.
 */
export function isRolledOutRoute(pathname: string): boolean {
    const route = getSvelteKitSupportedRoute(pathname)
    if (route) {
        return rolledoutRoutes.has(route)
    }
    return false
}

export function reload(): void {
    const url = new URL(window.location.href)
    url.searchParams.append('feat', 'web-next')
    window.location.href = url.toString()
}
