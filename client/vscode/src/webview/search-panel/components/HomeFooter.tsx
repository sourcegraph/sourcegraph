import React, { useCallback } from 'react'

import classNames from 'classnames'

import { SyntaxHighlightedSearchQuery } from '@sourcegraph/branded'
import type { QueryState } from '@sourcegraph/shared/src/search'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Card, Text } from '@sourcegraph/wildcard'

import { ModalVideo } from '../alias/ModalVideo'

import { type SearchExample, exampleQueries } from './SearchExamples'

import styles from './HomeFooter.module.scss'

export interface HomeFooterProps extends TelemetryProps {
    setQuery: (newState: QueryState) => void
}

interface SearchExamplesProps extends TelemetryProps {
    title: string
    subtitle: string
    examples: SearchExample[]
    icon: JSX.Element
    setQuery: (newState: QueryState) => void
}

const SearchExamples: React.FunctionComponent<React.PropsWithChildren<SearchExamplesProps>> = ({
    title,
    subtitle,
    examples,
    icon,
    telemetryService,
    setQuery,
}) => {
    const searchExampleClicked = useCallback(
        (trackEventName: string, fullQuery: string) => (): void => {
            setQuery({ query: fullQuery })
            telemetryService.log(trackEventName)
        },
        [setQuery, telemetryService]
    )
    return (
        <div className={styles.searchExamplesWrapper}>
            <div className={classNames('d-flex align-items-baseline mb-2', styles.searchExamplesTitleWrapper)}>
                <div className={classNames('mr-2', styles.title, styles.searchExamplesTitle)}>{title}</div>
                <div className={classNames(styles.searchExamplesSubtitle)}>{subtitle}</div>
            </div>
            <div className={styles.searchExamples}>
                {examples.map(example => (
                    <div key={example.queryPreview} className="search-example-card-wrapper">
                        <Card
                            as="button"
                            className={classNames('p-0 w-100', styles.searchExampleCard)}
                            onClick={searchExampleClicked(example.trackEventName, example.fullQuery)}
                        >
                            <div className={classNames('search-example-example-icons', styles.searchExampleIcon)}>
                                {icon}
                            </div>
                            <div className={styles.searchExampleQueryWrapper}>
                                <div className={styles.searchExampleQuery}>
                                    <SyntaxHighlightedSearchQuery query={example.queryPreview} />
                                </div>
                            </div>
                        </Card>
                        <Text className={styles.searchExampleLabel}>{example.label}</Text>
                    </div>
                ))}
            </div>
        </div>
    )
}

export const HomeFooter: React.FunctionComponent<React.PropsWithChildren<HomeFooterProps>> = props => (
    <>
        <div className={styles.footerContainer}>
            <div className={styles.helpContent}>
                <SearchExamples
                    title="Search examples"
                    subtitle="Find answers faster with code search across multiple repos and commits"
                    examples={exampleQueries}
                    icon={<MagnifyingGlassSearchIcon />}
                    {...props}
                />
                <div className={styles.thumbnailWrapper}>
                    <div className={classNames(styles.title, styles.searchExamplesTitle, 'mb-2')}>Watch and learn</div>
                    <div className={styles.thumbnail}>
                        {/* TODO: UPLOAD PREVIEW IMAGE TO SG TO USE SG AS ACCESSROOT */}
                        <ModalVideo
                            id="three-ways-to-search-title"
                            title="Three ways to search"
                            src="https://youtu.be/w6pz4GPL80g"
                            thumbnail={{
                                src: 'DtL9ZJs/vsce-watch-and-learn.png',
                                alt: 'Watch and learn video thumbnail',
                            }}
                            onToggle={() => props.telemetryService.log('VSCEHomeWatch&Lean')}
                            // assetsRoot="https://sourcegraph.com/.assets/"
                            assetsRoot="https://i.ibb.co/"
                        />
                    </div>
                </div>
            </div>
        </div>
    </>
)

const MagnifyingGlassSearchIcon = React.memo(() => (
    <svg width="18" height="18" fill="none" xmlns="http://www.w3.org/2000/svg">
        <path
            d="M6.686.5a6.672 6.672 0 016.685 6.686 6.438 6.438 0 01-1.645 4.32l.308.308h.823L18 16.957 16.457 18.5l-5.143-5.143v-.823l-.308-.308a6.438 6.438 0 01-4.32 1.645A6.672 6.672 0 010 7.186 6.672 6.672 0 016.686.5zm0 2.057a4.61 4.61 0 00-4.629 4.629 4.61 4.61 0 004.629 4.628 4.61 4.61 0 004.628-4.628 4.61 4.61 0 00-4.628-4.629z"
            fill="currentColor"
        />
    </svg>
))
