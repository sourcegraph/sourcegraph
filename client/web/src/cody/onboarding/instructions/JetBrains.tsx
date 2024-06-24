import { useState, useEffect } from 'react'

import { mdiOpenInNew } from '@mdi/js'
import classNames from 'classnames'

import type { TelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { EVENT_LOGGER } from '@sourcegraph/shared/src/telemetry/web/eventLogger'
import { Button, H2, Link, Text, Icon } from '@sourcegraph/wildcard'

import { EventName } from '../../../util/constants'
import { EditorStep } from '../../management/CodyManagementPage'

import { CodyFeatures } from './CodyFeatures'

import styles from '../CodyOnboarding.module.scss'

export function JetBrainsInstructions({
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
    const marketplaceUrl = 'https://plugins.jetbrains.com/plugin/9682-sourcegraph-cody--code-search'

    useEffect(() => {
        if (step === EditorStep.SetupInstructions) {
            EVENT_LOGGER.log(EventName.CODY_EDITOR_SETUP_VIEWED, { editor: 'JetBrains' })
            telemetryRecorder.recordEvent('cody.editorSetupPage', 'view', { metadata: { jetBrains: 1 } })
        } else if (step === EditorStep.CodyFeatures) {
            EVENT_LOGGER.log(EventName.CODY_EDITOR_FEATURES_VIEWED, { editor: 'JetBrains' })
            telemetryRecorder.recordEvent('cody.editorFeaturesPage', 'view', { metadata: { jetBrains: 1 } })
        }
    }, [step, telemetryRecorder])

    return (
        <>
            {step === EditorStep.SetupInstructions && (
                <>
                    <div className="pb-3 border-bottom">
                        <H2>Setup instructions for JetBrains</H2>
                    </div>

                    <div className={classNames('pt-3 px-3', styles.instructionsContainer)}>
                        <div className={classNames('d-flex flex-column border-bottom')}>
                            <div className="d-flex align-items-center">
                                <div className="mr-1">
                                    <div className={classNames('mr-2', styles.step)}>1</div>
                                </div>
                                <div>
                                    <Text className="mb-1" weight="bold">
                                        Open the Plugins Page (or via the{' '}
                                        <Link
                                            to={marketplaceUrl}
                                            target="_blank"
                                            rel="noopener"
                                            onClick={event => {
                                                event.preventDefault()
                                                EVENT_LOGGER.log(EventName.CODY_EDITOR_SETUP_OPEN_MARKETPLACE, {
                                                    editor: 'JetBrains',
                                                })
                                                telemetryRecorder.recordEvent(
                                                    'cody.onboarding.openMarketplace',
                                                    'click',
                                                    { metadata: { jetBrains: 1 } }
                                                )
                                                window.location.href = marketplaceUrl
                                            }}
                                        >
                                            JetBrains Marketplace
                                        </Link>
                                        )
                                    </Text>
                                    <Text className="text-muted mb-0" size="small">
                                        Click the cog [⚙️] icon in the top right corner of your IDE and select{' '}
                                        <strong>Plugins</strong>
                                        <br />
                                        Alternatively, go to the settings option (
                                        <strong> [⌘] + [,] on macOS, or File → Settings on Windows </strong>), then
                                        select "Plugins" from the menu on the left.
                                    </Text>
                                </div>
                            </div>
                            <img
                                alt="JetBrains Menu"
                                className="mt-2 m-auto"
                                width="70%"
                                src="https://storage.googleapis.com/sourcegraph-assets/jetBrainsInstructions/jetBrainsMenu.png"
                            />
                        </div>

                        <div className="mt-3 d-flex flex-column border-bottom">
                            <div className="d-flex align-items-center">
                                <div className="mr-1">
                                    <div className={classNames('mr-2', styles.step)}>2</div>
                                </div>
                                <div>
                                    <Text className="mb-1" weight="bold">
                                        Install the Cody plugin
                                    </Text>
                                    <Text className="text-muted mb-0" size="small">
                                        Type "Cody" in the search bar and <strong>install</strong> the plugin.
                                    </Text>
                                </div>
                            </div>
                            <img
                                alt="jetBrains Menu"
                                className="mt-2 m-auto"
                                width="70%"
                                src="https://storage.googleapis.com/sourcegraph-assets/jetBrainsInstructions/jetBrainsPluginList.png"
                            />
                        </div>

                        <div className="mt-3 d-flex flex-column border-bottom">
                            <div className="d-flex align-items-center">
                                <div className="mr-1">
                                    <div className={classNames('mr-2', styles.step)}>3</div>
                                </div>
                                <div>
                                    <Text className="mb-1" weight="bold">
                                        Open the plugin and log in
                                    </Text>
                                    <Text className="text-muted mb-0" size="small">
                                        Cody will be available on the right side of your IDE. Click the Cody icon to
                                        open the sidebar and login.
                                        <br />
                                        Log in with the same method you use to create this account.
                                    </Text>
                                </div>
                            </div>
                            <img
                                alt="jetBrains Menu"
                                className="mt-2 m-auto"
                                width="70%"
                                src="https://storage.googleapis.com/sourcegraph-assets/jetBrainsInstructions/jetBrainsOnboarding.png"
                            />
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

export function JetBrainsTabInstructions({
    telemetryRecorder,
}: {
    onBack?: () => void

    telemetryRecorder: TelemetryRecorder
}): JSX.Element {
    const marketplaceUrl = 'https://plugins.jetbrains.com/plugin/9682-sourcegraph-cody--code-search'

    const icons = [
        { id: 1, icon: 'PyCharm' },
        { id: 2, icon: 'PhpStorm' },
        { id: 3, icon: 'WebStorm' },
        { id: 4, icon: 'RubyMine' },
        { id: 5, icon: 'GoLand' },
        { id: 6, icon: 'RubyMine' },
    ]

    return (
        <div className="w-100">
            <div className={styles.ideContainer}>
                <Text className={classNames('mb-0', styles.install)}>Install</Text>
                <div className={classNames('d-flex flex-column gap-0', styles.ideTextLight)}>
                    <Text className="mb-0">1. Open your IDE</Text>
                    <Text className="mb-0">2. {'Settings > plugins (CMD + ,)'}</Text>
                    <Text className="mb-0">3. Search “Sourcegraph”</Text>
                </div>
                <div className={styles.ideOr}>or</div>
                <Link
                    to={marketplaceUrl}
                    target="_blank"
                    rel="noopener"
                    className="d-inline-flex align-items-center"
                    onClick={event => {
                        event.preventDefault()
                        EVENT_LOGGER.log(EventName.CODY_EDITOR_SETUP_OPEN_MARKETPLACE, {
                            editor: 'JetBrains',
                        })
                        telemetryRecorder.recordEvent('cody.onboarding.openMarketplace', 'click', {
                            metadata: { jetBrains: 1 },
                        })
                        window.location.href = marketplaceUrl
                    }}
                >
                    <div className={styles.ideLink}> Install from the marketplace</div>
                    <Icon
                        className={classNames('ml-1', styles.ideLink)}
                        role="img"
                        aria-label="Open in a new tab"
                        svgPath={mdiOpenInNew}
                    />
                </Link>
            </div>
            <div className={classNames('w-100', styles.ideWorks)}>
                <Text>Works with:</Text>
                <div className={styles.ideLogoContainer}>
                    {icons.map(icon => (
                        <img
                            key={icon.id}
                            src={`https://storage.googleapis.com/sourcegraph-assets/ideIcons/ideIcon${icon.icon}.svg`}
                            alt="PS"
                            aria-hidden={true}
                            width={36.725}
                            height={36.725}
                        />
                    ))}
                </div>
            </div>
        </div>
    )
}
