import { useMemo, useEffect, useState, useCallback } from 'react'

import { differenceInHours, formatISO, parseISO } from 'date-fns'

import { streamComputeQuery } from '@sourcegraph/shared/src/search/stream'
import { useTemporarySetting } from '@sourcegraph/shared/src/settings/temporary/useTemporarySetting'
import type { ProductStatusType } from '@sourcegraph/wildcard'

import { basicSyntaxColumns } from './QueryExamples.constants'

export interface QueryExamplesContent {
    repositoryName: string
    filePath: string
}

export interface QueryExamplesSection {
    title: string
    queryExamples: {
        query: string
        helperText?: string
        productStatus?: ProductStatusType
    }[]
}

interface ComputeResult {
    kind: string
    value: string
}

const defaultQueryExamplesContent = {
    repositoryName: 'org/repo',
    filePath: 'file.go',
}

function hasQueryExamplesContentCacheExpired(lastCachedTimestamp: string): boolean {
    return differenceInHours(Date.now(), parseISO(lastCachedTimestamp)) > 24
}

function quoteIfNeeded(value: string): string {
    return value.includes(' ') ? `"${value}"` : value
}

function getQueryExamplesContentFromComputeOutput(computeOutput: string): QueryExamplesContent {
    const [repositoryName, filePath] = computeOutput.trim().split(',|')
    return {
        repositoryName,
        filePath,
    }
}

function getRepoFilterExamples(repositoryName: string): { singleRepoExample: string; orgReposExample?: string } {
    const repositoryNameParts = repositoryName.split('/')

    if (repositoryNameParts.length <= 1) {
        return { singleRepoExample: quoteIfNeeded(repositoryName) }
    }

    const repoName = repositoryNameParts.at(-1)
    const repoOrg = repositoryNameParts.at(-2)
    return {
        singleRepoExample: quoteIfNeeded(`${repoOrg}/${repoName}`),
        orgReposExample: quoteIfNeeded(`${repoOrg}/`),
    }
}

export function useQueryExamples(
    selectedSearchContextSpec: string,
    isSourcegraphDotCom: boolean,
    keywordSearch: boolean
): QueryExamplesSection[][] {
    const [queryExamplesContent, setQueryExamplesContent] = useState<QueryExamplesContent>()
    const [cachedQueryExamplesContent, setCachedQueryExamplesContent, cachedQueryExamplesContentLoadStatus] =
        useTemporarySetting('search.homepage.queryExamplesContent')

    const loadQueryExamples = useCallback(
        (selectedSearchContextSpec: string) =>
            // We are using `,|` as the separator so we can "safely" split the compute output.
            streamComputeQuery(
                `context:${selectedSearchContextSpec} type:diff count:1 content:output((.|\n)* -> $repo,|$path)`
            ).subscribe(
                results => {
                    const firstComputeOutput = results
                        .flatMap(result => JSON.parse(result) as ComputeResult)
                        .find(result => result.kind === 'output')

                    const queryExamplesContent = firstComputeOutput
                        ? getQueryExamplesContentFromComputeOutput(firstComputeOutput.value)
                        : defaultQueryExamplesContent

                    setQueryExamplesContent(queryExamplesContent)
                    setCachedQueryExamplesContent({
                        ...queryExamplesContent,
                        lastCachedTimestamp: formatISO(Date.now()),
                    })
                },
                () => {
                    // In case of an error set default content.
                    setQueryExamplesContent(defaultQueryExamplesContent)
                }
            ),
        [setQueryExamplesContent, setCachedQueryExamplesContent]
    )

    useEffect(() => {
        if (queryExamplesContent || cachedQueryExamplesContentLoadStatus === 'initial' || isSourcegraphDotCom) {
            return
        }
        if (
            cachedQueryExamplesContentLoadStatus === 'loaded' &&
            cachedQueryExamplesContent &&
            !hasQueryExamplesContentCacheExpired(cachedQueryExamplesContent.lastCachedTimestamp)
        ) {
            setQueryExamplesContent(cachedQueryExamplesContent)
            return
        }

        const subscription = loadQueryExamples(selectedSearchContextSpec)
        return () => subscription.unsubscribe()
    }, [
        selectedSearchContextSpec,
        queryExamplesContent,
        cachedQueryExamplesContent,
        setCachedQueryExamplesContent,
        cachedQueryExamplesContentLoadStatus,
        loadQueryExamples,
        isSourcegraphDotCom,
    ])

    useEffect(() => {
        if (!queryExamplesContent) {
            return
        }
        const subscription = loadQueryExamples(selectedSearchContextSpec)
        return () => subscription.unsubscribe()
        // Only re-run this hook if the search context changes
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [selectedSearchContextSpec])

    return useMemo(() => {
        // Static examples for Sourcegraph.com.
        if (isSourcegraphDotCom) {
            return basicSyntaxColumns('test', 'facebook/react', 'kubernetes/', keywordSearch)
        }
        if (!queryExamplesContent) {
            return []
        }
        const { repositoryName, filePath } = queryExamplesContent

        const { singleRepoExample, orgReposExample } = getRepoFilterExamples(repositoryName)
        const filePathParts = filePath.split('/')
        const fileName = quoteIfNeeded(filePathParts.at(-1)!)

        return basicSyntaxColumns(fileName, singleRepoExample, orgReposExample, keywordSearch)
    }, [queryExamplesContent, isSourcegraphDotCom, keywordSearch])
}
