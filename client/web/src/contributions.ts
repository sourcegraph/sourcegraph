import { useEffect, useState } from 'react'

/**
 * A component that registers global contributions. It is implemented as a React component so that its
 * registrations use the React lifecycle.
 */
export function GlobalContributions(): null {
    const [error, setError] = useState<null | Error>(null)

    useEffect(() => {
        // Lazy-load `highlight/contributions.ts` to make main application bundle ~25kb Gzip smaller.
        import('@sourcegraph/common/src/util/markdown/contributions')
            .then(({ registerHighlightContributions }) => registerHighlightContributions()) // no way to unregister these
            .catch(setError)
    }, [])

    // Throw error to the <ErrorBoundary />
    if (error) {
        throw error
    }

    return null
}
