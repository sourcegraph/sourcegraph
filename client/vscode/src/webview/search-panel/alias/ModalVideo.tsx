import React from 'react'

import classNames from 'classnames'

import { Button } from '@sourcegraph/wildcard'

import { useWebviewPageContext } from '../../platform/context'

import styles from './ModalVideo.module.scss'

// We can't play video in VS Code Desktop: https://stackoverflow.com/a/57512681
// Open video in YouTube instead.

interface ModalVideoProps {
    id: string
    title: string
    src: string
    thumbnail?: { src: string; alt: string }
    onToggle?: (isOpen: boolean) => void
    showCaption?: boolean
    className?: string
    assetsRoot?: string
}

export const ModalVideo: React.FunctionComponent<React.PropsWithChildren<ModalVideoProps>> = ({
    title,
    src,
    thumbnail,
    onToggle,
    showCaption = false,
    className,
    assetsRoot = '',
}) => {
    const { extensionCoreAPI } = useWebviewPageContext()

    const onClick = (): void => {
        onToggle?.(false)
        extensionCoreAPI.openLink(src).catch(error => {
            console.error(`Error opening video at ${src}`, error)
        })
    }

    let thumbnailElement = thumbnail ? (
        <button type="button" className={classNames(styles.thumbnailButton, 'border-0')} onClick={onClick}>
            <img
                src={`${assetsRoot}/${thumbnail.src}`}
                alt={thumbnail.alt}
                className={classNames(styles.thumbnailImage, 'rounded border opacity-75')}
            />
            <div className={styles.playIconWrapper}>
                <PlayIcon />
            </div>
        </button>
    ) : null

    if (showCaption) {
        thumbnailElement = (
            <figure>
                {thumbnailElement}
                <figcaption>
                    <Button variant="link" onClick={onClick}>
                        {title}
                    </Button>
                </figcaption>
            </figure>
        )
    }

    return <div className={classNames(styles.wrapper, className)}>{thumbnailElement}</div>
}

const PlayIcon = React.memo(() => (
    <svg width="50" height="53" viewBox="0 0 50 53" fill="none" xmlns="http://www.w3.org/2000/svg">
        <g filter="url(#filter0_dd_268:5695)">
            <path d="M37.5 26.5L12.75 40.7894L12.75 12.2106L37.5 26.5Z" fill="white" />
        </g>
        <defs>
            <filter
                id="filter0_dd_268:5695"
                x="0.75"
                y="0.210449"
                width="48.75"
                height="52.5791"
                filterUnits="userSpaceOnUse"
                colorInterpolationFilters="sRGB"
            >
                <feFlood floodOpacity="0" result="BackgroundImageFix" />
                <feColorMatrix
                    in="SourceAlpha"
                    type="matrix"
                    values="0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 127 0"
                    result="hardAlpha"
                />
                <feOffset />
                <feGaussianBlur stdDeviation="6" />
                <feColorMatrix
                    type="matrix"
                    values="0 0 0 0 0.00505209 0 0 0 0 0.0449636 0 0 0 0 0.404167 0 0 0 0.25 0"
                />
                <feBlend mode="normal" in2="BackgroundImageFix" result="effect1_dropShadow_268:5695" />
                <feColorMatrix
                    in="SourceAlpha"
                    type="matrix"
                    values="0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 127 0"
                    result="hardAlpha"
                />
                <feOffset dy="4" />
                <feGaussianBlur stdDeviation="2" />
                <feColorMatrix type="matrix" values="0 0 0 0 0 0 0 0 0 0.055 0 0 0 0 0.25 0 0 0 0.25 0" />
                <feBlend mode="normal" in2="effect1_dropShadow_268:5695" result="effect2_dropShadow_268:5695" />
                <feBlend mode="normal" in="SourceGraphic" in2="effect2_dropShadow_268:5695" result="shape" />
            </filter>
        </defs>
    </svg>
))
