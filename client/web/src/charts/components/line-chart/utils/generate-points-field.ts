import { ScaleLinear, ScaleTime } from 'd3-scale'

import { getDatumValue, isDatumWithValidNumber, SeriesWithData, SeriesDatum } from './data-series-processing'

const NULL_LINK = (): undefined => undefined

interface PointsFieldInput<Datum> {
    dataSeries: SeriesWithData<Datum>[]
    xScale: ScaleTime<number, number>
    yScale: ScaleLinear<number, number>
}

export interface GeneratedPoint {
    id: string
    seriesId: string
    yValue: number
    xValue: Date
    y: number
    x: number
    color: string
    linkUrl: string | undefined
}

export function generatePointsField<Datum>(input: PointsFieldInput<Datum>): { [seriesId: string]: GeneratedPoint[] } {
    const { dataSeries, xScale, yScale } = input
    const starter: { [key: string]: GeneratedPoint[] } = {}

    return dataSeries.reduce((previous, series) => {
        const { id, data, getLinkURL = NULL_LINK } = series

        previous[id] = (data as SeriesDatum<Datum>[]).filter(isDatumWithValidNumber).map((datum, index) => {
            const datumValue = getDatumValue(datum)

            return {
                id: `${id}-${index}`,
                seriesId: id.toString(),
                yValue: datumValue,
                xValue: datum.x,
                y: yScale(datumValue),
                x: xScale(datum.x),
                color: series.color ?? 'green',
                linkUrl: getLinkURL(datum.datum, index),
            }
        })

        return previous
    }, starter)
}
