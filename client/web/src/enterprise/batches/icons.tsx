import React from 'react'

/**
 * The internal type used for base icons, which accept both a class name and a
 * viewbox.
 */
type BaseIcon = React.FunctionComponent<{
    className?: string
    viewBox?: string
}>

/**
 * The internal type used for exported icons, which only support an optional
 * class name.
 */
type Icon = React.FunctionComponent<{
    className?: string
}>

/**
 * The base batch changes icon, which may have its class and viewBox overridden by
 * the exported components.
 */
const BaseBatchChangesIcon: BaseIcon = React.memo(function BaseBatchChangesIcon({
    className = '',
    viewBox = '0 0 20 20',
    ...props
}) {
    return (
        <svg
            width="20"
            height="20"
            fill="currentColor"
            className={className}
            viewBox={viewBox}
            {...props}
            xmlns="http://www.w3.org/2000/svg"
        >
            <path
                fillRule="evenodd"
                clipRule="evenodd"
                d="M4.5 5.5a1 1 0 100-2 1 1 0 000 2zm0 1.5a2.5 2.5 0 100-5 2.5 2.5 0 000 5z"
            />
            <path d="M13.117 2.967h4v4h-4v-4zM13.117 8.767h4v4h-4v-4zM13.117 14.567h4v4h-4v-4z" />
            <path
                fillRule="evenodd"
                clipRule="evenodd"
                d="M7.702 5a.75.75 0 01.75-.75h3a.75.75 0 110 1.5h-3a.75.75 0 01-.75-.75zM4.87 7.72a.75.75 0 00-.828.662l.746.083-.746-.083v.006L4.04 8.4l-.004.04a12.537 12.537 0 00-.04.671c-.015.44-.015 1.053.044 1.74.118 1.348.474 3.116 1.521 4.425 1.077 1.346 2.656 1.846 3.872 2.029a9.468 9.468 0 002.05.078l.136-.013.039-.004.012-.001h.005s.001-.001-.092-.745l.093.744a.75.75 0 00-.185-1.489M5.533 8.547v.007l-.003.028a11.019 11.019 0 00-.035.581c-.014.395-.014.944.04 1.559.11 1.258.432 2.66 1.197 3.617.736.92 1.875 1.325 2.924 1.482a7.968 7.968 0 001.81.057l.022-.002h.003M4.87 7.72a.75.75 0 01.662.827L4.87 7.72z"
            />
            <path
                fillRule="evenodd"
                clipRule="evenodd"
                d="M4.43 7.806a.75.75 0 00-.301 1.017l.659-.358-.66.358.001.001.001.001.002.003.004.008.011.02.04.067c.033.054.08.129.143.219a6.467 6.467 0 002.9 2.286c1.06.418 2.177.537 3.005.558a11.195 11.195 0 001.328-.046 5.62 5.62 0 00.084-.009l.025-.003h.007l.003-.001-.099-.744.1.744a.75.75 0 00-.198-1.487h-.001l-.015.002a9.684 9.684 0 01-1.196.045c-.737-.019-1.659-.126-2.492-.455a4.967 4.967 0 01-2.219-1.746 3.02 3.02 0 01-.115-.178l-.002-.005a.75.75 0 00-1.016-.297z"
            />
        </svg>
    )
})

/**
 * The icon to use everywhere to represent a batch change. Square, and by default
 * 20x20. If the icon's left side needs to be flush with the left edge, use
 * {@link BatchChangesIconFlushLeft} instead.
 */
export const BatchChangesIcon: Icon = props => <BaseBatchChangesIcon {...props} />

/**
 * The same icon as {@link BatchChangesIcon}, except the icon has no internal
 * padding on the left hand side. This is useful when the icon's left edge
 * should be flush with the left edges of other content displayed above and/or
 * below it.
 */
export const BatchChangesIconFlushLeft: Icon = props => <BaseBatchChangesIcon {...props} viewBox="2 0 20 20" />

/**
 * The base component for the navbar version of the batch changes
 * icon, with different proportions and bounding box.
 */
const BaseBatchChangesIconNav: BaseIcon = props => (
    <svg width="20" height="20" {...props} fill="currentColor" xmlns="http://www.w3.org/2000/svg">
        <path
            fillRule="evenodd"
            clipRule="evenodd"
            d="M5.829 6.76a1.932 1.932 0 100-3.863 1.932 1.932 0 000 3.863zm0 2.898a4.829 4.829 0 100-9.658 4.829 4.829 0 000 9.658z"
        />
        <path d="M22.473 1.867H30.2v7.726h-7.726V1.867zM22.473 13.07H30.2v7.727h-7.726V13.07zM22.473 24.274H30.2V32h-7.726v-7.726z" />
        <path
            fillRule="evenodd"
            clipRule="evenodd"
            d="M12.014 5.795c0-.8.648-1.449 1.448-1.449h5.795a1.449 1.449 0 110 2.897h-5.795c-.8 0-1.448-.648-1.448-1.448zM6.544 11.047a1.449 1.449 0 00-1.6 1.28l1.44.16-1.44-.16v.011l-.003.023-.008.08c-.006.066-.015.162-.024.283-.018.242-.04.587-.055 1.013a28.23 28.23 0 00.087 3.36c.226 2.602.915 6.018 2.937 8.546 2.08 2.599 5.13 3.566 7.48 3.918a18.29 18.29 0 003.957.15c.111-.008.2-.017.263-.023l.076-.008.023-.003h.008l.003-.001s.002 0-.178-1.438l.18 1.438a1.449 1.449 0 00-.358-2.875M7.824 12.646l-.001.012-.006.055-.02.231a25.333 25.333 0 00.03 3.902c.212 2.43.835 5.14 2.314 6.987 1.42 1.776 3.62 2.56 5.646 2.863a15.408 15.408 0 003.303.127 7.78 7.78 0 00.193-.017l.043-.005h.006M6.544 11.046a1.449 1.449 0 011.28 1.6l-1.28-1.6z"
        />
        <path
            fillRule="evenodd"
            clipRule="evenodd"
            d="M5.692 11.214a1.449 1.449 0 00-.58 1.965l1.272-.692-1.272.692v.002l.002.002.003.006.008.014.023.04a8.703 8.703 0 00.353.551 12.492 12.492 0 005.602 4.416c2.047.807 4.203 1.038 5.803 1.079a21.55 21.55 0 001.986-.04 16.55 16.55 0 00.742-.067l.047-.006.014-.002h.008l-.193-1.436.192 1.435a1.45 1.45 0 00-.383-2.871h-.002l-.027.003a13.35 13.35 0 01-.594.053c-.416.028-1.012.052-1.716.035-1.424-.037-3.205-.244-4.814-.878a9.594 9.594 0 01-4.286-3.373 5.756 5.756 0 01-.221-.345l-.005-.008a1.449 1.449 0 00-1.962-.575z"
        />
    </svg>
)

/**
 * The nav icon with a viewbox set up to align the icon with the other icons on
 * the global navbar.
 */
export const BatchChangesIconNav: Icon = props => <BaseBatchChangesIconNav {...props} viewBox="0 -3 38 38" />

/**
 * The nav icon with a viewbox set up to align the icon with the other icons on
 * the namespace navbar.
 */
export const BatchChangesIconNamespaceNav: Icon = props => <BaseBatchChangesIconNav {...props} viewBox="0 0 36 36" />
