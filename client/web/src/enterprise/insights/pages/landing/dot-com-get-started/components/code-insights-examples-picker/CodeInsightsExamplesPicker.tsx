import { type FunctionComponent, useLayoutEffect, useState } from 'react'

import classNames from 'classnames'
import { throttle } from 'lodash'

import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { Button, Card, CardBody, Link, H2 } from '@sourcegraph/wildcard'

import { CodeInsightExampleCard } from '../../../getting-started/components/code-insights-examples/code-insight-example-card/CodeInsightExampleCard'

import { CodeInsightsExamplesSlider } from './code-insights-examples-slider/CodeInsightsExamplesSlider'
import { EXAMPLES } from './examples'

import styles from './CodeInsightsExamplesPicker.module.scss'

interface CodeInsightsExamplesPickerProps extends TelemetryProps, TelemetryV2Props {}

export const CodeInsightsExamplesPicker: FunctionComponent<CodeInsightsExamplesPickerProps> = props => {
    const { telemetryService, telemetryRecorder } = props
    const [activeExampleIndex, setActiveExampleIndex] = useState(0)
    const [windowSize, setWindowSize] = useState(0)

    useLayoutEffect(() => {
        setWindowSize(window.innerWidth)
        const handleWindowResize = throttle(() => setWindowSize(window.innerWidth), 200)

        window.addEventListener('resize', handleWindowResize)

        return () => window.removeEventListener('resize', handleWindowResize)
    }, [])

    const handleUseCaseButtonClick = (useCaseIndex: number): void => {
        setActiveExampleIndex(useCaseIndex)
        telemetryService.log('CloudCodeInsightsGetStartedUseCase')
        telemetryRecorder.recordEvent('codeInsights.getStarted.useCase', 'click')
    }

    const isMobileLayout = windowSize <= 900

    return (
        <Card as={CardBody} className={classNames(styles.root, { [styles.rootMobile]: isMobileLayout })}>
            <div className={styles.section}>
                <H2>Use Code Insights to...</H2>

                <Link to="/help/code_insights/references/common_use_cases" target="_blank" rel="noopener">
                    See more use cases
                </Link>

                {!isMobileLayout && (
                    <ul className={styles.list}>
                        {EXAMPLES.map((example, index) => (
                            <li key={index}>
                                <Button
                                    outline={index !== activeExampleIndex}
                                    variant={index === activeExampleIndex ? 'secondary' : undefined}
                                    className={classNames(styles.button, {
                                        [styles.buttonActive]: index === activeExampleIndex,
                                    })}
                                    onClick={() => handleUseCaseButtonClick(index)}
                                >
                                    {example.description}
                                    <svg
                                        className={styles.buttonSvg}
                                        viewBox="0 0 16 36"
                                        fill="none"
                                        xmlns="http://www.w3.org/2000/svg"
                                    >
                                        <path d="M0.443444 0H0V36H0.443444C2.04896 36 3.55682 35.229 4.49684 33.9275L15.1543 19.171C15.6591 18.472 15.6591 17.528 15.1543 16.829L4.49684 2.07255C3.55682 0.770985 2.04896 0 0.443444 0Z" />
                                    </svg>
                                </Button>
                            </li>
                        ))}
                    </ul>
                )}
            </div>

            {!isMobileLayout && (
                <CodeInsightExampleCard
                    {...EXAMPLES[activeExampleIndex]}
                    className={styles.section}
                    telemetryService={telemetryService}
                    telemetryRecorder={telemetryRecorder}
                />
            )}

            {isMobileLayout && (
                <CodeInsightsExamplesSlider telemetryService={telemetryService} telemetryRecorder={telemetryRecorder} />
            )}
        </Card>
    )
}
