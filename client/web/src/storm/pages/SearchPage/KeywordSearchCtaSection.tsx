import React from 'react'

import { mdiClose } from '@mdi/js'
import classNames from 'classnames'

import { useTemporarySetting } from '@sourcegraph/shared/src/settings/temporary'
import { Code, H2, Icon, Link, ProductStatusBadge, Text } from '@sourcegraph/wildcard'

import { MarketingBlock } from '../../../components/MarketingBlock'

import { KeywordSearchStarsIcon } from './KeywordSearchStarsIcon'

import styles from './KeywordSearchCtaSection.module.scss'

interface KeywordSearchCtaSection {
    className?: string
}

export const KeywordSearchCtaSection: React.FC<KeywordSearchCtaSection> = ({ className }) => {
    const [isDismissed = true, setIsDismissed] = useTemporarySetting('search.homepage.keywordCta.dismissed', false)
    if (isDismissed) {
        return null
    }

    return (
        <MarketingBlock
            wrapperClassName={classNames(styles.container)}
            contentClassName={classNames('flex-grow-1 d-flex justify-content-between p-4', styles.card)}
        >
            <div>
                <H2 className="d-flex align-items-center">
                    New keyword search
                    <ProductStatusBadge status="beta" className="ml-2" />
                </H2>
                <div className="d-flex d-flex-column">
                    <div>
                        <KeywordSearchStarsIcon aria-hidden={true} />
                    </div>
                    <div>
                        <Text>
                            <ul>
                                <li>
                                    The search bar now supports <b>keyword search</b>, where terms match broadly across
                                    the file contents and path
                                </li>
                                <li>The new behavior ANDs terms together instead of searching literally by default </li>
                                <li>
                                    To search literally, wrap the query in quotes like{' '}
                                    <Code>"Error 101: service failed"</Code>
                                </li>
                            </ul>
                        </Text>
                        <Text>
                            <Link
                                to="https://sourcegraph.com/docs/code-search/queries#keyword-search-default"
                                target="_blank"
                                rel="noopener noreferrer"
                            >
                                Read the docs
                            </Link>{' '}
                            to learn more.
                        </Text>
                    </div>
                </div>
            </div>
            <Icon
                svgPath={mdiClose}
                aria-label="Close keyword search explanation"
                className={classNames(styles.closeButton)}
                onClick={() => setIsDismissed(true)}
            />
        </MarketingBlock>
    )
}
