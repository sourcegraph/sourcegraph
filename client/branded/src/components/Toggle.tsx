import * as React from 'react'
import classnames from 'classnames'

interface Props {
    /** The initial value. */
    value?: boolean

    /** The DOM ID of the element. */
    id?: string

    /**
     * Called when the user changes the input's value.
     */
    onToggle?: (value: boolean) => void

    onClick?: (event: React.MouseEvent<HTMLButtonElement>) => void

    /** The title attribute (tooltip). */
    title?: string

    'aria-label'?: string
    'aria-labelledby'?: string
    'aria-describedby'?: string

    disabled?: boolean
    tabIndex?: number
    className?: string

    /** Data attribute for testing */
    dataTest?: string
}

/** A toggle switch input component. */
export const Toggle: React.FunctionComponent<Props> = ({
    disabled,
    className,
    id,
    title,
    value,
    tabIndex,
    onToggle,
    onClick,
    dataTest,
    'aria-label': ariaLabel,
    'aria-labelledby': ariaLabelledby,
    'aria-describedby': ariaDescribedby,
}) => {
    function onButtonClick(event: React.MouseEvent<HTMLButtonElement>): void {
        event.stopPropagation()
        if (!disabled && onToggle) {
            onToggle(!value)
        }
        if (!disabled && onClick) {
            onClick(event)
        }
    }

    return (
        <button
            type="button"
            className={classnames('toggle', className, {})}
            id={id}
            title={title}
            value={value ? 1 : 0}
            onClick={onButtonClick}
            tabIndex={tabIndex}
            disabled={disabled}
            role="switch"
            aria-checked={value}
            aria-label={ariaLabel}
            aria-labelledby={ariaLabelledby}
            aria-describedby={ariaDescribedby}
            data-test={dataTest}
        >
            <span
                className={classnames('toggle__bar', {
                    'toggle__bar--on': value,
                })}
            />
            <span
                className={classnames('toggle__bar-shadow', {
                    'toggle__bar-shadow--on': value,
                })}
            />
            <span
                className={classnames('toggle__knob', {
                    'toggle__knob--on': value,
                })}
            />
        </button>
    )
}
