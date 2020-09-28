import React from 'react'

export const SourcegraphIcon: React.FunctionComponent<React.SVGAttributes<SVGSVGElement> & { size?: number }> = ({
    size,
    ...props
}) => (
    <svg
        width={size ?? '65'}
        height={size ?? '64'}
        viewBox="0 0 65 64"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
        {...props}
    >
        <path
            fillRule="evenodd"
            clipRule="evenodd"
            d="M19.5809 8.42498L33.4692 59.2756C34.4044 62.6921 37.9254 64.7045 41.3365 63.772C44.7477 62.8342 46.7547 59.3051 45.8222 55.8886L31.9312 5.03529C30.996 1.61881 27.475 -0.393568 24.0639 0.541611C20.6554 1.47679 18.6457 5.00582 19.5809 8.42498Z"
            fill="#F96216"
        />
        <path
            fillRule="evenodd"
            clipRule="evenodd"
            d="M45.2995 8.23211L10.5184 47.5659C8.17375 50.2187 8.41759 54.2756 11.065 56.6256C13.7125 58.9756 17.7587 58.7291 20.1033 56.0763L54.8845 16.7425C57.2291 14.0897 56.9853 10.0355 54.3378 7.68548C51.6904 5.33547 47.6469 5.57931 45.2995 8.23211Z"
            fill="#B200F8"
        />
        <path
            fillRule="evenodd"
            clipRule="evenodd"
            d="M5.89199 30.0308L55.494 46.4621C58.8515 47.5768 62.4716 45.7493 63.5837 42.3864C64.6957 39.0208 62.8709 35.3927 59.516 34.2833L9.91138 17.844C6.55385 16.7346 2.93372 18.5568 1.82437 21.9223C0.712335 25.2879 2.53446 28.9161 5.89199 30.0308Z"
            fill="#00B4F2"
        />
    </svg>
)
