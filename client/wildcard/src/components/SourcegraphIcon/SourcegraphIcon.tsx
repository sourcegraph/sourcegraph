import * as React from 'react'

export const SourcegraphIcon: React.FunctionComponent<
    React.PropsWithChildren<React.SVGAttributes<SVGSVGElement>>
> = props => (
    <svg viewBox="0 0 52 52" fill="none" xmlns="http://www.w3.org/2000/svg" {...props}>
        <path
            d="M30.8 51.8c-2.8.5-5.5-1.3-6-4.1L17.2 6.2c-.5-2.8 1.3-5.5 4.1-6s5.5 1.3 6 4.1l7.6 41.5c.5 2.8-1.4 5.5-4.1 6z"
            fill="#FF5543"
        />
        <path
            d="M10.9 44.7C9.1 45 7.3 44.4 6 43c-1.8-2.2-1.6-5.4.6-7.2L38.7 8.5c2.2-1.8 5.4-1.6 7.2.6 1.8 2.2 1.6 5.4-.6 7.2l-32 27.3c-.7.6-1.6 1-2.4 1.1z"
            fill="#A112FF"
        />
        <path
            d="M46.8 38.1c-.9.2-1.8.1-2.6-.2L4.4 23.8c-2.7-1-4.1-3.9-3.1-6.6 1-2.7 3.9-4.1 6.6-3.1l39.7 14.1c2.7 1 4.1 3.9 3.1 6.6-.6 1.8-2.2 3-3.9 3.3z"
            fill="#00CBEC"
        />
    </svg>
)
