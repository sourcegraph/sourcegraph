import { Series, SeriesLikeChart } from '../../../../../../../charts'

interface SeriesWithQuery<T> extends Series<T> {
    name: string
    query: string
}

export interface InsightExampleCommonContent {
    title: string
    repositories: string
}

export interface SearchInsightExampleContent<T> extends SeriesLikeChart<T> {
    series: SeriesWithQuery<T>[]
    title: string
    repositories: string
}

export interface CaptureGroupExampleContent<T> extends SeriesLikeChart<T> {
    groupSearch: string
    title: string
    repositories: string
}
