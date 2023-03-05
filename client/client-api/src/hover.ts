import { MarkupKind, Range } from '@sourcegraph/extension-api-classes'
import { Badged, Hover, MarkupContent, AggregableBadge, Range as PlainRange } from '@sourcegraph/extension-api-types'

/** A hover that is merged from multiple Hover results and normalized. */
export interface HoverMerged {
    contents: Badged<MarkupContent>[]
    range?: PlainRange

    /** Sorted and de-duplicated set of badges in all source hover values. */
    aggregatedBadges?: AggregableBadge[]
}

/** Create a merged hover from the given individual hovers. */
export function fromHoverMerged(values: (Badged<Hover> | null | undefined)[]): HoverMerged | null {
    const contents: HoverMerged['contents'] = []
    const aggregatedBadges = new Map<string, AggregableBadge>()
    let range: PlainRange | undefined
    for (const result of values) {
        if (result) {
            if (result.contents?.value) {
                contents.push({
                    value: result.contents.value,
                    kind: result.contents.kind || MarkupKind.PlainText,
                })
            }

            for (const badge of result.aggregableBadges || []) {
                aggregatedBadges.set(badge.text, badge)
            }

            if (result.range && !range) {
                // TODO(tj): Merge ranges so we highlight all provided ranges, not just the first range
                if (result.range instanceof Range) {
                    range = result.range.toPlain()
                } else {
                    range = result.range
                }
            }
        }
    }

    if (contents.length === 0) {
        return null
    }

    return {
        contents,
        ...(range ? { range } : {}),
        aggregatedBadges: [...aggregatedBadges.values()].sort((a, b) => a.text.localeCompare(b.text)),
    }
}
