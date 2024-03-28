import { useCallback, useEffect, useState } from 'react'

import { escapeRegExp, partition, sum } from 'lodash'
import { defer, lastValueFrom, type Observable } from 'rxjs'
import { map, retry } from 'rxjs/operators'

import { asError } from '@sourcegraph/common'
import { dataOrThrowErrors, gql } from '@sourcegraph/http-client'
import { buildSearchURLQuery } from '@sourcegraph/shared/src/util/url'
import { createLinkUrl } from '@sourcegraph/wildcard'

import { requestGraphQL } from '../../../../../backend/graphql'
import {
    type LangStatsInsightContentResult,
    type LangStatsInsightContentVariables,
    SearchPatternType,
} from '../../../../../graphql-operations'
import type { CategoricalChartContent } from '../../backend/code-insights-backend-types'

import { LivePreviewStatus, type State } from './types'

interface LangStatsDatum {
    name: string
    totalLines: number
    linkURL: string
    fill: string
}

interface Props {
    skip?: boolean
    repository: string
    otherThreshold: number
    path?: string
}

interface Result<R> {
    state: State<R>
    refetch: () => void
}

/**
 * Language stats insight live preview handler. This hook doesn't use code insight GQL API,
 * instead, it internally runs the search GQL API for data fetching and does post-processing
 * (language stats aggregation) on the frontend in browser runtime.
 */
export function useLivePreviewLangStatsInsight(props: Props): Result<CategoricalChartContent<LangStatsDatum>> {
    const { skip = false, path, repository, otherThreshold } = props

    const [updateTrigger, setResetTrigger] = useState(0)
    const [state, setState] = useState<State<CategoricalChartContent<LangStatsDatum>>>({
        status: LivePreviewStatus.Intact,
    })

    useEffect(() => {
        let hasRequestCanceled = false

        if (!skip) {
            setState({ status: LivePreviewStatus.Loading })

            getLangStats({ repository, otherThreshold, path })
                .then(data => !hasRequestCanceled && setState({ status: LivePreviewStatus.Data, data }))
                .catch(
                    error => !hasRequestCanceled && setState({ status: LivePreviewStatus.Error, error: asError(error) })
                )
        } else {
            setState({ status: LivePreviewStatus.Intact })
        }

        return () => {
            hasRequestCanceled = true
        }
    }, [skip, path, repository, otherThreshold, updateTrigger])

    const refetch = useCallback(() => setResetTrigger(count => count + 1), [])

    return { state, refetch }
}

interface LazyQueryProps {
    repository: string
    otherThreshold: number
    path?: string
}

interface LazyPreviewResult {
    lazyQuery: () => Promise<CategoricalChartContent<LangStatsDatum>>
}

/**
 * Lazy live preview lang stats insight, it's primarily used on the code insights
 * dashboard page where we have to load insight on demand if they overlap with
 * visible viewport.
 */
export function useLazyLivePreviewLangStatsInsight(props: LazyQueryProps): LazyPreviewResult {
    const { repository, otherThreshold, path } = props

    const lazyQuery = useCallback(
        () => getLangStats({ repository, otherThreshold, path }),
        [repository, otherThreshold, path]
    )

    return { lazyQuery }
}

interface GetInsightContentInputs {
    repository: string
    otherThreshold: number
    path?: string
}

async function getLangStats(inputs: GetInsightContentInputs): Promise<CategoricalChartContent<LangStatsDatum>> {
    const { repository, path, otherThreshold } = inputs

    const pathRegexp = path ? `file:^${escapeRegExp(path)}/` : ''
    const query = `repo:^${escapeRegExp(repository)}$ ${pathRegexp}`

    const stats = await lastValueFrom(
        defer(() => fetchLangStatsInsight(query)).pipe(
            // The search may time out, but a retry is then likely faster because caches are warm
            retry(3),
            map(data => data.search!.stats)
        ),
        { defaultValue: undefined }
    )

    if (!stats || stats.languages.length === 0) {
        throw new Error("We couldn't find the language statistics, try changing the repository.")
    }

    const totalLines = sum(stats.languages.map(language => language.totalLines))

    const [notOther, other] = partition(stats.languages, language => language.totalLines / totalLines >= otherThreshold)
    const OTHER_NAME = 'Other'
    const data = await Promise.all(
        [...notOther, { name: 'Other', totalLines: sum(other.map(language => language.totalLines)) }].map(
            async language => ({
                ...language,
                fill: await getLangColor(language.name),
                linkURL: createLinkUrl({
                    pathname: '/search',
                    search: buildSearchURLQuery(
                        language.name === OTHER_NAME ? query : `${query} lang:${quoteIfNeeded(language.name)}`,
                        SearchPatternType.standard,
                        false
                    ),
                }),
            })
        )
    )

    return {
        data,
        getDatumColor: datum => datum.fill,
        getDatumLink: datum => datum.linkURL,
        getDatumName: datum => datum.name,
        getDatumValue: datum => datum.totalLines,
    }
}

function quoteIfNeeded(value: string): string {
    return value.includes(' ') ? `"${value}"` : value
}

export const GET_LANG_STATS_GQL = gql`
    query LangStatsInsightContent($query: String!) {
        search(query: $query) {
            results {
                limitHit
            }
            stats {
                languages {
                    name
                    totalLines
                }
            }
        }
    }
`

function fetchLangStatsInsight(query: string): Observable<LangStatsInsightContentResult> {
    return requestGraphQL<LangStatsInsightContentResult, LangStatsInsightContentVariables>(GET_LANG_STATS_GQL, {
        query,
    }).pipe(map(dataOrThrowErrors))
}

async function getLangColor(language: string): Promise<string> {
    const { default: languagesMap } = await import('linguist-languages')

    const isLinguistLanguage = (language: string): language is keyof typeof languagesMap =>
        Object.prototype.hasOwnProperty.call(languagesMap, language)

    if (isLinguistLanguage(language)) {
        return languagesMap[language].color ?? 'gray'
    }

    return 'gray'
}
