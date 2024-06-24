import { useEffect, useState } from 'react'

import { mdiArrowLeft } from '@mdi/js'
import classNames from 'classnames'

import type { TelemetryRecorder } from '@sourcegraph/shared/src/telemetry'
import { H2, Text, H5, Icon, Button } from '@sourcegraph/wildcard'

import { editorGroups, newEditorGroups } from '../editorGroups'

import type { IEditor } from './CodyOnboarding'

import styles from './CodyOnboarding.module.scss'

export function EditorStep({
    onCompleted,
    pro,
    telemetryRecorder,
    toggleView,
}: {
    onCompleted: () => void
    pro: boolean
    toggleView?: boolean
    telemetryRecorder: TelemetryRecorder
}): JSX.Element {
    useEffect(() => {
        telemetryRecorder.recordEvent('cody.onboarding.chooseEditor', 'view', { metadata: { tier: pro ? 1 : 0 } })
        telemetryRecorder.recordEvent('cody.onboarding.chooseEditor', 'enroll', { metadata: { tier: pro ? 1 : 0 } })
    }, [pro, telemetryRecorder])

    const [editor, setEditor] = useState<null | IEditor>(null)

    const onBack = (): void => setEditor(null)

    if (editor?.instructions) {
        const Instructions = editor.instructions

        return <Instructions onBack={onBack} onClose={onCompleted} telemetryRecorder={telemetryRecorder} />
    }
    const handleEventLog = (editor: IEditor, canSetEditor?: boolean): void => {
        if (canSetEditor) {
            setEditor(editor)
        }
        telemetryRecorder.recordEvent('cody.onboarding.chooseEditor', 'select', {
            metadata: { tier: pro ? 1 : 0, editor: editor.id },
        })
    }
    const handleEventSkip = (): void => {
        onCompleted()
        telemetryRecorder.recordEvent('cody.onboarding.chooseEditor', 'skip', {
            metadata: { tier: pro ? 1 : 0 },
        })
    }
    const releaseStageStyles: { [key: number]: string } = {
        1: styles.releaseStageVs,
        2: styles.releaseStageJetbrains,
        3: styles.releaseStageNeovim,
    }

    return (
        <>
            {toggleView ? (
                <>
                    <div className={classNames(styles.ideHeader, toggleView ? styles.ideBorderColor : 'border-bottom')}>
                        <H2 className={classNames(toggleView && styles.ideHeaderAlt)}>Choose your editor</H2>
                    </div>
                    <div className="mb-3 border-bottom pb-3">
                        <div className={classNames('d-flex flex-row', styles.responsiveContainer)}>
                            {newEditorGroups.map(({ instructions: EditorInstructions, ...editor }) => (
                                <div
                                    key={editor.id}
                                    className={classNames(
                                        'd-flex flex-1 flex-column',
                                        editor.id === 1 ? styles.ideGuideRtb : styles.ideGuide
                                    )}
                                >
                                    <div
                                        className={classNames('d-flex flex-column', styles.ideMainLogo)}
                                        role="button"
                                        tabIndex={0}
                                        onKeyDown={() => handleEventLog(editor)}
                                        onClick={() => handleEventLog(editor)}
                                    >
                                        <div className="d-flex flex-row align-items-center">
                                            <div>
                                                <img
                                                    alt={editor.name}
                                                    src={
                                                        editor.icon
                                                            ? `https://storage.googleapis.com/sourcegraph-assets/ideIcons/ideIcon${editor.icon}.svg`
                                                            : `${window.context?.assetsRoot}/img/jetbrains-logo.svg`
                                                    }
                                                    width={editor.width}
                                                    height={editor.height}
                                                    className={classNames(styles.ideLogo)}
                                                />
                                            </div>
                                            <div>
                                                <Text className="text-muted mb-0 text-truncate" size="base">
                                                    {editor.publisher}
                                                </Text>
                                                <Text className={classNames('mb-0', styles.ideNameAlt)}>
                                                    {editor.name}
                                                </Text>
                                                <H5
                                                    className={classNames(
                                                        styles.releaseStage,
                                                        releaseStageStyles[editor.id]
                                                    )}
                                                >
                                                    {editor.releaseStage}
                                                </H5>
                                            </div>
                                        </div>
                                    </div>
                                    <div className="d-flex">
                                        {EditorInstructions && (
                                            <EditorInstructions
                                                onClose={onCompleted}
                                                telemetryRecorder={telemetryRecorder}
                                            />
                                        )}
                                    </div>
                                </div>
                            ))}
                        </div>
                    </div>
                </>
            ) : (
                <>
                    <div className="border-bottom pb-3 mb-3">
                        <H2 className="mb-1">Choose your editor</H2>
                        <Text className="mb-0 text-muted" size="small">
                            Most of Cody experience happens in the IDE. Let's get that set up.
                        </Text>
                    </div>
                    <div className="mb-3 border-bottom pb-3">
                        {editorGroups.map((group, groupIndex) => (
                            <div
                                key={group[0].id}
                                className={classNames('d-flex mt-3', styles.responsiveContainer, {
                                    'border-bottom pb-3': groupIndex < group.length - 1,
                                })}
                            >
                                {group.map((editor, editorIndex) => (
                                    <div
                                        key={editor.id}
                                        className={classNames(
                                            'd-flex flex-column flex-1 p-3 cursor-pointer',
                                            styles.ideGrid,
                                            {
                                                'border-left': editorIndex !== 0,
                                            }
                                        )}
                                        role="button"
                                        tabIndex={0}
                                        onKeyDown={() => handleEventLog(editor, true)}
                                        onClick={() => handleEventLog(editor, true)}
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
                                                <Text className={classNames('mb-0', styles.ideName)}>
                                                    {editor.name}
                                                </Text>
                                                <H5 className={styles.releaseStage}>{editor.releaseStage}</H5>
                                            </div>
                                        </div>
                                    </div>
                                ))}
                                {group.length < 4
                                    ? Array.from(new Array(4 - group.length).keys()).map(item => (
                                          <div key={item} className="flex-1 p-3" />
                                      ))
                                    : null}
                            </div>
                        ))}
                    </div>
                </>
            )}

            <div
                className={classNames(
                    'd-flex justify-content-start align-items-center position-relative',
                    toggleView && styles.editorButtonContainer
                )}
            >
                {toggleView ? (
                    <>
                        <Button
                            variant="secondary"
                            onClick={() => onCompleted()}
                            outline={true}
                            size="sm"
                            className={classNames('mb-0 cursor-pointer d-flex align-items-center', styles.editorButton)}
                        >
                            <Icon aria-hidden={true} svgPath={mdiArrowLeft} className="mr-1" />
                            Back
                        </Button>

                        <Text
                            className={classNames('mb-0 text-muted cursor-pointer', styles.idePositioningAlt)}
                            size="base"
                            onClick={() => handleEventSkip()}
                        >
                            Skip
                        </Text>
                    </>
                ) : (
                    <Text
                        className={classNames('mb-0 text-muted cursor-pointer', styles.idePositioning)}
                        size="small"
                        onClick={() => handleEventSkip()}
                    >
                        Skip for now
                    </Text>
                )}
            </div>
        </>
    )
}
