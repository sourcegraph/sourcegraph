import React, { useEffect, useState } from 'react'

import classNames from 'classnames'
import { useNavigate } from 'react-router-dom'

import { useTemporarySetting } from '@sourcegraph/shared/src/settings/temporary'
import type { TelemetryRecorder, TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import { Button, H2, H5, Modal, Text, useSearchParameters } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../auth'
import { HubSpotForm } from '../../marketing/components/HubSpotForm'
import { eventLogger } from '../../tracking/eventLogger'
import { EventName } from '../../util/constants'

import { JetBrainsInstructions } from './instructions/JetBrains'
import { NeoVimInstructions } from './instructions/NeoVim'
import { VSCodeInstructions } from './instructions/VsCode'

import styles from './CodyOnboarding.module.scss'

export interface IEditor {
    id: number // a unique number identifier for telemetry
    icon: string
    name: string
    publisher: string
    releaseStage: string
    docs?: string
    instructions?: React.FC<{ onBack?: () => void; onClose: () => void; showStep?: number }>
}

export const editorGroups: IEditor[][] = [
    [
        {
            id: 1,
            icon: 'VsCode',
            name: 'VS Code',
            publisher: 'Microsoft',
            releaseStage: 'Stable',
            docs: 'https://sourcegraph.com/docs/cody/clients/install-vscode',
            instructions: VSCodeInstructions,
        },
        {
            id: 2,
            icon: 'IntelliJ',
            name: 'IntelliJ IDEA',
            publisher: 'JetBrains',
            releaseStage: 'Beta',
            docs: 'https://sourcegraph.com/docs/cody/clients/install-jetbrains',
            instructions: JetBrainsInstructions,
        },
        {
            id: 3,
            icon: 'PhpStorm',
            name: 'PhpStorm ',
            publisher: 'JetBrains',
            releaseStage: 'Beta',
            docs: 'https://sourcegraph.com/docs/cody/clients/install-jetbrains',
            instructions: JetBrainsInstructions,
        },
        {
            id: 4,
            icon: 'PyCharm',
            name: 'PyCharm',
            publisher: 'JetBrains',
            releaseStage: 'Beta',
            docs: 'https://sourcegraph.com/docs/cody/clients/install-jetbrains',
            instructions: JetBrainsInstructions,
        },
    ],
    [
        {
            id: 5,
            icon: 'WebStorm',
            name: 'WebStorm',
            publisher: 'JetBrains',
            releaseStage: 'Beta',
            docs: 'https://sourcegraph.com/docs/cody/clients/install-jetbrains',
            instructions: JetBrainsInstructions,
        },
        {
            id: 6,
            icon: 'RubyMine',
            name: 'RubyMine',
            publisher: 'JetBrains',
            releaseStage: 'Beta',
            docs: 'https://sourcegraph.com/docs/cody/clients/install-jetbrains',
            instructions: JetBrainsInstructions,
        },
        {
            id: 7,
            icon: 'GoLand',
            name: 'GoLand',
            publisher: 'JetBrains',
            releaseStage: 'Beta',
            docs: 'https://sourcegraph.com/docs/cody/clients/install-jetbrains',
            instructions: JetBrainsInstructions,
        },
        {
            id: 8,
            icon: 'AndroidStudio',
            name: 'Android Studio',
            publisher: 'Google',
            releaseStage: 'Beta',
            docs: 'https://sourcegraph.com/docs/cody/clients/install-jetbrains',
            instructions: JetBrainsInstructions,
        },
    ],
    [
        {
            id: 9,
            icon: 'NeoVim',
            name: 'Neovim',
            publisher: 'Neovim Team',
            releaseStage: 'Experimental',
            docs: 'https://sourcegraph.com/docs/cody/clients/install-neovim',
            instructions: NeoVimInstructions,
        },
    ],
]

function formatUseCase(action: { work: boolean; personal: boolean }): string {
    const useCases = []
    for (const [key, value] of Object.entries(action)) {
        if (value) {
            useCases.push(key)
        }
    }
    return useCases.length === 0 ? 'none' : useCases.join(',')
}

interface CodyOnboardingProps extends TelemetryV2Props {
    authenticatedUser: AuthenticatedUser | null
}

export function CodyOnboarding({ authenticatedUser, telemetryRecorder }: CodyOnboardingProps): JSX.Element | null {
    const [showEditorStep, setShowEditorStep] = useState(false)
    const [completed = false, setOnboardingCompleted] = useTemporarySetting('cody.onboarding.completed', false)
    // steps start from 0
    const [step = -1, setOnboardingStep] = useTemporarySetting('cody.onboarding.step', 0)

    const onNext = (): void => setOnboardingStep(currentsStep => (currentsStep || 0) + 1)

    const parameters = useSearchParameters()
    const enrollPro = parameters.get('pro') === 'true'
    const returnToURL = parameters.get('returnTo')

    const navigate = useNavigate()

    useEffect(() => {
        if (completed && returnToURL) {
            navigate(returnToURL)
        }
    }, [completed, returnToURL, navigate])

    if (completed && returnToURL) {
        return null
    }

    if (!showEditorStep && (completed || step === -1 || step > 1)) {
        return null
    }

    if (!authenticatedUser) {
        return null
    }

    return (
        <Modal
            isOpen={true}
            position="center"
            aria-label="Cody Onboarding"
            className={styles.modal}
            containerClassName={styles.root}
        >
            {step === 0 && <WelcomeStep onNext={onNext} pro={enrollPro} telemetryRecorder={telemetryRecorder} />}
            {step === 1 && (
                <PurposeStep
                    authenticatedUser={authenticatedUser}
                    onNext={() => {
                        onNext()
                        setOnboardingCompleted(true)
                        setShowEditorStep(true)
                    }}
                    pro={enrollPro}
                    telemetryRecorder={telemetryRecorder}
                />
            )}
            {showEditorStep && (
                <EditorStep
                    onCompleted={() => {
                        setShowEditorStep(false)
                    }}
                    pro={enrollPro}
                    telemetryRecorder={telemetryRecorder}
                />
            )}
        </Modal>
    )
}

function WelcomeStep({
    onNext,
    pro,
    telemetryRecorder,
}: {
    onNext: () => void
    pro: boolean
    telemetryRecorder: TelemetryRecorder
}): JSX.Element {
    const [show, setShow] = useState(false)
    const isLightTheme = useIsLightTheme()
    useEffect(() => {
        eventLogger.log(
            EventName.CODY_ONBOARDING_WELCOME_VIEWED,
            { tier: pro ? 'pro' : 'free' },
            { tier: pro ? 'pro' : 'free' }
        )
        telemetryRecorder.recordEvent('cody.onboarding.welcome', 'view', { metadata: { tier: pro ? 1 : 0 } })
    }, [pro, telemetryRecorder])

    useEffect(() => {
        // theme is not ready on first render, it defaults to system theme.
        // so we need to wait a bit before showing the welcome video.
        setTimeout(() => {
            setShow(true)
        }, 500)
    }, [])

    return (
        <div className={classNames('d-flex flex-column align-items-center p-5')}>
            {show ? (
                <>
                    <video width="180" className={classNames('mb-5', styles.welcomeVideo)} autoPlay={true} muted={true}>
                        <source
                            src={
                                isLightTheme
                                    ? 'https://storage.googleapis.com/sourcegraph-assets/hiCodyWhite.mp4'
                                    : 'https://storage.googleapis.com/sourcegraph-assets/hiCodyDark.mp4'
                            }
                            type="video/mp4"
                        />
                        Your browser does not support the video tag.
                    </video>
                    <Text className={classNames('mb-4 pb-4', styles.fadeIn, styles.fadeSecond, styles.welcomeSubtitle)}>
                        Ready to breeze through the basics and get comfortable with Cody
                        {pro ? ' Pro' : ''}?
                    </Text>
                    <Button
                        onClick={onNext}
                        variant="primary"
                        size="lg"
                        className={classNames(styles.fadeIn, styles.fadeThird)}
                    >
                        Sure, let's dive in!
                    </Button>
                </>
            ) : (
                <div className={styles.blankPlaceholder} />
            )}
        </div>
    )
}

function PurposeStep({
    onNext,
    pro,
    authenticatedUser,
    telemetryRecorder,
}: {
    onNext: () => void
    pro: boolean
    authenticatedUser: AuthenticatedUser
    telemetryRecorder: TelemetryRecorder
}): JSX.Element {
    useEffect(() => {
        eventLogger.log(
            EventName.CODY_ONBOARDING_PURPOSE_VIEWED,
            { tier: pro ? 'pro' : 'free' },
            { tier: pro ? 'pro' : 'free' }
        )
        telemetryRecorder.recordEvent('cody.onboarding.purpose', 'view', { metadata: { tier: pro ? 1 : 0 } })
    }, [pro, telemetryRecorder])

    const primaryEmail = authenticatedUser.emails.find(email => email.isPrimary)?.email

    const handleFormSubmit = (form: HTMLFormElement): void => {
        const workInput = form[0].querySelector('input[name="using_cody_for_work"]') as HTMLInputElement
        const personalInput = form[0].querySelector('input[name="using_cody_for_personal"]') as HTMLInputElement

        const useCase = formatUseCase({
            work: workInput.checked,
            personal: personalInput.checked,
        })
        eventLogger.log(EventName.CODY_ONBOARDING_PURPOSE_SELECTED, { useCase }, { useCase })
        telemetryRecorder.recordEvent('cody.onboarding.purpose', 'select', {
            metadata: { workUseCase: workInput.checked ? 1 : 0, personalUseCase: personalInput.checked ? 1 : 0 },
        })
    }

    return (
        <>
            <div className="border-bottom pb-3 mb-3">
                <H2 className="mb-1">What are you using Cody for?</H2>
                <Text className="mb-0 text-muted" size="small">
                    This will allow us to understand our audience better and guide your journey
                </Text>
            </div>
            <div className="d-flex align-items-center border-bottom mb-3 pb-3 justify-content-center">
                <HubSpotForm
                    formId="85548efc-a879-4553-9ef0-a8da8fdcf541"
                    onFormSubmitted={() => {
                        onNext()
                    }}
                    onFormLoadError={() => {
                        onNext()
                    }}
                    userId={authenticatedUser.id}
                    userEmail={primaryEmail}
                    masterFormName="qualificationSurvey"
                    onFormSubmit={handleFormSubmit}
                />
            </div>
        </>
    )
}

function EditorStep({
    onCompleted,
    pro,
    telemetryRecorder,
}: {
    onCompleted: () => void
    pro: boolean
    telemetryRecorder: TelemetryRecorder
}): JSX.Element {
    useEffect(() => {
        eventLogger.log(
            EventName.CODY_ONBOARDING_CHOOSE_EDITOR_VIEWED,
            { tier: pro ? 'pro' : 'free' },
            { tier: pro ? 'pro' : 'free' }
        )
        telemetryRecorder.recordEvent('cody.onboarding.chooseEditor', 'view', { metadata: { tier: pro ? 1 : 0 } })
    }, [pro, telemetryRecorder])

    const [editor, setEditor] = useState<null | IEditor>(null)

    const onBack = (): void => setEditor(null)

    if (editor?.instructions) {
        const Instructions = editor.instructions

        return <Instructions onBack={onBack} onClose={onCompleted} />
    }

    return (
        <>
            <div className="border-bottom pb-3 mb-3">
                <H2 className="mb-1">Choose your editor</H2>
                <Text className="mb-0 text-muted" size="small">
                    Most of Cody experience happens in the IDE. Let's get that set up.
                </Text>
            </div>
            <div className="mb-3 border-bottom pb-3">
                {editorGroups.map((group, index) => (
                    <div
                        key={index}
                        className={classNames('d-flex mt-3', styles.responsiveContainer, {
                            'border-bottom pb-3': index < group.length - 1,
                        })}
                    >
                        {group.map((editor, index) => (
                            <div
                                key={index}
                                className={classNames('d-flex flex-column flex-1 p-3 cursor-pointer', styles.ideGrid, {
                                    'border-left': index !== 0,
                                })}
                                role="button"
                                tabIndex={0}
                                onKeyDown={() => {
                                    setEditor(editor)

                                    eventLogger.log(
                                        EventName.CODY_ONBOARDING_CHOOSE_EDITOR_SELECTED,
                                        {
                                            tier: pro ? 'pro' : 'free',
                                            editor,
                                        },
                                        {
                                            tier: pro ? 'pro' : 'free',
                                            editor,
                                        }
                                    )
                                    telemetryRecorder.recordEvent('cody.onboarding.chooseEditor', 'select', {
                                        metadata: { tier: pro ? 1 : 0, editor: editor.id },
                                    })
                                }}
                                onClick={() => {
                                    eventLogger.log(
                                        EventName.CODY_ONBOARDING_CHOOSE_EDITOR_SELECTED,
                                        {
                                            tier: pro ? 'pro' : 'free',
                                            editor,
                                        },
                                        {
                                            tier: pro ? 'pro' : 'free',
                                            editor,
                                        }
                                    )
                                    telemetryRecorder.recordEvent('cody.onboarding.chooseEditor', 'select', {
                                        metadata: { tier: pro ? 1 : 0, editor: editor.id },
                                    })
                                    setEditor(editor)
                                }}
                            >
                                <div className="d-flex align-items-center">
                                    <div>
                                        <img
                                            alt={editor.name}
                                            src={`https://storage.googleapis.com/sourcegraph-assets/ideIcons/ideIcon${editor.icon}.svg`}
                                            width={34}
                                            className="mr-3"
                                        />
                                    </div>
                                    <div>
                                        <Text className="text-muted mb-0 text-truncate" size="small">
                                            {editor.publisher}
                                        </Text>
                                        <Text className={classNames('mb-0', styles.ideName)}>{editor.name}</Text>
                                        <H5 className={styles.releaseStage}>{editor.releaseStage}</H5>
                                    </div>
                                </div>
                            </div>
                        ))}
                        {group.length < 4
                            ? [...new Array(4 - group.length)].map((_, index) => (
                                  <div key={index} className="flex-1 p-3" />
                              ))
                            : null}
                    </div>
                ))}
            </div>
            <div className="d-flex justify-content-end align-items-center">
                <Text
                    className="mb-0 text-muted cursor-pointer"
                    size="small"
                    onClick={() => {
                        onCompleted()
                        eventLogger.log(
                            EventName.CODY_ONBOARDING_CHOOSE_EDITOR_SKIPPED,
                            { tier: pro ? 'pro' : 'free' },
                            { tier: pro ? 'pro' : 'free' }
                        )
                        telemetryRecorder.recordEvent('cody.onboarding.chooseEditor', 'skip', {
                            metadata: { tier: pro ? 1 : 0 },
                        })
                    }}
                >
                    Skip for now
                </Text>
            </div>
        </>
    )
}
