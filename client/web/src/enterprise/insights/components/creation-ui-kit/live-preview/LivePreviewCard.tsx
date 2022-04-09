import React, { forwardRef } from 'react'

import { ParentSize } from '@visx/responsive'
import classNames from 'classnames'
import RefreshIcon from 'mdi-react/RefreshIcon'

import { Button, ForwardReferenceComponent } from '@sourcegraph/wildcard'

import { getLineColor, LegendItem, LegendList, Series } from '../../../../../charts'
import { InsightCard, InsightCardBanner, InsightCardLoading } from '../../views'

import styles from './LivePreviewCard.module.scss'

const LivePreviewCard: React.FunctionComponent = props => (
    <InsightCard className={styles.insightCard}>{props.children}</InsightCard>
)

export interface LivePreviewUpdateButtonProps {
    disabled: boolean
    onClick: () => void
}

const LivePreviewUpdateButton: React.FunctionComponent<LivePreviewUpdateButtonProps> = props => {
    const { disabled, onClick } = props

    return (
        <Button variant="icon" disabled={disabled} className={styles.updateButton} onClick={onClick}>
            Live preview <RefreshIcon size="1rem" />
        </Button>
    )
}

const LivePreviewLoading = InsightCardLoading

const LivePreviewBlurBackdrop = forwardRef((props, reference) => {
    const { as: Component = 'svg', className, ...attributes } = props

    return <Component ref={reference} className={classNames(styles.chartWithMock, className)} {...attributes} />
}) as ForwardReferenceComponent<'svg', {}>

const LivePreviewBanner: React.FunctionComponent = props => (
    <InsightCardBanner className={styles.disableBanner}>{props.children}</InsightCardBanner>
)

interface LivePreviewChartProps extends React.ComponentProps<typeof ParentSize> {}

const LivePreviewChart: React.FunctionComponent<LivePreviewChartProps> = props => (
    <ParentSize {...props} className={classNames(styles.chartBlock, props.className)} />
)

interface LivePreviewLegendProps {
    series: Series<unknown>[]
}

const LivePreviewLegend: React.FunctionComponent<LivePreviewLegendProps> = props => {
    const { series } = props

    return (
        <LegendList className="mt-3">
            {series.map(series => (
                <LegendItem key={series.dataKey} color={getLineColor(series)} name={series.name} />
            ))}
        </LegendList>
    )
}

export {
    LivePreviewCard,
    LivePreviewUpdateButton,
    LivePreviewLoading,
    LivePreviewChart,
    LivePreviewLegend,
    LivePreviewBlurBackdrop,
    LivePreviewBanner,
}
