import React, { useCallback } from 'react'

import { mdiOpenInNew } from '@mdi/js'
import classNames from 'classnames'

import { SearchPatternType } from '@sourcegraph/shared/src/graphql-operations'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { buildSearchURLQuery } from '@sourcegraph/shared/src/util/url'
import {
    ButtonLink,
    H2,
    Icon,
    Link,
    ProductStatusBadge,
    type ProductStatusType,
    Tab,
    TabList,
    TabPanel,
    TabPanels,
    Tabs,
} from '@sourcegraph/wildcard'

import { exampleQueryColumns } from './QueryExamples.constants'
import { SyntaxHighlightedSearchQuery } from './SyntaxHighlightedSearchQuery'
import { type QueryExamplesSection, useQueryExamples } from './useQueryExamples'

import styles from './QueryExamples.module.scss'

export interface QueryExamplesProps extends TelemetryProps, TelemetryV2Props {
    selectedSearchContextSpec?: string
    isSourcegraphDotCom?: boolean
    showQueryExamplesForKeywordSearch: boolean
}

export const QueryExamples: React.FunctionComponent<QueryExamplesProps> = ({
    selectedSearchContextSpec,
    telemetryService,
    telemetryRecorder,
    isSourcegraphDotCom = false,
    showQueryExamplesForKeywordSearch,
}) => {
    const exampleSyntaxColumns = useQueryExamples(
        selectedSearchContextSpec ?? 'global',
        isSourcegraphDotCom,
        showQueryExamplesForKeywordSearch
    )

    const patternType = showQueryExamplesForKeywordSearch ? SearchPatternType.keyword : SearchPatternType.standard

    const onQueryExampleClick = useCallback(
        (query: string) => {
            telemetryService.log('QueryExampleClicked', { queryExample: query }, { queryExample: query })
            telemetryRecorder.recordEvent('search.queryExample', 'click')
        },
        [telemetryService, telemetryRecorder]
    )

    return isSourcegraphDotCom ? (
        <Tabs size="medium">
            <TabList wrapperClassName={classNames('mb-4', styles.tabHeader)}>
                <Tab>How to search</Tab>
                <Tab>Popular queries</Tab>
            </TabList>
            <TabPanels>
                <TabPanel className={styles.tabPanel}>
                    <QueryExamplesLayout
                        queryColumns={exampleSyntaxColumns}
                        onQueryExampleClick={onQueryExampleClick}
                        patternType={patternType}
                    />
                </TabPanel>
                <TabPanel className={styles.tabPanel}>
                    <QueryExamplesLayout
                        queryColumns={exampleQueryColumns}
                        onQueryExampleClick={onQueryExampleClick}
                        patternType={patternType}
                    />
                </TabPanel>
            </TabPanels>
        </Tabs>
    ) : (
        <QueryExamplesLayout
            queryColumns={exampleSyntaxColumns}
            onQueryExampleClick={onQueryExampleClick}
            patternType={patternType}
        />
    )
}

interface QueryExamplesLayout {
    queryColumns: QueryExamplesSection[][]
    onQueryExampleClick: (query: string) => void
    patternType: SearchPatternType
}

const QueryExamplesLayout: React.FunctionComponent<QueryExamplesLayout> = ({
    queryColumns,
    onQueryExampleClick,
    patternType,
}) => (
    <div className={styles.queryExamplesSectionsColumns}>
        {queryColumns.map((column, index) => (
            <div key={`column-${queryColumns[index][0].title}`}>
                {column.map(({ title, queryExamples }) => (
                    <ExamplesSection
                        key={title}
                        title={title}
                        queryExamples={queryExamples}
                        onQueryExampleClick={onQueryExampleClick}
                        patternType={patternType}
                    />
                ))}
                {/* Add docs link to last column */}
                {queryColumns.length === index + 1 && (
                    <small className="d-block">
                        <Link target="blank" to="/help/code_search/reference/queries">
                            Complete query reference{' '}
                            <Icon role="img" aria-label="Open in a new tab" svgPath={mdiOpenInNew} />
                        </Link>
                    </small>
                )}
            </div>
        ))}
    </div>
)

interface ExamplesSection extends QueryExamplesSection {
    onQueryExampleClick: (query: string) => void
    patternType: SearchPatternType
}

const ExamplesSection: React.FunctionComponent<ExamplesSection> = ({
    title,
    queryExamples,
    onQueryExampleClick,
    patternType,
}) => (
    <div className={styles.queryExamplesSection}>
        <H2 className={styles.queryExamplesSectionTitle}>{title}</H2>
        <ul className={classNames('list-unstyled', styles.queryExamplesItems)}>
            {queryExamples
                .filter(({ query }) => query.length > 0)
                .map(({ query, helperText, productStatus }) => (
                    <QueryExampleChip
                        key={query}
                        query={query}
                        helperText={helperText}
                        onClick={onQueryExampleClick}
                        productStatus={productStatus}
                        patternType={patternType}
                    />
                ))}
        </ul>
    </div>
)

interface QueryExample {
    query: string
    patternType: SearchPatternType
    helperText?: string
    productStatus?: ProductStatusType
}

interface QueryExampleChipProps extends QueryExample {
    className?: string
    onClick: (query: string) => void | undefined
}

const QueryExampleChip: React.FunctionComponent<QueryExampleChipProps> = ({
    query,
    helperText,
    className,
    onClick,
    productStatus,
    patternType,
}) => (
    <li className={classNames('d-flex align-items-center', className)}>
        <ButtonLink
            className={styles.queryExampleChip}
            to={`/search?${buildSearchURLQuery(query, patternType, false)}`}
            onClick={() => onClick(query)}
        >
            <SyntaxHighlightedSearchQuery query={query} searchPatternType={SearchPatternType.standard} />
        </ButtonLink>
        {helperText && (
            <span className="text-muted ml-2">
                <small>{helperText}</small>
            </span>
        )}
        {productStatus && <ProductStatusBadge status={productStatus} className="ml-2" />}
    </li>
)
