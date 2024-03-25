import React, { type MouseEvent, useContext, useState } from 'react'

import { mdiContentCopy } from '@mdi/js'
import copy from 'copy-to-clipboard'

import { SyntaxHighlightedSearchQuery } from '@sourcegraph/branded'
import { useExperimentalFeatures } from '@sourcegraph/shared/src/settings/settings'
import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import {
    Button,
    Card,
    CardBody,
    CardText,
    CardTitle,
    Tab,
    TabList,
    TabPanel,
    TabPanels,
    Tabs,
    Icon,
    Link,
    ProductStatusBadge,
    H2,
    Text,
    Tooltip,
} from '@sourcegraph/wildcard'

import { InsightType } from '../../../../../core'
import { encodeCaptureInsightURL } from '../../../../insights/creation/capture-group'
import { encodeSearchInsightUrl } from '../../../../insights/creation/search-insight'
import {
    CodeInsightsLandingPageContext,
    CodeInsightsLandingPageType,
    useLogEventName,
} from '../../../CodeInsightsLandingPageContext'
import { CodeInsightsQueryBlock } from '../code-insights-query-block/CodeInsightsQueryBlock'

import { type Template, getTemplateSections } from './constants'

import styles from './CodeInsightsTemplates.module.scss'

function getTemplateURL(template: Template): string {
    switch (template.type) {
        case InsightType.CaptureGroup: {
            return `/insights/create/capture-group?${encodeCaptureInsightURL(template.templateValues)}`
        }
        case InsightType.SearchBased: {
            return `/insights/create/search?${encodeSearchInsightUrl(template.templateValues)}`
        }
    }
}

interface CodeInsightsTemplates extends TelemetryProps, TelemetryV2Props, React.HTMLAttributes<HTMLElement> {}

export const CodeInsightsTemplates: React.FunctionComponent<React.PropsWithChildren<CodeInsightsTemplates>> = props => {
    const { telemetryService, telemetryRecorder, ...otherProps } = props
    const tabChangePingName = useLogEventName('InsightsGetStartedTabClick')
    const goCodeCheckerTemplates = useExperimentalFeatures(features => features.goCodeCheckerTemplates)
    const templateSections = getTemplateSections(goCodeCheckerTemplates)

    const handleTabChange = (index: number): void => {
        const template = templateSections[index]

        telemetryService.log(tabChangePingName, { tabName: template.title }, { tabName: template.title })
        telemetryRecorder.recordEvent('insights.getStarted.tab', 'click', { metadata: { tab: index } })
    }

    return (
        <section {...otherProps}>
            <H2 id="code-insights-templates">Templates</H2>
            <Text className="text-muted">
                Some of the most popular{' '}
                <Link to="/help/code_insights/references/common_use_cases" rel="noopener noreferrer" target="_blank">
                    use cases
                </Link>
                .
            </Text>

            <Tabs size="medium" className="mt-3" onChange={handleTabChange}>
                <TabList wrapperClassName={styles.tabList}>
                    {templateSections.map(section => (
                        <Tab key={section.title}>
                            {section.title}
                            {section.experimental && <ProductStatusBadge className="ml-1" status="experimental" />}
                        </Tab>
                    ))}
                </TabList>
                <TabPanels>
                    {templateSections.map((section, index) => (
                        <TemplatesPanel
                            key={section.title}
                            sectionTitle={section.title}
                            sectionIndex={index}
                            templates={section.templates}
                            telemetryService={telemetryService}
                            telemetryRecorder={telemetryRecorder}
                        />
                    ))}
                </TabPanels>
            </Tabs>
        </section>
    )
}

interface TemplatesPanelProps extends TelemetryProps, TelemetryV2Props {
    sectionTitle: string
    sectionIndex: number
    templates: Template[]
}

const TemplatesPanel: React.FunctionComponent<React.PropsWithChildren<TemplatesPanelProps>> = props => {
    const { templates, sectionTitle, sectionIndex, telemetryService, telemetryRecorder } = props
    const [allVisible, setAllVisible] = useState(false)
    const tabMoreClickPingName = useLogEventName('InsightsGetStartedTabMoreClick')

    const maxNumberOfCards = allVisible ? templates.length : 4
    const hasMoreLessButton = templates.length > 4

    const handleShowMoreButtonClick = (): void => {
        if (!allVisible) {
            telemetryService.log(tabMoreClickPingName, { tabName: sectionTitle }, { tabName: sectionTitle })
            telemetryRecorder.recordEvent('insights.getStarted.tab.more', 'click', { metadata: { tab: sectionIndex } })
        }

        setAllVisible(!allVisible)
    }

    return (
        <TabPanel className={styles.cards}>
            {templates.slice(0, maxNumberOfCards).map(template => (
                <TemplateCard
                    key={template.title}
                    template={template}
                    telemetryService={telemetryService}
                    telemetryRecorder={telemetryRecorder}
                />
            ))}

            {hasMoreLessButton && (
                <Button
                    variant="secondary"
                    outline={true}
                    className={styles.cardsFooterButton}
                    onClick={handleShowMoreButtonClick}
                >
                    {allVisible ? 'Show less' : 'Show more'}
                </Button>
            )}
        </TabPanel>
    )
}

interface TemplateCardProps extends TelemetryProps, TelemetryV2Props {
    template: Template
}

const TemplateCard: React.FunctionComponent<React.PropsWithChildren<TemplateCardProps>> = props => {
    const { template, telemetryService, telemetryRecorder } = props
    const { mode } = useContext(CodeInsightsLandingPageContext)

    const series =
        template.type === InsightType.SearchBased
            ? template.templateValues.series ?? []
            : [{ query: template.templateValues.groupSearchQuery }]

    const handleUseTemplateLinkClick = (): void => {
        telemetryService.log('InsightGetStartedTemplateClick')
        telemetryRecorder.recordEvent('insights.getStarted.template', 'click')
    }

    return (
        <Card as={CardBody} className={styles.card}>
            <CardTitle>{template.title}</CardTitle>
            <CardText>{template.description}.</CardText>

            <div className={styles.queries}>
                {series.map(
                    line =>
                        line.query && (
                            <QueryPanel
                                key={line.query}
                                query={line.query}
                                telemetryService={telemetryService}
                                telemetryRecorder={telemetryRecorder}
                            />
                        )
                )}
            </div>

            {mode === CodeInsightsLandingPageType.InProduct && (
                <Button
                    as={Link}
                    to={getTemplateURL(template)}
                    variant="secondary"
                    outline={true}
                    className="mr-auto"
                    onClick={handleUseTemplateLinkClick}
                >
                    Use this template
                </Button>
            )}
        </Card>
    )
}

interface QueryPanelProps extends TelemetryProps, TelemetryV2Props {
    query: string
}

const copyTooltip = 'Copy query'
const copyCompletedTooltip = 'Copied!'

const QueryPanel: React.FunctionComponent<React.PropsWithChildren<QueryPanelProps>> = props => {
    const { query, telemetryService, telemetryRecorder } = props

    const templateCopyClickEvenName = useLogEventName('InsightGetStartedTemplateCopyClick')
    const [currentCopyTooltip, setCurrentCopyTooltip] = useState(copyTooltip)

    const onCopy = (event: MouseEvent<HTMLButtonElement>): void => {
        copy(query)
        setCurrentCopyTooltip(copyCompletedTooltip)
        setTimeout(() => setCurrentCopyTooltip(copyTooltip), 1000)

        event.preventDefault()
        telemetryService.log(templateCopyClickEvenName)
        telemetryRecorder.recordEvent('insights.getStarted.template', 'copy')
    }

    return (
        <CodeInsightsQueryBlock className={styles.query}>
            <SyntaxHighlightedSearchQuery query={query} />
            <Tooltip content={currentCopyTooltip} placement="top">
                <Button
                    className={styles.copyButton}
                    onClick={onCopy}
                    aria-label="Copy Docker command to clipboard"
                    variant="icon"
                >
                    <Icon aria-hidden={true} svgPath={mdiContentCopy} />
                </Button>
            </Tooltip>
        </CodeInsightsQueryBlock>
    )
}
