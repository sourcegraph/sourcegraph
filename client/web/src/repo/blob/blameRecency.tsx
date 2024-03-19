import { subYears } from 'date-fns'

// We use an exponential scale to get more diverse colors for more recent changes.
//
// The values are sampled from the following function:
//   y=0.005*1.7^x
const STEPS = [0.008, 0.0144, 0.0245, 0.0417, 0.0709, 0.1206, 0.2051, 0.3487, 0.5929, 1]

const COLORS = [
    'var(--blame-recency-0)',
    'var(--blame-recency-1)',
    'var(--blame-recency-2)',
    'var(--blame-recency-3)',
    'var(--blame-recency-4)',
    'var(--blame-recency-5)',
    'var(--blame-recency-6)',
    'var(--blame-recency-7)',
    'var(--blame-recency-8)',
    'var(--blame-recency-9)',
]
const ONE_YEAR_AGO = subYears(Date.now(), 1).getTime()
const THREE_YEARS_AGO = subYears(Date.now(), 3).getTime()

export function getBlameRecencyColor(commit: Date | undefined, firstCommitDate: Date | undefined): string {
    if (!commit) {
        return COLORS[0]
    }

    // We create a recency range depending on the repo creation date. If the
    // repo is newer than a year, we use the last year so that we don't have a
    // scale that is too sensible.
    const now = Date.now()
    const start = Math.min(firstCommitDate ? firstCommitDate.getTime() : THREE_YEARS_AGO, ONE_YEAR_AGO)

    // Get a value between [0, 1] that represents the recency of the commit in a linear scale
    const recency = Math.min(Math.max((now - commit.getTime()) / (now - start), 0), 1)

    // Map from the linear scale to the exponential scale
    const index = STEPS.findIndex(step => recency <= step)

    return COLORS[index]
}
