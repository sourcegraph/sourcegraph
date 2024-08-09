export function nextResult(root: HTMLElement, direction: 'up' | 'down'): HTMLElement | undefined {
    const focusedResult = getFocusedResult()
    if (!focusedResult) {
        return undefined
    }
    for (const nextFocus of focusableSearchResults(root, direction, focusedResult)) {
        return nextFocus
    }
    return undefined
}

export function focusedResultIndex(root: HTMLElement): number | undefined {
    const focusedResult = getFocusedResult()
    if (focusedResult) {
        for (const [i, focusable] of enumerate(focusableSearchResults(root, 'down'))) {
            if (focusedResult.isEqualNode(focusable)) {
                return i
            }
        }
    }
    return undefined
}

export function nthFocusableResult(root: HTMLElement, n: number): HTMLElement | undefined {
    for (const [i, focusable] of enumerate(focusableSearchResults(root, 'down'))) {
        if (i === n) {
            return focusable
        }
    }
    return undefined
}

// A generator that iterates over all focusable search result elements in the given direction.
// Also supports starting from a specific element by providing `from`.
function* focusableSearchResults(root: HTMLElement, direction: 'up' | 'down', from?: HTMLElement) {
    const walker = document.createTreeWalker(root, NodeFilter.SHOW_ELEMENT)
    if (from) {
        walker.currentNode = from
    }
    const next = () => (direction === 'up' ? walker.previousNode() : walker.nextNode()) as HTMLElement | null
    for (let candidate = next(); candidate !== null; candidate = next()) {
        if (candidate.hasAttribute('data-focusable-search-result')) {
            yield candidate
        }
    }
}

// A helper that wraps a generator
function* enumerate<T>(iter: Iterable<T>): Generator<[number, T]> {
    let i = 0
    for (let value of iter) {
        yield [i++, value]
    }
}

function getFocusedResult(): HTMLElement | null {
    return document.activeElement &&
        document.activeElement instanceof HTMLElement &&
        document.activeElement.dataset.focusableSearchResult === 'true'
        ? document.activeElement
        : null
}
