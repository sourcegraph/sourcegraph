import type { FC, SVGProps } from 'react'

interface CustomIconProps extends SVGProps<SVGSVGElement> {}

// This file contains core-line icons by https://www.streamlinehq.com. It is licensed under CC BY 4.0
// Original source of icon set - https://www.streamlinehq.com/icons/core-line-free

export const OpenBookIcon: FC<CustomIconProps> = props => (
    <svg width="18" height="18" viewBox="0 0 18 18" fill="transparent" {...props}>
        <path
            d="M8.99988 16.4284C7.32587 14.5854 5.05564 13.3916 2.58846 13.057C2.30763 13.0259 2.04827 12.8919 1.86044 12.6809C1.6726 12.4698 1.5696 12.1967 1.57131 11.9141V2.71413C1.5713 2.54906 1.60705 2.38594 1.6761 2.23599C1.74515 2.08606 1.84586 1.95286 1.97131 1.84556C2.09455 1.74021 2.23879 1.66229 2.39446 1.61697C2.55011 1.57165 2.71364 1.55997 2.87417 1.5827C5.23336 1.97425 7.39156 3.14999 8.99988 4.91985V16.4284Z"
            stroke="var(--icon-color)"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
        />
        <path
            d="M9 16.4284C10.674 14.5854 12.9442 13.3916 15.4114 13.057C15.6922 13.0259 15.9517 12.8919 16.1394 12.6809C16.3273 12.4698 16.4303 12.1967 16.4286 11.9141V2.71413C16.4286 2.54906 16.3928 2.38594 16.3238 2.23599C16.2547 2.08606 16.1541 1.95286 16.0286 1.84556C15.9054 1.74021 15.7611 1.66229 15.6055 1.61697C15.4497 1.57165 15.2863 1.55997 15.1257 1.5827C12.7665 1.97425 10.6083 3.14999 9 4.91985V16.4284Z"
            stroke="var(--icon-color)"
            strokeWidth="1.5"
            strokeLinecap="round"
            strokeLinejoin="round"
        />
    </svg>
)

export const LinkShareIcon: FC<CustomIconProps> = props => (
    <svg width="14" height="14" viewBox="0 0 14 14" fill="none" {...props}>
        <path
            d="M10.0001 7.85714V11.7143C10.0001 11.9416 9.9098 12.1597 9.74908 12.3204C9.58837 12.4811 9.37033 12.5714 9.143 12.5714H2.28585C2.05852 12.5714 1.84051 12.4811 1.67976 12.3204C1.51902 12.1597 1.42871 11.9416 1.42871 11.7143V4.85714C1.42871 4.62981 1.51902 4.4118 1.67976 4.25105C1.84051 4.09031 2.05852 4 2.28585 4H6.143"
            stroke="var(--icon-color)"
            strokeWidth="1.25"
            strokeLinecap="round"
            strokeLinejoin="round"
        />
        <path
            d="M9.57178 1.42859H12.5718V4.42859"
            stroke="var(--icon-color)"
            strokeWidth="1.25"
            strokeLinecap="round"
            strokeLinejoin="round"
        />
        <path
            d="M12.5717 1.42859L7.85742 6.14287"
            stroke="var(--icon-color)"
            strokeWidth="1.25"
            strokeLinecap="round"
            strokeLinejoin="round"
        />
    </svg>
)

export const ArrowBendIcon: FC<CustomIconProps> = props => (
    <svg viewBox="0 0 14 14" height="14" width="14" fill="none" {...props}>
        <path
            id="Vector"
            stroke="var(--icon-color)"
            strokeWidth="1"
            strokeLinecap="round"
            strokeLinejoin="round"
            d="m11.5 10.5 -3 3 -3 -3"
        />
        <path
            id="Vector_2"
            stroke="var(--icon-color)"
            strokeWidth="1"
            strokeLinecap="round"
            strokeLinejoin="round"
            d="M2.5 0.5h2c1.06087 0 2.07828 0.421427 2.82843 1.17157C8.07857 2.42172 8.5 3.43913 8.5 4.5v9"
        />
    </svg>
)

export const DeleteIcon: FC<CustomIconProps> = props => (
    <svg fill="none" viewBox="0 0 14 14" height="14" width="14" {...props}>
        <path
            stroke="var(--icon-color)"
            strokeLinecap="round"
            strokeLinejoin="round"
            d="m13.5 0.5 -13 13"
            strokeWidth="1"
        />
        <path
            stroke="var(--icon-color)"
            strokeLinecap="round"
            strokeLinejoin="round"
            d="m0.5 0.5 13 13"
            strokeWidth="1"
        />
    </svg>
)
