import { useState, useEffect } from 'react'

import { mdiOpenInNew } from '@mdi/js'
import classNames from 'classnames'

import type { TelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { EVENT_LOGGER } from '@sourcegraph/shared/src/telemetry/web/eventLogger'
import { Button, ButtonLink, H2, Text, Link, Icon } from '@sourcegraph/wildcard'

import { EventName } from '../../../util/constants'
import { EditorStep } from '../../management/CodyManagementPage'

import { CodyFeatures } from './CodyFeatures'

import styles from '../CodyOnboarding.module.scss'

export function VSCodeInstructions({
    onBack,
    onClose,
    showStep,
    telemetryRecorder,
}: {
    onBack?: () => void
    onClose: () => void
    showStep?: EditorStep
    telemetryRecorder: TelemetryRecorder
}): JSX.Element {
    const [step, setStep] = useState<EditorStep>(showStep || 0)
    const marketplaceUrl = 'https://marketplace.visualstudio.com/items?itemName=sourcegraph.cody-ai'

    useEffect(() => {
        if (step === EditorStep.SetupInstructions) {
            EVENT_LOGGER.log(EventName.CODY_EDITOR_SETUP_VIEWED, { editor: 'VS Code' })
            telemetryRecorder.recordEvent('cody.editorSetupPage', 'view', { metadata: { vsCode: 1 } })
        } else if (step === EditorStep.CodyFeatures) {
            EVENT_LOGGER.log(EventName.CODY_EDITOR_FEATURES_VIEWED, { editor: 'VS Code' })
            telemetryRecorder.recordEvent('cody.editorFeaturesPage', 'view', { metadata: { vsCode: 1 } })
        }
    }, [step, telemetryRecorder])

    return (
        <>
            {step === EditorStep.SetupInstructions && (
                <>
                    <div className="pb-3 border-bottom">
                        <H2>Setup instructions for VS Code</H2>
                    </div>

                    <div className={classNames('pt-3 px-3', styles.instructionsContainer)}>
                        <div className={classNames('border-bottom', styles.highlightStep)}>
                            <div className="d-flex align-items-center">
                                <div className="mr-1">
                                    <div className={classNames('mr-2', styles.step)}>1</div>
                                </div>
                                <div>
                                    <Text className="mb-1" weight="bold">
                                        Install Cody
                                    </Text>
                                    <Text className="text-muted mb-0" size="small">
                                        Alternatively, you can reach this page by clicking{' '}
                                        <strong>View {'>'} Extensions</strong> and searching for{' '}
                                        <strong>Cody AI</strong>
                                    </Text>
                                </div>
                            </div>
                            <div className="d-flex flex-column justify-content-center align-items-center mt-4">
                                <ButtonLink
                                    variant="primary"
                                    to={marketplaceUrl}
                                    target="_blank"
                                    onClick={event => {
                                        event.preventDefault()
                                        EVENT_LOGGER.log(EventName.CODY_EDITOR_SETUP_OPEN_MARKETPLACE, {
                                            editor: 'VS Code',
                                        })
                                        telemetryRecorder.recordEvent('cody.onboarding.openMarketplace', 'click', {
                                            metadata: { vsCode: 1 },
                                        })
                                        window.location.href = marketplaceUrl
                                    }}
                                >
                                    Open Marketplace
                                </ButtonLink>
                                <img
                                    alt="VS Code Marketplace"
                                    className="mt-4"
                                    width="70%"
                                    src="https://storage.googleapis.com/sourcegraph-assets/VSCodeInstructions/__step1.png"
                                />
                            </div>
                        </div>
                        <div className="mt-3 border-bottom">
                            <div className="d-flex align-items-center">
                                <div className="mr-1">
                                    <div className={classNames('mr-2', styles.step)}>2</div>
                                </div>
                                <div>
                                    <Text className="mb-1" weight="bold">
                                        Open Cody from the sidebar on the left
                                    </Text>
                                    <Text className="text-muted mb-0" size="small">
                                        Typically Cody will be the last item in the sidebar
                                    </Text>
                                </div>
                            </div>
                            <div className="d-flex flex-column justify-content-center align-items-center mt-4">
                                <img
                                    alt="VS Code Marketplace"
                                    className="mt-2"
                                    width="70%"
                                    src="https://storage.googleapis.com/sourcegraph-assets/VSCodeInstructions/__step2.png"
                                />
                            </div>
                        </div>
                        <div className="mt-3 border-bottom">
                            <div className="d-flex align-items-center">
                                <div className="mr-1">
                                    <div className={classNames('mr-2', styles.step)}>3</div>
                                </div>
                                <div>
                                    <Text className="mb-1" weight="bold">
                                        Log in
                                    </Text>
                                    <Text className="text-muted mb-0" size="small">
                                        Choose the same login method you used when you created your account
                                    </Text>
                                </div>
                            </div>
                            <div className="d-flex flex-column justify-content-center align-items-center mt-4">
                                <img
                                    alt="VS Code Marketplace"
                                    className="mt-2"
                                    width="70%"
                                    src="https://storage.googleapis.com/sourcegraph-assets/VSCodeInstructions/__step3.png"
                                />
                            </div>
                        </div>
                    </div>

                    {showStep === undefined ? (
                        <div className="mt-3 d-flex justify-content-between">
                            <Button variant="secondary" onClick={onBack} outline={true} size="sm">
                                Back
                            </Button>
                            <Button variant="primary" onClick={() => setStep(1)} size="sm">
                                Next
                            </Button>
                        </div>
                    ) : (
                        <div className="mt-3 d-flex justify-content-end">
                            <Button variant="primary" onClick={onClose} size="sm">
                                Close
                            </Button>
                        </div>
                    )}
                </>
            )}
            {step === EditorStep.CodyFeatures && (
                <CodyFeatures onClose={onClose} showStep={showStep} setStep={setStep} />
            )}
        </>
    )
}

export function VSCodeTabInstructions({ telemetryRecorder }: { telemetryRecorder: TelemetryRecorder }): JSX.Element {
    const marketplaceUrl = 'https://marketplace.visualstudio.com/items?itemName=sourcegraph.cody-ai'

    return (
        <div className={styles.ideContainer}>
            <Text className={classNames('mb-0', styles.install)}>Install</Text>
            <Link
                to="vscode:extension/sourcegraph.cody-ai"
                target="_blank"
                rel="noreferrer"
                className="d-inline-flex align-items-center"
                onClick={event => {
                    event.preventDefault()
                    EVENT_LOGGER.log(EventName.CODY_EDITOR_SETUP_OPEN_MARKETPLACE, {
                        editor: 'VS Code',
                    })
                    telemetryRecorder.recordEvent('cody.onboarding.extension', 'click', {
                        metadata: { vsCode: 1 },
                    })
                    window.location.href = 'vscode:extension/sourcegraph.cody-ai'
                }}
            >
                <div className={styles.ideLinkAlt}> Open in VS Code</div>
                <Icon
                    className={classNames('ml-1', styles.ideLink)}
                    role="img"
                    aria-label="Open in a new tab"
                    svgPath={mdiOpenInNew}
                />
            </Link>
            <div className={styles.ideOr}>or</div>
            <Link
                to={marketplaceUrl}
                target="_blank"
                rel="noreferrer"
                className="d-inline-flex align-items-center"
                onClick={event => {
                    event.preventDefault()
                    EVENT_LOGGER.log(EventName.CODY_EDITOR_SETUP_OPEN_MARKETPLACE, {
                        editor: 'VS Code',
                    })
                    telemetryRecorder.recordEvent('cody.onboarding.openMarketplace', 'click', {
                        metadata: { vsCode: 1 },
                    })
                    window.location.href = marketplaceUrl
                }}
            >
                <div className={classNames(styles.ideLink)}>View in VS Code marketplace</div>
                <Icon
                    className={classNames('ml-1', styles.ideLink)}
                    role="img"
                    aria-label="Open in a new tab"
                    svgPath={mdiOpenInNew}
                />
            </Link>
        </div>
    )
}
