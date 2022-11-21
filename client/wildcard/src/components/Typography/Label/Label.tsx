import React, { MouseEvent } from 'react'

import classNames from 'classnames'
import { useMergeRefs } from 'use-callback-ref'

import { ForwardReferenceComponent } from '../../../types'
import { TypographyProps } from '../utils'

import { getLabelClassName } from './utils'

interface LabelProps extends React.HTMLAttributes<HTMLLabelElement>, TypographyProps {
    size?: 'small' | 'base'
    weight?: 'regular' | 'medium' | 'bold'
    isUnderline?: boolean
    isUppercase?: boolean
}

export const Label = React.forwardRef((props, reference) => {
    const {
        children,
        as: Component = 'label',
        size,
        weight,
        alignment,
        mode,
        isUnderline,
        isUppercase,
        className,
        onClick,
        ...rest
    } = props

    const mergedRef = useMergeRefs([reference])

    // Listen to all clicks on the label element in order to improve click-to-focus logic
    // for contenteditable="true". By default, label element's native behavior (click to focus the first input element)
    // doesn't work with contenteditable elements.
    // Since we use contenteditable inputs (the CodeMirror search box) and labels together in some
    // consumers, we need to support this behavior manually for contenteditable elements.
    const handleClick = (event: MouseEvent<HTMLLabelElement>): void => {
        const forAttribute = mergedRef.current?.getAttribute('for')

        if (forAttribute) {
            onClick?.(event)
            return
        }

        // Extend labelable elements set with contenteditable elements
        const labelableElement = mergedRef.current?.querySelector<HTMLElement>(
            'input, keygen, meter, output, progress, select, textarea, [contenteditable=""], [contenteditable="true"]'
        )

        if (labelableElement) {
            labelableElement.focus()
        }

        onClick?.(event)
    }

    return (
        <Component
            ref={mergedRef}
            className={classNames(
                getLabelClassName({ isUppercase, isUnderline, alignment, weight, size, mode }),
                className
            )}
            onClick={handleClick}
            {...rest}
        >
            {children}
        </Component>
    )
}) as ForwardReferenceComponent<'label', LabelProps>
