import { InsightType } from '../core'

export enum CodeInsightTrackType {
    SearchBasedInsight = 'SearchBased',
    LangStatsInsight = 'LangStats',
    CaptureGroupInsight = 'CaptureGroup',
    ComputeInsight = 'ComputeInsight',
    InProductLandingPageInsight = 'InProductLandingPageInsight',
    CloudLandingPageInsight = 'CloudLandingPageInsight',
}

export const V2InsightType: { [k in CodeInsightTrackType]: number } = {
    SearchBased: 0,
    LangStats: 1,
    CaptureGroup: 2,
    ComputeInsight: 3,
    InProductLandingPageInsight: 4,
    CloudLandingPageInsight: 5,
}

export const getTrackingTypeByInsightType = (insightType: InsightType): CodeInsightTrackType => {
    switch (insightType) {
        case InsightType.CaptureGroup: {
            return CodeInsightTrackType.CaptureGroupInsight
        }
        case InsightType.SearchBased: {
            return CodeInsightTrackType.SearchBasedInsight
        }
        case InsightType.LangStats: {
            return CodeInsightTrackType.LangStatsInsight
        }
        case InsightType.Compute: {
            return CodeInsightTrackType.ComputeInsight
        }
    }
}
