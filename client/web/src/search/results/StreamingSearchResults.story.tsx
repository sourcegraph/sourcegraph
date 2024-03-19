import type { Decorator, Meta, StoryFn } from '@storybook/react'
import { createBrowserHistory } from 'history'
import { EMPTY, NEVER, of } from 'rxjs'

import { SearchQueryStateStoreProvider } from '@sourcegraph/shared/src/search'
import type { AggregateStreamingSearchResults } from '@sourcegraph/shared/src/search/stream'
import { NOOP_TELEMETRY_SERVICE } from '@sourcegraph/shared/src/telemetry/telemetryService'
import {
    HIGHLIGHTED_FILE_LINES_LONG_REQUEST,
    MULTIPLE_SEARCH_RESULT,
    REPO_MATCH_RESULTS_WITH_METADATA,
} from '@sourcegraph/shared/src/testing/searchTestHelpers'

import type { AuthenticatedUser } from '../../auth'
import { WebStory } from '../../components/WebStory'
import { useNavbarQueryState } from '../../stores'

import { StreamingSearchResults, type StreamingSearchResultsProps } from './StreamingSearchResults'

const history = createBrowserHistory()
history.replace({ search: 'q=r:golang/oauth2+test+f:travis' })

const streamingSearchResult: AggregateStreamingSearchResults = {
    state: 'complete',
    results: [...MULTIPLE_SEARCH_RESULT.results, ...REPO_MATCH_RESULTS_WITH_METADATA],
    filters: MULTIPLE_SEARCH_RESULT.filters,
    progress: {
        durationMs: 500,
        matchCount: MULTIPLE_SEARCH_RESULT.progress.matchCount,
        skipped: [],
    },
}

const defaultProps: StreamingSearchResultsProps = {
    telemetryService: NOOP_TELEMETRY_SERVICE,

    authenticatedUser: {
        url: '/users/alice',
        displayName: 'Alice',
        username: 'alice',
        emails: [{ email: 'alice@email.test', isPrimary: true, verified: true }],
    } as AuthenticatedUser,

    settingsCascade: {
        subjects: null,
        final: null,
    },
    platformContext: { settings: NEVER, requestGraphQL: () => EMPTY, sourcegraphURL: 'https://sourcegraph.com' } as any,

    streamSearch: () => of(streamingSearchResult),

    fetchHighlightedFileLineRanges: HIGHLIGHTED_FILE_LINES_LONG_REQUEST,
    isSourcegraphDotCom: false,
    searchContextsEnabled: true,
    searchAggregationEnabled: true,
    codeMonitoringEnabled: true,
    ownEnabled: true,
}

const decorator: Decorator = Story => {
    useNavbarQueryState.setState({ searchQueryFromURL: 'r:golang/oauth2 test f:travis' })
    return <Story />
}

const config: Meta = {
    title: 'web/search/results/StreamingSearchResults',
    decorators: [decorator],
    parameters: {
        chromatic: { viewports: [577, 769, 993], disableSnapshot: false },
    },
}

export default config

export const StandardRender: StoryFn = () => (
    <WebStory>
        {() => (
            <SearchQueryStateStoreProvider useSearchQueryState={useNavbarQueryState}>
                <StreamingSearchResults {...defaultProps} />
            </SearchQueryStateStoreProvider>
        )}
    </WebStory>
)

StandardRender.storyName = 'standard render'

export const UnauthenticatedUserStandardRender: StoryFn = () => (
    <WebStory>
        {() => (
            <SearchQueryStateStoreProvider useSearchQueryState={useNavbarQueryState}>
                <StreamingSearchResults {...defaultProps} authenticatedUser={null} />
            </SearchQueryStateStoreProvider>
        )}
    </WebStory>
)

UnauthenticatedUserStandardRender.storyName = 'unauthenticated user standard render'

export const NoResults: StoryFn = () => {
    const result: AggregateStreamingSearchResults = {
        state: 'complete',
        results: [],
        filters: [],
        progress: {
            durationMs: 500,
            matchCount: 0,
            skipped: [],
        },
    }

    return (
        <WebStory>
            {() => (
                <SearchQueryStateStoreProvider useSearchQueryState={useNavbarQueryState}>
                    <StreamingSearchResults {...defaultProps} streamSearch={() => of(result)} />
                </SearchQueryStateStoreProvider>
            )}
        </WebStory>
    )
}

NoResults.storyName = 'no results'

export const DidYouMean: StoryFn = () => {
    useNavbarQueryState.setState({ searchQueryFromURL: 'javascript test' })

    return (
        <WebStory>
            {() => (
                <SearchQueryStateStoreProvider useSearchQueryState={useNavbarQueryState}>
                    <StreamingSearchResults {...defaultProps} />
                </SearchQueryStateStoreProvider>
            )}
        </WebStory>
    )
}

DidYouMean.storyName = 'did you mean'

export const ProgressWithWarning: StoryFn = () => {
    const result: AggregateStreamingSearchResults = {
        state: 'complete',
        results: MULTIPLE_SEARCH_RESULT.results,
        filters: MULTIPLE_SEARCH_RESULT.filters,
        progress: {
            durationMs: 500,
            matchCount: MULTIPLE_SEARCH_RESULT.progress.matchCount,
            skipped: [
                {
                    reason: 'excluded-fork',
                    message: '10k forked repositories excluded',
                    severity: 'info',
                    title: '10k forked repositories excluded',
                    suggested: {
                        title: 'forked:yes',
                        queryExpression: 'forked:yes',
                    },
                },
                {
                    reason: 'excluded-archive',
                    message: '60k archived repositories excluded',
                    severity: 'info',
                    title: '60k archived repositories excluded',
                    suggested: {
                        title: 'archived:yes',
                        queryExpression: 'archived:yes',
                    },
                },
                {
                    reason: 'shard-timedout',
                    message: 'Search timed out',
                    severity: 'warn',
                    title: 'Search timed out',
                    suggested: {
                        title: 'timeout:2m',
                        queryExpression: 'timeout:2m',
                    },
                },
            ],
        },
    }

    return (
        <WebStory>
            {() => (
                <SearchQueryStateStoreProvider useSearchQueryState={useNavbarQueryState}>
                    <StreamingSearchResults {...defaultProps} streamSearch={() => of(result)} />
                </SearchQueryStateStoreProvider>
            )}
        </WebStory>
    )
}

ProgressWithWarning.storyName = 'progress with warnings'

export const LoadingWithNoResults: StoryFn = () => (
    <WebStory>
        {() => (
            <SearchQueryStateStoreProvider useSearchQueryState={useNavbarQueryState}>
                <StreamingSearchResults {...defaultProps} streamSearch={() => NEVER} />
            </SearchQueryStateStoreProvider>
        )}
    </WebStory>
)

LoadingWithNoResults.storyName = 'loading with no results'

export const LoadingWithSomeResults: StoryFn = () => {
    const result: AggregateStreamingSearchResults = {
        state: 'loading',
        results: MULTIPLE_SEARCH_RESULT.results,
        filters: MULTIPLE_SEARCH_RESULT.filters,
        progress: {
            durationMs: 500,
            matchCount: MULTIPLE_SEARCH_RESULT.progress.matchCount,
            skipped: [],
        },
    }

    return (
        <WebStory>
            {() => (
                <SearchQueryStateStoreProvider useSearchQueryState={useNavbarQueryState}>
                    <StreamingSearchResults {...defaultProps} streamSearch={() => of(result)} />
                </SearchQueryStateStoreProvider>
            )}
        </WebStory>
    )
}

LoadingWithSomeResults.storyName = 'loading with some results'

export const ServerSideAlert: StoryFn = () => {
    const result: AggregateStreamingSearchResults = {
        state: 'complete',
        results: MULTIPLE_SEARCH_RESULT.results,
        filters: MULTIPLE_SEARCH_RESULT.filters,
        progress: {
            durationMs: 500,
            matchCount: MULTIPLE_SEARCH_RESULT.progress.matchCount,
            skipped: [],
        },
        alert: {
            proposedQueries: [{ query: 'test', description: 'new query' }],
            title: 'Test alert',
            description: 'This is an alert',
        },
    }

    return (
        <WebStory>
            {() => (
                <SearchQueryStateStoreProvider useSearchQueryState={useNavbarQueryState}>
                    <StreamingSearchResults {...defaultProps} streamSearch={() => of(result)} />
                </SearchQueryStateStoreProvider>
            )}
        </WebStory>
    )
}

ServerSideAlert.storyName = 'server-side alert'

export const ServerSideAlertNoResults: StoryFn = () => {
    const result: AggregateStreamingSearchResults = {
        state: 'complete',
        results: [],
        filters: [],
        progress: {
            durationMs: 500,
            matchCount: MULTIPLE_SEARCH_RESULT.progress.matchCount,
            skipped: [],
        },
        alert: {
            proposedQueries: [{ query: 'test', description: 'Test query' }],
            title: 'Test Alert',
            description: 'This is a test alert',
        },
    }

    return (
        <WebStory>
            {() => (
                <SearchQueryStateStoreProvider useSearchQueryState={useNavbarQueryState}>
                    <StreamingSearchResults {...defaultProps} streamSearch={() => of(result)} />
                </SearchQueryStateStoreProvider>
            )}
        </WebStory>
    )
}

ServerSideAlertNoResults.storyName = 'server-side alert with no results'

export const ServerSideAlertUnownedResults: StoryFn = () => {
    const result: AggregateStreamingSearchResults = {
        state: 'complete',
        results: [],
        filters: [],
        progress: {
            durationMs: 500,
            matchCount: MULTIPLE_SEARCH_RESULT.progress.matchCount,
            skipped: [],
        },
        alert: {
            proposedQueries: [],
            kind: 'unowned-results',
            title: 'Some results have no owners',
            description:
                'For some results, no ownership data was found, or no rule applied to the result. [Learn more about configuring code ownership](https://sourcegraph.com/docs/own).',
        },
    }

    return (
        <WebStory>
            {() => (
                <SearchQueryStateStoreProvider useSearchQueryState={useNavbarQueryState}>
                    <StreamingSearchResults {...defaultProps} streamSearch={() => of(result)} />
                </SearchQueryStateStoreProvider>
            )}
        </WebStory>
    )
}

ServerSideAlertUnownedResults.storyName = 'server-side alert with unowned results'

export const ErrorWithNoResults: StoryFn = () => {
    const result: AggregateStreamingSearchResults = {
        state: 'error',
        results: [],
        filters: [],
        progress: {
            durationMs: 500,
            matchCount: MULTIPLE_SEARCH_RESULT.progress.matchCount,
            skipped: [],
        },
        error: new Error('test error'),
    }

    return (
        <WebStory>
            {() => (
                <SearchQueryStateStoreProvider useSearchQueryState={useNavbarQueryState}>
                    <StreamingSearchResults {...defaultProps} streamSearch={() => of(result)} />
                </SearchQueryStateStoreProvider>
            )}
        </WebStory>
    )
}

ErrorWithNoResults.storyName = 'error with no results'

export const ErrorWithSomeResults: StoryFn = () => {
    const result: AggregateStreamingSearchResults = {
        state: 'error',
        results: MULTIPLE_SEARCH_RESULT.results,
        filters: MULTIPLE_SEARCH_RESULT.filters,
        progress: {
            durationMs: 500,
            matchCount: MULTIPLE_SEARCH_RESULT.progress.matchCount,
            skipped: [],
        },
        error: new Error('test error'),
    }

    return (
        <WebStory>
            {() => (
                <SearchQueryStateStoreProvider useSearchQueryState={useNavbarQueryState}>
                    <StreamingSearchResults {...defaultProps} streamSearch={() => of(result)} />
                </SearchQueryStateStoreProvider>
            )}
        </WebStory>
    )
}

ErrorWithSomeResults.storyName = 'error with some results'

export const LimitHitWithSomeResults: StoryFn = () => {
    const result: AggregateStreamingSearchResults = {
        state: 'complete',
        results: MULTIPLE_SEARCH_RESULT.results,
        filters: MULTIPLE_SEARCH_RESULT.filters,
        progress: {
            durationMs: 500,
            matchCount: MULTIPLE_SEARCH_RESULT.progress.matchCount,
            skipped: [
                {
                    reason: 'document-match-limit',
                    message: 'result limit hit',
                    severity: 'info',
                    title: 'result limit hit',
                },
            ],
        },
    }

    return (
        <WebStory>
            {() => (
                <SearchQueryStateStoreProvider useSearchQueryState={useNavbarQueryState}>
                    <StreamingSearchResults {...defaultProps} streamSearch={() => of(result)} />
                </SearchQueryStateStoreProvider>
            )}
        </WebStory>
    )
}

LimitHitWithSomeResults.storyName = 'limit hit with some results'
