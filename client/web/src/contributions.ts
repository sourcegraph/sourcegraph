import { useEffect } from 'react'

/**
 * A React hook that registers global contributions.
 */
export const useGlobalContributions = (): void => {
    useEffect(() => {
        // Lazy-load `highlight/contributions.ts` to make main application bundle ~25kb Gzip smaller.
        import('@sourcegraph/common/src/util/markdown/contributions')
            .then(({ registerHighlightContributions }) => registerHighlightContributions()) // no way to unregister these
            .catch(error => {
                throw error // Throw error to the <ErrorBoundary />
            })
    }, [])
}
