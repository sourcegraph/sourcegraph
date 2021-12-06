import classNames from 'classnames'
import React, { PropsWithChildren, useCallback, useMemo } from 'react'
import { Layout as ReactGridLayout, Layouts as ReactGridLayouts, Responsive, WidthProvider } from 'react-grid-layout'

import { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { isFirefox } from '@sourcegraph/shared/src/util/browserDetection'

import styles from './ViewGrid.module.scss'

// TODO use a method to get width that also triggers when file explorer is closed
// (WidthProvider only listens to window resize events)
const ResponsiveGridLayout = WidthProvider(Responsive)

export const BREAKPOINTS_NAMES = ['xs', 'sm', 'md', 'lg'] as const

export type BreakpointName = typeof BREAKPOINTS_NAMES[number]

/** Minimum size in px after which a breakpoint is active. */
export const BREAKPOINTS: Record<BreakpointName, number> = { xs: 0, sm: 576, md: 768, lg: 992 } // no xl because TreePage's max-width is the xl breakpoint.
export const COLUMNS: Record<BreakpointName, number> = { xs: 1, sm: 6, md: 8, lg: 12 }
export const DEFAULT_ITEMS_PER_ROW: Record<BreakpointName, number> = { xs: 1, sm: 2, md: 2, lg: 3 }
export const MIN_WIDTHS: Record<BreakpointName, number> = { xs: 1, sm: 2, md: 3, lg: 3 }
export const DEFAULT_HEIGHT = 3.25

const DEFAULT_VIEWS_LAYOUT_GENERATOR = (viewIds: string[]): ReactGridLayouts =>
    Object.fromEntries(
        BREAKPOINTS_NAMES.map(
            breakpointName =>
                [
                    breakpointName,
                    viewIds.map(
                        (id, index): ReactGridLayout => {
                            const width = COLUMNS[breakpointName] / DEFAULT_ITEMS_PER_ROW[breakpointName]
                            return {
                                i: id,
                                h: DEFAULT_HEIGHT,
                                w: width,
                                x: (index * width) % COLUMNS[breakpointName],
                                y: Math.floor((index * width) / COLUMNS[breakpointName]),
                                minW: MIN_WIDTHS[breakpointName],
                                minH: 2,
                            }
                        }
                    ),
                ] as const
        )
    )

export type ViewGridProps =
    | {
          /**
           * All view ids within the grid component. Used to calculate
           * layouts for each grid element.
           */
          viewIds: string[]
          layouts?: never
      }
    | {
          /**
           * Sets custom layout for react-grid-layout library.
           */
          layouts: ReactGridLayouts
          viewIds?: never
      }

interface ViewGridCommonProps extends TelemetryProps {
    /** Custom classname for root element of the grid. */
    className?: string
}

/**
 * Renders drag and drop and resizable views grid.
 */
export const ViewGrid: React.FunctionComponent<PropsWithChildren<ViewGridProps & ViewGridCommonProps>> = props => {
    const { layouts, viewIds = [], telemetryService, children, className } = props

    const onResizeOrDragStart: ReactGridLayout.ItemCallback = useCallback(
        (_layout, item) => {
            try {
                telemetryService.log(
                    'InsightUICustomization',
                    { insightType: item.i.split('.')[0] },
                    { insightType: item.i.split('.')[0] }
                )
            } catch {
                // noop
            }
        },
        [telemetryService]
    )

    // For Firefox we can't use css transform/translate to put view grid item in right place.
    // Instead of this we have to fallback on css position:absolute top/left properties.
    // Reason: Since we use coordinate transformation svg.getScreenCTM from svg viewport
    // to dom viewport in order to calculate chart tooltip position and firefox has
    // a bug about providing right value of screenCTM matrix when parent has css transform.
    // https://bugzilla.mozilla.org/show_bug.cgi?id=1610093
    // Back to css transforms when this bug will be resolved in Firefox.
    const useCSSTransforms = useMemo(() => !isFirefox(), [])

    return (
        <div className={classNames(className, styles.viewGrid)}>
            <ResponsiveGridLayout
                measureBeforeMount={true}
                breakpoints={BREAKPOINTS}
                layouts={layouts ?? DEFAULT_VIEWS_LAYOUT_GENERATOR(viewIds)}
                cols={COLUMNS}
                autoSize={true}
                rowHeight={6 * 16}
                containerPadding={[0, 0]}
                useCSSTransforms={useCSSTransforms}
                margin={[12, 12]}
                onResizeStart={onResizeOrDragStart}
                onDragStart={onResizeOrDragStart}
            >
                {children}
            </ResponsiveGridLayout>
        </div>
    )
}
