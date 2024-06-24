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

export function NeoVimInstructions({
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
    const marketplaceUrl = 'https://github.com/sourcegraph/sg.nvim#setup'

    useEffect(() => {
        if (step === EditorStep.SetupInstructions) {
            EVENT_LOGGER.log(EventName.CODY_EDITOR_SETUP_VIEWED, { editor: 'NeoVim' })
            telemetryRecorder.recordEvent('cody.editorSetupPage', 'view', { metadata: { neoVim: 1 } })
        } else if (step === EditorStep.CodyFeatures) {
            EVENT_LOGGER.log(EventName.CODY_EDITOR_FEATURES_VIEWED, { editor: 'NeoVim' })
            telemetryRecorder.recordEvent('cody.editorFeaturesPage', 'view', { metadata: { neoVim: 1 } })
        }
    }, [step, telemetryRecorder])

    return (
        <>
            {step === EditorStep.SetupInstructions && (
                <>
                    <div className="pb-3 border-bottom">
                        <H2>Setup instructions for Neovim</H2>
                    </div>

                    <div className={classNames('pt-3 px-3', styles.instructionsContainer)}>
                        <div className={classNames('d-flex flex-column border-bottom')}>
                            <div className="d-flex align-items-center">
                                <div className="mr-1">
                                    <div className={classNames('mr-2', styles.step)}>1</div>
                                </div>
                                <div>
                                    <Text className="mb-1" weight="bold">
                                        Open the plugin repo on GitHub
                                    </Text>
                                    <Text className="text-muted mb-0" size="small">
                                        Follow the instructions detailed in the <strong>readme.md</strong> file to
                                        install the plugin.
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
                                            editor: 'NeoVim',
                                        })
                                        telemetryRecorder.recordEvent('cody.onboarding.openMarketplace', 'click', {
                                            metadata: { neoVim: 1 },
                                        })
                                        window.location.href = marketplaceUrl
                                    }}
                                >
                                    Navigate to GitHub repo
                                </ButtonLink>
                                <img
                                    alt="Neovim Repo"
                                    className="mt-2 m-auto"
                                    width="70%"
                                    src="https://storage.googleapis.com/sourcegraph-assets/NeoVimInstructions/NeovimStep1.png"
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

export function NeoVimTabInstructions({ telemetryRecorder }: { telemetryRecorder: TelemetryRecorder }): JSX.Element {
    const marketplaceUrl = 'https://github.com/sourcegraph/sg.nvim#setup'

    return (
        <div className={styles.ideContainer}>
            <Text className={classNames('mb-0', styles.install)}>Install</Text>
            <Link
                to={marketplaceUrl}
                target="_blank"
                rel="noreferrer"
                className="d-inline-flex align-items-center"
                onClick={event => {
                    event.preventDefault()
                    EVENT_LOGGER.log(EventName.CODY_EDITOR_SETUP_OPEN_MARKETPLACE, {
                        editor: 'NeoVim',
                    })
                    telemetryRecorder.recordEvent('cody.onboarding.openMarketplace', 'click', {
                        metadata: { neoVim: 1 },
                    })
                    window.location.href = marketplaceUrl
                }}
            >
                <div className={styles.ideLink}>View the GitHub repo</div>
                <Icon
                    className={classNames('ml-1', styles.ideLink)}
                    role="img"
                    aria-label="Open in a new tab"
                    svgPath={mdiOpenInNew}
                />
            </Link>
            <Text className={classNames('mb-0', styles.ideTextLight)}>
                Follow the instructions in <br /> the readme.md
            </Text>
        </div>
    )
}
