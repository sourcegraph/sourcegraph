import { type FunctionComponent, useContext } from 'react'

import { SyntaxHighlightedSearchQuery } from '@sourcegraph/branded'
import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Button, Link, LegendItem, LegendList, ParentSize, LegendItemPoint } from '@sourcegraph/wildcard'

import { useSeriesToggle } from '../../../../../../../../insights/utils/use-series-toggle'
import {
    InsightCard,
    InsightCardHeader,
    InsightCardLegend,
    SeriesBasedChartTypes,
    SeriesChart,
} from '../../../../../../components'
import { InsightType } from '../../../../../../core'
import { CodeInsightTrackType, useCodeInsightViewPings } from '../../../../../../pings'
import {
    CodeInsightsLandingPageContext,
    CodeInsightsLandingPageType,
    useLogEventName,
} from '../../../../CodeInsightsLandingPageContext'
import { CodeInsightsQueryBlock } from '../../code-insights-query-block/CodeInsightsQueryBlock'
import type { CaptureGroupExampleContent, SearchInsightExampleContent } from '../types'

import styles from './CodeInsightExampleCard.module.scss'

type CodeInsightExampleProps = (CodeInsightSearchExampleProps | CodeInsightCaptureExampleProps) & {
    templateLink?: string
    className?: string
}

export const CodeInsightExampleCard: FunctionComponent<CodeInsightExampleProps> = props => {
    if (props.type === InsightType.SearchBased) {
        return <CodeInsightSearchExample {...props} />
    }

    return <CodeInsightCaptureExample {...props} />
}

interface CodeInsightSearchExampleProps extends TelemetryProps, TelemetryV2Props {
    type: InsightType.SearchBased
    content: SearchInsightExampleContent<any>
    templateLink?: string
    className?: string
}

const CodeInsightSearchExample: FunctionComponent<CodeInsightSearchExampleProps> = props => {
    const { templateLink, className, content, telemetryService, telemetryRecorder } = props
    const seriesToggleState = useSeriesToggle()

    const { mode } = useContext(CodeInsightsLandingPageContext)

    const bigTemplateClickPingName = useLogEventName('InsightsGetStartedBigTemplateClick')

    const { trackMouseEnter, trackMouseLeave } = useCodeInsightViewPings({
        telemetryService,
        telemetryRecorder,
        insightType:
            mode === CodeInsightsLandingPageType.Cloud
                ? CodeInsightTrackType.CloudLandingPageInsight
                : CodeInsightTrackType.InProductLandingPageInsight,
    })

    const handleTemplateLinkClick = (): void => {
        telemetryService.log(bigTemplateClickPingName)
        telemetryRecorder.recordEvent('insights.getStarted.bigTemplate', 'click')
    }

    return (
        <InsightCard className={className} onMouseEnter={trackMouseEnter} onMouseLeave={trackMouseLeave}>
            <InsightCardHeader
                title={content.title}
                subtitle={
                    <CodeInsightsQueryBlock
                        as={SyntaxHighlightedSearchQuery}
                        query={content.repositories}
                        className="mt-1"
                    />
                }
            >
                {templateLink && (
                    <Button
                        as={Link}
                        variant="link"
                        size="sm"
                        className={styles.actionLink}
                        to={templateLink}
                        onClick={handleTemplateLinkClick}
                    >
                        Use as template
                    </Button>
                )}
            </InsightCardHeader>

            <ParentSize className={styles.chart}>
                {parent => (
                    <SeriesChart
                        {...content}
                        type={SeriesBasedChartTypes.Line}
                        width={parent.width}
                        height={parent.height}
                        seriesToggleState={seriesToggleState}
                    />
                )}
            </ParentSize>

            <LegendList className={styles.legend}>
                {content.series.map(series => (
                    <LegendItem key={series.id as string}>
                        <LegendItemPoint color={series.color} />
                        <span className={styles.legendItem}>{series.name}</span>
                        <CodeInsightsQueryBlock as={SyntaxHighlightedSearchQuery} query={series.query} />
                    </LegendItem>
                ))}
            </LegendList>
        </InsightCard>
    )
}

interface CodeInsightCaptureExampleProps extends TelemetryProps, TelemetryV2Props {
    type: InsightType.CaptureGroup
    content: CaptureGroupExampleContent<any>
    templateLink?: string
    className?: string
}

const CodeInsightCaptureExample: FunctionComponent<CodeInsightCaptureExampleProps> = props => {
    const {
        content: { title, groupSearch, repositories, ...content },
        templateLink,
        className,
        telemetryService,
        telemetryRecorder,
    } = props
    const seriesToggleState = useSeriesToggle()

    const { mode } = useContext(CodeInsightsLandingPageContext)
    const bigTemplateClickPingName = useLogEventName('InsightsGetStartedBigTemplateClick')

    const { trackMouseEnter, trackMouseLeave } = useCodeInsightViewPings({
        telemetryService,
        telemetryRecorder,
        insightType:
            mode === CodeInsightsLandingPageType.Cloud
                ? CodeInsightTrackType.CloudLandingPageInsight
                : CodeInsightTrackType.InProductLandingPageInsight,
    })

    const handleTemplateLinkClick = (): void => {
        telemetryService.log(bigTemplateClickPingName)
        telemetryRecorder.recordEvent('insights.getStarted.bigTemplate', 'click')
    }

    return (
        <InsightCard className={className} onMouseEnter={trackMouseEnter} onMouseLeave={trackMouseLeave}>
            <InsightCardHeader
                title={title}
                subtitle={
                    <CodeInsightsQueryBlock as={SyntaxHighlightedSearchQuery} query={repositories} className="mt-1" />
                }
            >
                {templateLink && (
                    <Button
                        as={Link}
                        variant="link"
                        size="sm"
                        className={styles.actionLink}
                        to={templateLink}
                        onClick={handleTemplateLinkClick}
                    >
                        Use as template
                    </Button>
                )}
            </InsightCardHeader>

            <div className={styles.captureGroup}>
                <div className={styles.chart}>
                    <ParentSize className={styles.chartContent}>
                        {parent => (
                            <SeriesChart
                                {...content}
                                type={SeriesBasedChartTypes.Line}
                                width={parent.width}
                                height={parent.height}
                                seriesToggleState={seriesToggleState}
                            />
                        )}
                    </ParentSize>
                </div>

                <InsightCardLegend series={content.series} className={styles.legend} />
            </div>

            <CodeInsightsQueryBlock as={SyntaxHighlightedSearchQuery} query={groupSearch} className="mt-3" />
        </InsightCard>
    )
}
