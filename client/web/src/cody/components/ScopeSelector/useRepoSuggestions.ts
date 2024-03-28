import { useMemo } from 'react'

import type { TranscriptJSON } from '@sourcegraph/cody-shared'
import { useQuery } from '@sourcegraph/http-client'
import type { AuthenticatedUser } from '@sourcegraph/shared/src/auth'

import { useUserHistory } from '../../../components/useUserHistory'
import type {
    ContextSelectorRepoFields,
    SuggestedReposResult,
    SuggestedReposVariables,
} from '../../../graphql-operations'

import { SuggestedReposQuery } from './backend'

interface UseRepoSuggestionsOpts {
    numSuggestions: number
    fallbackSuggestions: string[]
    omitSuggestions: { name: string }[]
}

const DEFAULT_OPTS: UseRepoSuggestionsOpts = {
    numSuggestions: 10,
    fallbackSuggestions: [
        // This is mostly relevant for dotcom but could fill in for any instance which
        // indexes GitHub OSS repositories. If the repositories are not indexed on the
        // instance, they will not return any results from the search API and thus will
        // not be included in the final list of suggestions.
        //
        // NOTE: The actual 10 most popular OSS repositories on GitHub are typically just
        // plaintext resource collections like awesome lists or how-to-program tutorials, which
        // are not likely to be as interesting to Cody users. Instead, we hardcode a list of
        // repositories that are among the top 100 that contain actual source code.
        'github.com/sourcegraph/sourcegraph',
        'github.com/freeCodeCamp/freeCodeCamp',
        'github.com/facebook/react',
        'github.com/tensorflow/tensorflow',
        'github.com/torvalds/linux',
        'github.com/microsoft/vscode',
        'github.com/flutter/flutter',
        'github.com/golang/go',
        'github.com/d3/d3',
        'github.com/kubernetes/kubernetes',
    ],
    omitSuggestions: [],
}

/**
 * useRepoSuggestions is a custom hook that generates repository suggestions for the
 * context scope selector.
 *
 * The suggestions are based on the current user's chat transcript history, search
 * browsing history, embedded repositories available on their instance, and any fallbacks
 * configured in the options.
 *
 * The number of suggestions can be configured with `opts.numSuggestions`. The default is 10.
 *
 * Fallback suggestions can be configured with `opts.fallbackSuggestions`. The default is
 * a list of 10 of the most popular OSS repositories on GitHub.
 *
 * Repositories can be omitted from the suggestions (for example, repositories that are
 * already added to the context scope) by passing them as `opts.omitSuggestions.`
 * @param transcriptHistory the current user's chat transcript history from the store
 * @param authenticatedUser the current authenticated user
 * @param opts any options for further configuring the suggestions
 * @returns a list of repository suggestions as `ContextSelectorRepoFields` objects
 */
export const useRepoSuggestions = (
    transcriptHistory: TranscriptJSON[],
    authenticatedUser: AuthenticatedUser | null = null,
    opts?: Partial<UseRepoSuggestionsOpts>
): ContextSelectorRepoFields[] => {
    const { numSuggestions, fallbackSuggestions, omitSuggestions } = { ...DEFAULT_OPTS, ...opts }

    const userHistory = useUserHistory(authenticatedUser?.id, false)
    const suggestedRepoNames: string[] = useMemo(() => {
        const flattenedTranscriptHistoryEntries = transcriptHistory
            .map(item => {
                const { scope, lastInteractionTimestamp } = item
                return (
                    // Return a new item for each repository in the scope.
                    scope?.repositories.map(name => ({
                        // Parse a date from the last interaction timestamp.
                        lastAccessed: new Date(lastInteractionTimestamp),
                        name,
                    })) || []
                )
            })
            .flat()
            // Remove duplicates.
            .filter(removeDupes)
            // We only need up to the first numSuggestions.
            .slice(0, numSuggestions)

        const userHistoryEntries =
            userHistory
                .loadEntries()
                .map(item => ({
                    name: item.repoName,
                    // Parse a date from the last acessed timestamp.
                    lastAccessed: new Date(item.lastAccessed),
                }))
                // We only need up to the first numSuggestions.
                .slice(0, numSuggestions) || []

        // We also can take a list of up to numSuggestions fallback repos to
        // fill in if we have fewer than numSuggestions to actually suggest.
        const fallbackRepos = fallbackSuggestions
            .map(name => ({
                name,
                // We order by most recently accessed; these should always be ranked last.
                lastAccessed: new Date(0),
            }))
            .slice(0, numSuggestions)

        // Merge the lists.
        const merged = [...flattenedTranscriptHistoryEntries, ...userHistoryEntries, ...fallbackRepos]
            // Sort by most recently accessed.
            .sort((a, b) => b.lastAccessed.getTime() - a.lastAccessed.getTime())
            // Remove duplicates.
            .filter(removeDupes)
            // Take the most recent numSuggestions.
            .slice(0, numSuggestions)

        // Return just the names.
        return merged.map(({ name }) => name)
    }, [transcriptHistory, userHistory, numSuggestions, fallbackSuggestions])

    // Query for the suggested repositories.
    const { data: suggestedReposData } = useQuery<SuggestedReposResult, SuggestedReposVariables>(SuggestedReposQuery, {
        variables: {
            names: suggestedRepoNames,
            numResults: numSuggestions,
        },
        fetchPolicy: 'cache-first',
    })

    // Filter out and reorder the suggested repository results.
    const suggestions: ContextSelectorRepoFields[] = useMemo(() => {
        if (!suggestedReposData) {
            return []
        }

        const nodes = [...suggestedReposData.byName.nodes]
        // The order of the by-name repos returned by the search API will not match the
        // order of suggestions we intend to display (the ordering of suggestedRepoNames),
        // since the default ordering of the search API is alphabetical. Thus, we reorder
        // them to match the initial ordering of suggestedRepoNames.
        const sortedByNameNodes = nodes.sort(
            (a, b) => suggestedRepoNames.indexOf(a.name) - suggestedRepoNames.indexOf(b.name)
        )
        // Make sure we have a full numSuggestions to display in the
        // suggestions. We'll prioritize the repositories we looked up by name, and then
        // fill in the rest from the first 10 embedded repositories returned by the search
        // API.
        return (
            [...sortedByNameNodes, ...suggestedReposData.firstN.nodes]
                // Remove any duplicates.
                .filter(removeDupes)
                // Take the first numSuggestions.
                .slice(0, numSuggestions)
                // Finally, filter out repositories that are should be omitted.
                .filter(suggestion => !omitSuggestions.find(toOmit => toOmit.name === suggestion.name))
        )
    }, [suggestedReposData, suggestedRepoNames, omitSuggestions, numSuggestions])

    return suggestions
}

/**
 * removeDupes is an `Array.filter` predicate function which removes duplicate entries
 * from an array of objects based on the `name` property. It filters out any entries which
 * are not the first occurrence with a given `name`, which means it will preserve the
 * earliest occurrence of each.
 */
const removeDupes = (first: { name: string }, index: number, self: { name: string }[]): boolean =>
    index === self.findIndex(entry => entry.name === first.name)
