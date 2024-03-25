import React, { forwardRef, useEffect, useRef, useState } from 'react'

import classNames from 'classnames'
import { useMergeRefs } from 'use-callback-ref'

import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Button, type ForwardReferenceComponent, H3 } from '@sourcegraph/wildcard'

import { CodeInsightExampleCard } from '../../../../getting-started/components/code-insights-examples/code-insight-example-card/CodeInsightExampleCard'
import { EXAMPLES } from '../examples'

import styles from './CodeInsightsExamplesSlider.module.scss'

interface CodeInsightsExamplesSliderProps extends TelemetryProps, TelemetryV2Props {}

export const CodeInsightsExamplesSlider: React.FunctionComponent<
    React.PropsWithChildren<CodeInsightsExamplesSliderProps>
> = props => {
    const { telemetryService, telemetryRecorder } = props
    const itemElementReferences = useRef<Map<number, HTMLElement | null>>(new Map())
    const [activeExampleIndex, setActiveExampleIndex] = useState<number>(0)

    const handleBackClick = (): void => {
        const nextActiveExampleIndex = activeExampleIndex - 1
        const nextElementReference = itemElementReferences.current.get(nextActiveExampleIndex)

        if (nextElementReference) {
            nextElementReference.scrollIntoView({ block: 'nearest', inline: 'start' })
            telemetryService.log('CloudCodeInsightsGetStartedUseCase')
            telemetryRecorder.recordEvent('cloudInsights.getStarted.useCase.back', 'click')
        }
    }

    const handleForwardClick = (): void => {
        const nextActiveExampleIndex = activeExampleIndex + 1
        const nextElementReference = itemElementReferences.current.get(nextActiveExampleIndex)

        if (nextElementReference) {
            nextElementReference.scrollIntoView({ block: 'nearest', inline: 'start' })
            telemetryService.log('CloudCodeInsightsGetStartedUseCase')
            telemetryRecorder.recordEvent('cloudInsights.getStarted.useCase.forward', 'click')
        }
    }

    const registerReferenceElement = (index: number, elementReference: HTMLElement | null): void => {
        itemElementReferences.current.set(index, elementReference)
    }

    const activeExample = EXAMPLES[activeExampleIndex]

    return (
        <section className={styles.sliderSection}>
            <header className={styles.header}>
                <Button
                    variant="icon"
                    className={styles.headerControl}
                    onClick={handleBackClick}
                    disabled={activeExampleIndex === 0}
                >
                    <ArrowIcon side="left" />
                </Button>

                <H3 className={styles.headerTitle}>{activeExample.content.title}</H3>

                <Button
                    variant="icon"
                    className={styles.headerControl}
                    onClick={handleForwardClick}
                    disabled={activeExampleIndex === EXAMPLES.length - 1}
                >
                    <ArrowIcon side="right" />
                </Button>
            </header>

            <ul className={styles.sliderList}>
                {EXAMPLES.map((example, index) => (
                    <CodeInsightsExamplesSliderItem
                        key={index}
                        as="li"
                        ref={element => registerReferenceElement(index, element)}
                        className={styles.sliderItem}
                        onFullIntersection={() => setActiveExampleIndex(index)}
                    >
                        <CodeInsightExampleCard
                            {...example}
                            className={styles.sliderChart}
                            telemetryService={telemetryService}
                            telemetryRecorder={telemetryRecorder}
                        />
                    </CodeInsightsExamplesSliderItem>
                ))}
            </ul>

            <ul className={styles.sliderDots}>
                {EXAMPLES.map((example, index) => (
                    <li
                        key={index}
                        className={classNames(styles.sliderDot, {
                            [styles.sliderDotActive]: index === activeExampleIndex,
                        })}
                    />
                ))}
            </ul>
        </section>
    )
}

interface CodeInsightsExamplesSliderItemProps {
    onFullIntersection: () => void
}

export const CodeInsightsExamplesSliderItem = forwardRef((props, publicReference) => {
    const { onFullIntersection, as: Comp = 'div', ...attributes } = props
    const localReference = useRef<HTMLDivElement>(null)
    const mergedReference = useMergeRefs([localReference, publicReference])

    const onFullIntersectionReference = useRef(onFullIntersection)
    onFullIntersectionReference.current = onFullIntersection

    useEffect(() => {
        const element = localReference.current

        if (!element) {
            return
        }

        const observer = new IntersectionObserver(
            entries => {
                for (const entry of entries) {
                    if (entry.isIntersecting) {
                        onFullIntersectionReference.current()
                    }
                }
            },
            { threshold: 0.5 }
        )

        observer.observe(element)

        return () => observer.disconnect()
    }, [])

    return <Comp ref={mergedReference} {...attributes} />
}) as ForwardReferenceComponent<'div', CodeInsightsExamplesSliderItemProps>
CodeInsightsExamplesSliderItem.displayName = 'CodeInsightsExamplesSliderItem'

interface ArrowIconProps {
    side: 'right' | 'left'
}

const ArrowIcon: React.FunctionComponent<React.PropsWithChildren<ArrowIconProps>> = props => {
    const { side } = props
    const rotate = `rotate(${side === 'left' ? 180 : 0}deg)`

    return (
        <svg
            width="24"
            height="24"
            viewBox="0 0 24 24"
            fill="none"
            /* eslint-disable-next-line react/forbid-dom-props */
            style={{ transform: rotate }}
            xmlns="http://www.w3.org/2000/svg"
            className="inline-icon"
        >
            <path
                fill="var(--body-color)"
                fillRule="evenodd"
                clipRule="evenodd"
                d="M3.1875 12C3.1875 11.6893 3.43934 11.4375 3.75 11.4375L20.25 11.4375C20.5607 11.4375 20.8125 11.6893 20.8125 12C20.8125 12.3107 20.5607 12.5625 20.25 12.5625L3.75 12.5625C3.43934 12.5625 3.1875 12.3107 3.1875 12Z"
            />
            <path
                fill="var(--body-color)"
                fillRule="evenodd"
                clipRule="evenodd"
                d="M13.1023 4.85225C13.3219 4.63258 13.6781 4.63258 13.8977 4.85225L20.6477 11.6023C20.8674 11.8219 20.8674 12.1781 20.6477 12.3977L13.8977 19.1477C13.6781 19.3674 13.3219 19.3674 13.1023 19.1477C12.8826 18.9281 12.8826 18.5719 13.1023 18.3523L19.4545 12L13.1023 5.64775C12.8826 5.42808 12.8826 5.07192 13.1023 4.85225Z"
            />
        </svg>
    )
}
