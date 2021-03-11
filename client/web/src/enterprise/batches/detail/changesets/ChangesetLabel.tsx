import React from 'react'
import classNames from 'classnames'
import { ChangesetLabelFields } from '../../../../graphql-operations'

interface Props {
    label: ChangesetLabelFields
}

/**
 * Converts a hex color to a perceived brightness. Algorithm to determine color brightness taken from https://www.w3.org/TR/AERT/#color-contrast
 *
 * @param color The hex encoded RGB color without the #
 *
 * @returns The perceived brightness from 0 to 255
 */
export function colorBrightness(color: string): number {
    const [red, green, blue] = [color.slice(0, 2), color.slice(2, 4), color.slice(4, 6)].map(value =>
        parseInt(value, 16)
    )
    return (red * 299 + green * 587 + blue * 114) / 1000
}

export const ChangesetLabel: React.FunctionComponent<Props> = ({ label }) => {
    // We use this value to determine the label text color (dark or bright, depending on the colorBrightness of the label)
    const labelBrightness = colorBrightness(label.color)

    return (
        <span
            className={classNames(
                'badge mr-2 badge-secondary',
                labelBrightness < 127 ? 'text-white' : 'changeset-label__text--dark'
            )}
            // eslint-disable-next-line react/forbid-dom-props
            style={{ backgroundColor: '#' + label.color }}
            data-tooltip={label.description}
        >
            {label.text}
        </span>
    )
}
