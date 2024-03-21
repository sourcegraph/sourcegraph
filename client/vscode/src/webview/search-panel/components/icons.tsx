import * as React from 'react'

export const BookmarkRadialGradientIcon = React.memo(() => (
    <svg width="26" height="26" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path
            d="M20.143 21.667L13 18.517l-7.143 3.15V2.889h14.286v18.778zm0-21.667H5.857C5.1 0 4.373.304 3.837.846A2.905 2.905 0 003 2.89V26l10-4.333L23 26V2.889c0-.766-.301-1.501-.837-2.043A2.842 2.842 0 0020.143 0z"
            fill="#6B47D6"
        />
        <path
            d="M20.143 21.667L13 18.517l-7.143 3.15V2.889h14.286v18.778zm0-21.667H5.857C5.1 0 4.373.304 3.837.846A2.905 2.905 0 003 2.89V26l10-4.333L23 26V2.889c0-.766-.301-1.501-.837-2.043A2.842 2.842 0 0020.143 0z"
            fill="url(#paint0_radial)"
            fillOpacity=".59"
        />
        <defs>
            <radialGradient
                id="paint0_radial"
                cx="0"
                cy="0"
                r="1"
                gradientUnits="userSpaceOnUse"
                gradientTransform="matrix(18.00004 -1 1.4154 25.47731 8.5 12)"
            >
                <stop stopColor="#E105CB" />
                <stop offset="1" stopColor="#fff" stopOpacity="0" />
            </radialGradient>
        </defs>
    </svg>
))

export const SearchBetaIcon = React.memo(() => (
    <svg width="77" height="60" fill="none" xmlns="http://www.w3.org/2000/svg">
        <rect x="4.25" width="60" height="60" rx="4" fill="url(#paint0_linear)" />
        <rect x="4.25" width="60" height="60" rx="4" fill="url(#paint1_radial)" fillOpacity=".4" />
        <path
            d="M29.621 12a13.372 13.372 0 0113.372 13.371c0 3.312-1.214 6.357-3.21 8.702l.556.556h1.625L52.25 44.914 49.164 48 38.88 37.714V36.09l-.556-.555a13.404 13.404 0 01-8.702 3.209 13.371 13.371 0 010-26.743zm0 4.114a9.219 9.219 0 00-9.257 9.257 9.219 9.219 0 009.257 9.258 9.219 9.219 0 009.258-9.258 9.219 9.219 0 00-9.258-9.257z"
            fill="#fff"
        />
        <g filter="url(#filter0_d)">
            <rect x="44.75" y="11" width="28" height="14" rx="1.613" fill="#66D9E8" />
            <path
                d="M51.038 21c1.274 0 2.051-.645 2.051-1.696 0-.777-.554-1.356-1.356-1.43v-.075c.6-.095 1.062-.65 1.062-1.278 0-.918-.682-1.489-1.815-1.489h-2.494V21h2.552zm-1.485-5.136h1.166c.645 0 1.022.31 1.022.843 0 .546-.401.84-1.158.84h-1.03v-1.683zm0 4.305v-1.865h1.2c.814 0 1.244.318 1.244.926 0 .612-.417.939-1.203.939h-1.24zm8.56-.066h-2.795v-1.708h2.643v-.848h-2.643V15.93h2.796v-.898H54.25V21h3.863v-.897zM61.99 21v-5.07h1.84v-.898h-4.743v.898h1.836V21h1.067zm6.145 0h1.146l-2.122-5.968h-1.2L63.842 21h1.08l.513-1.543h2.196L68.134 21zm-1.633-4.847h.07l.81 2.482h-1.7l.82-2.482z"
                fill="#2B3750"
            />
        </g>
        <defs>
            <radialGradient
                id="paint1_radial"
                cx="0"
                cy="0"
                r="1"
                gradientUnits="userSpaceOnUse"
                gradientTransform="rotate(-35.166 71.162 -22.034) scale(28.8342)"
            >
                <stop stopColor="#DC06FF" />
                <stop offset="1" stopColor="#DB00FF" stopOpacity="0" />
            </radialGradient>
            <linearGradient
                id="paint0_linear"
                x1="4.364"
                y1="43.846"
                x2="64.469"
                y2="43.596"
                gradientUnits="userSpaceOnUse"
            >
                <stop stopColor="#4B52F3" />
                <stop offset="1" stopColor="#8E35F4" />
            </linearGradient>
            <filter
                id="filter0_d"
                x="42.519"
                y="9.885"
                width="32.462"
                height="18.462"
                filterUnits="userSpaceOnUse"
                colorInterpolationFilters="sRGB"
            >
                <feFlood floodOpacity="0" result="BackgroundImageFix" />
                <feColorMatrix in="SourceAlpha" values="0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 127 0" result="hardAlpha" />
                <feOffset dy="1.115" />
                <feGaussianBlur stdDeviation="1.115" />
                <feColorMatrix values="0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0.35 0" />
                <feBlend in2="BackgroundImageFix" result="effect1_dropShadow" />
                <feBlend in="SourceGraphic" in2="effect1_dropShadow" result="shape" />
            </filter>
        </defs>
    </svg>
))

export const CodeMonitoringLogo: React.FunctionComponent<React.PropsWithChildren<React.SVGProps<SVGSVGElement>>> = (
    props: React.SVGProps<SVGSVGElement>
) => (
    <svg width="20" height="20" viewBox="0 0 20 20" fill="currentColor" xmlns="http://www.w3.org/2000/svg" {...props}>
        <path
            fillRule="evenodd"
            clipRule="evenodd"
            d="M18.01 8.01C18.01 8.29 18.23 8.51 18.51 8.51C18.79 8.51 19.01 8.29 19.01 8.01C19.01 4.15 15.87 1 12 1C11.72 1 11.5 1.22 11.5 1.5C11.5 1.78 11.72 2 12 2C15.31 2 18.01 4.7 18.01 8.01ZM16.1801 7.96002C15.9001 7.96002 15.6801 7.74002 15.6801 7.46002C15.6801 5.81002 14.3301 4.46002 12.6801 4.46002C12.4001 4.46002 12.1801 4.24002 12.1801 3.96002C12.1801 3.68002 12.4001 3.46002 12.6801 3.46002C14.8901 3.46002 16.6801 5.25002 16.6801 7.46002C16.6801 7.74002 16.4601 7.96002 16.1801 7.96002ZM4.83996 6.79999L13.34 15.3C12.39 15.88 11.29 16.18 10.15 16.18C8.49996 16.18 6.93996 15.54 5.76996 14.37C4.59996 13.2 3.94996 11.65 3.94996 9.98999C3.94996 8.84999 4.25996 7.74999 4.83996 6.79999ZM4.70996 4.54999C1.70996 7.54999 1.70996 12.43 4.70996 15.43C6.20996 16.93 8.17996 17.68 10.15 17.68C12.12 17.68 14.09 16.93 15.59 15.43L4.70996 4.54999ZM4 16.14C3.7 15.84 3.43 15.52 3.18 15.18L2.89 15.69L1 18.97H4.79H8.59L8.31 18.49C6.69 18.14 5.2 17.34 4 16.14ZM13.85 8.04999C13.85 9.01999 13.07 9.79999 12.1 9.79999C11.13 9.79999 10.35 9.01999 10.35 8.04999C10.35 7.07999 11.13 6.29999 12.1 6.29999C13.07 6.29999 13.85 7.07999 13.85 8.04999Z"
        />
    </svg>
)
