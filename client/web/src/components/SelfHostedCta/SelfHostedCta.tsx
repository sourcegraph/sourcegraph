import React from 'react'

import { mdiArrowRight } from '@mdi/js'
import classNames from 'classnames'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Link, Icon, H3 } from '@sourcegraph/wildcard'

import { MarketingBlock } from '../MarketingBlock'

export interface SelfHostedCtaProps extends TelemetryProps, TelemetryV2Props {
    className?: string
    contentClassName?: string
    // The name of the page the CTA will be posted. Add more as needed. DO NOT
    // include full URLs here, because this will be logged to our analytics
    // systems. We do not want to expose private repo names or search queries
    // to our analytics.
    page: 'organizations' | 'storybook'
}

// pages as numbers for v2 telemetry
const V2Pages: { [k in SelfHostedCtaProps['page']]: number } = {
    organizations: 0,
    storybook: 1,
}

export const SelfHostedCta: React.FunctionComponent<React.PropsWithChildren<SelfHostedCtaProps>> = ({
    className,
    contentClassName,
    telemetryService,
    telemetryRecorder,
    page,
    children,
}) => {
    const linkProps = { rel: 'noopener noreferrer' }

    const gettingStartedCTAOnClick = (): void => {
        telemetryService.log('InstallSourcegraphCTAClicked', { page }, { page })
        telemetryRecorder.recordEvent('selfHostedCTA.install', 'click', { metadata: { page: V2Pages[page] } })
    }

    const helpGettingStartedCTAOnClick = (): void => {
        telemetryService.log('HelpGettingStartedCTA', { page }, { page })
        telemetryRecorder.recordEvent('selfHostedCTA.talkToADev', 'click', { metadata: { page: V2Pages[page] } })
    }

    return (
        <div
            className={classNames(
                'd-flex flex-md-row align-items-md-start justify-content-md-between flex-column',
                className
            )}
        >
            <div className={classNames('mr-md-4 mr-0', contentClassName)}>
                {children}

                <ul>
                    <li>
                        <Link onClick={gettingStartedCTAOnClick} to="/help/admin/deploy" {...linkProps}>
                            Learn how to install
                        </Link>
                    </li>
                </ul>
            </div>

            <MarketingBlock wrapperClassName="flex-md-shrink-0 mt-md-0 mt-sm-2 w-sm-100">
                <H3 className="pr-3">Need help getting started?</H3>

                <div>
                    <Link
                        onClick={helpGettingStartedCTAOnClick}
                        to="https://info.sourcegraph.com/talk-to-a-developer?form_submission_source=inproduct&utm_campaign=inproduct-talktoadev&utm_medium=direct_traffic&utm_source=in-product&utm_term=null&utm_content=talktoadevform"
                        {...linkProps}
                    >
                        Speak to an engineer
                        <Icon className="ml-2" aria-hidden={true} svgPath={mdiArrowRight} />
                    </Link>
                </div>
            </MarketingBlock>
        </div>
    )
}
