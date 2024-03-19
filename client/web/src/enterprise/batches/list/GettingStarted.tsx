import React from 'react'

import { mdiOpenInNew } from '@mdi/js'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Alert, Container, H2, H3, Link, Text, Icon, useReducedMotion } from '@sourcegraph/wildcard'

import { BatchChangesIcon } from '../../../batches/icons'
import { CallToActionBanner } from '../../../components/CallToActionBanner'
import { CtaBanner } from '../../../components/CtaBanner'
import { eventLogger } from '../../../tracking/eventLogger'

export interface GettingStartedProps extends TelemetryV2Props {
    isSourcegraphDotCom: boolean
    // canCreate indicates whether or not the currently-authenticated user has sufficient
    // permissions to create a batch change in whatever context this getting started
    // section is being presented. If not, canCreate will be a string reason why the user
    // cannot create.
    canCreate: true | string
    className?: string
}

const productPageUrl = 'https://sourcegraph.com/batch-changes'

export const GettingStarted: React.FunctionComponent<React.PropsWithChildren<GettingStartedProps>> = ({
    isSourcegraphDotCom,
    canCreate,
    className,
    telemetryRecorder,
}) => {
    const allowAutoplay = !useReducedMotion()

    return (
        <div className={className} data-testid="test-getting-started">
            <Container className="mb-3">
                {canCreate === true ? null : (
                    <Alert className="my-3" variant="info">
                        {canCreate}
                    </Alert>
                )}
                <div className="row align-items-center">
                    <div className="col-12 col-md-7">
                        <video
                            className="w-100 h-auto shadow percy-hide"
                            width={1280}
                            height={720}
                            autoPlay={allowAutoplay}
                            muted={true}
                            loop={true}
                            playsInline={true}
                            controls={!allowAutoplay}
                        >
                            <source
                                type="video/webm"
                                src="https://storage.googleapis.com/sourcegraph-assets/batch-changes/how-it-works.webm"
                            />
                            <source
                                type="video/mp4"
                                src="https://storage.googleapis.com/sourcegraph-assets/batch-changes/how-it-works.mp4"
                            />
                        </video>
                    </div>
                    <div className="col-12 col-md-5">
                        <H2>Automate large-scale code changes</H2>
                        <Text>
                            Batch Changes makes it easy to find and change code across many repositories (or many
                            subtrees in a big monorepo). It lets you create, update, and track pull requests to ensure
                            the change is reviewed, tested, and safely merged everywhere.
                        </Text>
                        <H3>Use Batch Changes to...</H3>
                        <ul>
                            <li>Update configuration files across many repositories</li>
                            <li>Update libraries consuming your APIs</li>
                            <li>Rapidly fix critical security issues</li>
                            <li>Update boilerplate code</li>
                            <li>Pay down tech debt</li>
                        </ul>
                        <H3>Resources</H3>
                        <ul>
                            <li>
                                <Link to="/help/batch_changes" target="_blank" rel="noopener">
                                    Documentation{' '}
                                    <Icon role="img" aria-label="Open in a new tab" svgPath={mdiOpenInNew} />
                                </Link>
                            </li>
                            <li>
                                <Link to={productPageUrl} target="_blank" rel="noopener">
                                    Product page{' '}
                                    <Icon role="img" aria-label="Open in a new tab" svgPath={mdiOpenInNew} />
                                </Link>
                            </li>
                        </ul>
                    </div>
                </div>
            </Container>
            {isSourcegraphDotCom ? (
                <CallToActionBanner variant="filled">
                    To automate changes across your team's private repositories,{' '}
                    <Link
                        to="https://sourcegraph.com"
                        onClick={() => {
                            eventLogger.log('ClickedOnEnterpriseCTA', { location: 'BatchChangesGettingStarted' })
                            telemetryRecorder.recordEvent('batchChanges.enterpriseCTA', 'click', {
                                metadata: { location: 1 },
                            })
                        }}
                    >
                        get Sourcegraph Enterprise
                    </Link>
                    .
                </CallToActionBanner>
            ) : (
                <div className="d-flex justify-content-start">
                    <CtaBanner
                        bodyText="Try it yourself in less than 10 minutes (without actually pushing changes)."
                        title={<H3>Start using Batch Changes</H3>}
                        linkText="Read quickstart docs"
                        href="/help/batch_changes/quickstart"
                        icon={<BatchChangesIcon />}
                    />
                </div>
            )}
        </div>
    )
}
