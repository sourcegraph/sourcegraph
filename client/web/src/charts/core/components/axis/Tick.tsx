import { FC } from 'react'

import { TickRendererProps } from '@visx/axis'
import { Group } from '@visx/group'
import { Text, TextProps } from '@visx/text'
import classNames from 'classnames';

import styles from './Tick.module.scss'

export interface TickProps extends TickRendererProps {
    /** Max width of value of ticks in symbols */
    maxWidth?: number
}

/** Tick component displays tick label for each axis line of chart. */
export const Tick: FC<TickProps> = props => {
    const { formattedValue = '', maxWidth, 'aria-label': ariaLabel, className, ...tickLabelProps } = props

    const tickLabelText = maxWidth && maxWidth <= formattedValue.length
        ? `${formattedValue.slice(0, maxWidth)}...`
        : formattedValue

    // Hack with Group + Text (aria hidden)
    // Because the Text component renders text inside svg element and text element with tspan
    // this makes another nested group for a11y tree. To avoid "group - end group"
    // phrase in voice over we hide nested children from a11y tree and put explicit aria-label
    // on the parent Group element with role text
    return (
        // eslint-disable-next-line jsx-a11y/aria-role
        <Group role="text" aria-label={ariaLabel}>
            <Text
                aria-hidden={true}
                className={classNames(styles.tick, className)}
                {...(tickLabelProps as TextProps)}>

                {tickLabelText}
            </Text>
        </Group>
    )
}

interface TickSizeResult {
    maxWidth: number
    maxHeight: number,
}

/**
 * Text (labels) ticks measure helper. Since there is no way to measure text
 * before rendering inside React tree we have to conduct pre-rendering measurements
 * for ticks labels.
 *
 * It renders each labels (text tick) inside selection element with SVG text element
 * and measures its sizes.
 */
export const getMaxTickSize = (selection: Element, labels: string[]): TickSizeResult => {
    const tester = document.createElementNS('http://www.w3.org/2000/svg', 'text');

    // In order to sync Tick component and pre-rendering text styles which is vital for
    // text measurements
    tester.classList.add(styles.tick)
    selection.append(tester)

    const boundingBoxes = labels.map(label => {
        tester.textContent = label

        return tester.getBBox()
    });

    tester.remove()

    const maxHeight = Math.max(...boundingBoxes.map(b => b.height));
    const maxWidth = Math.max(...boundingBoxes.map(b => b.width));

    return {
        maxHeight,
        maxWidth
    }
}
