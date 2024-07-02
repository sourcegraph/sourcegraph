import { type FC, useEffect } from 'react'

import { mdiPlus } from '@mdi/js'
import classNames from 'classnames'
import { noop } from 'rxjs'

import { gql, useQuery } from '@sourcegraph/http-client'
import { useSettingsCascade } from '@sourcegraph/shared/src/settings/settings'
import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import {
    Button,
    Card,
    Link,
    Icon,
    Input,
    Text,
    Label,
    useForm,
    useField,
    useDebounce,
    getDefaultInputProps,
} from '@sourcegraph/wildcard'

import type { GetExampleRepositoryResult, GetExampleRepositoryVariables } from '../../../../../../../graphql-operations'
import { SearchPatternType } from '../../../../../../../graphql-operations'
import { defaultPatternTypeFromSettings } from '../../../../../../../util/settings'
import { InsightQueryInput, RepositoriesField, insightRepositoriesValidator } from '../../../../../components'
import { getQueryPatternTypeFilter } from '../../../../insights/creation/search-insight'
import { CodeInsightsDescription } from '../code-insights-description/CodeInsightsDescription'

import { DynamicInsightPreview } from './DynamicInsightPreview'

import styles from './DynamicCodeInsightExample.module.scss'

interface CodeInsightExampleFormValues {
    repositories: string[]
    query: string
}

const INITIAL_INSIGHT_VALUES: CodeInsightExampleFormValues = {
    repositories: [],
    query: 'TODO',
}

interface DynamicCodeInsightExampleProps
    extends TelemetryProps,
        TelemetryV2Props,
        React.HTMLAttributes<HTMLDivElement> {}

export const DynamicCodeInsightExample: FC<DynamicCodeInsightExampleProps> = props => {
    const { telemetryService, telemetryRecorder, ...otherProps } = props

    const { repositoryUrl, loading: repositoryValueLoading } = useExampleRepositoryUrl()

    const form = useForm<CodeInsightExampleFormValues>({
        initialValues: INITIAL_INSIGHT_VALUES,
        touched: true,
        onSubmit: noop,
    })

    const repositories = useField({
        name: 'repositories',
        formApi: form.formAPI,
        disabled: repositoryValueLoading,
        validators: {
            sync: insightRepositoriesValidator,
        },
    })

    const query = useField({
        name: 'query',
        formApi: form.formAPI,
    })

    const debouncedQuery = useDebounce(query.input.value, 1000)
    const debouncedRepositories = useDebounce(repositories.input.value, 1000)

    const { onChange: setRepositoryValue } = repositories.input

    useEffect(() => {
        // This is to prevent resetting the name in an endless loop
        if (repositoryUrl) {
            setRepositoryValue([repositoryUrl])
        }
    }, [setRepositoryValue, repositoryUrl])

    useEffect(() => {
        if (debouncedQuery !== INITIAL_INSIGHT_VALUES.query) {
            telemetryService.log('InsightsGetStartedPageQueryModification')
            telemetryRecorder.recordEvent('insights.getStarted.query', 'modify')
        }
    }, [debouncedQuery, telemetryService, telemetryRecorder])

    useEffect(() => {
        if (debouncedRepositories !== INITIAL_INSIGHT_VALUES.repositories) {
            telemetryService.log('InsightsGetStartedPageRepositoriesModification')
            telemetryRecorder.recordEvent('insights.getStarted.repositories', 'modify')
        }
    }, [debouncedRepositories, telemetryService, telemetryRecorder])

    const handleGetStartedClick = (): void => {
        telemetryService.log('InsightsGetStartedPrimaryCTAClick')
        telemetryRecorder.recordEvent('insights.getStarted.primaryCTA', 'click')
    }

    const hasValidLivePreview =
        !repositoryValueLoading && repositories.meta.validState === 'VALID' && query.meta.validState === 'VALID'

    const { status: repositoryStatus, ...repositoryProps } = getDefaultInputProps(repositories)

    const defaultPatternType: SearchPatternType = defaultPatternTypeFromSettings(useSettingsCascade())

    return (
        <Card {...otherProps} className={classNames(styles.wrapper, otherProps.className)}>
            {/* eslint-disable-next-line react/forbid-elements */}
            <form ref={form.ref} noValidate={true} onSubmit={form.handleSubmit} className={styles.chartSection}>
                <DynamicInsightPreview
                    telemetryService={telemetryService}
                    telemetryRecorder={telemetryRecorder}
                    disabled={!hasValidLivePreview}
                    repositories={repositories.input.value}
                    query={query.input.value}
                    className={styles.chart}
                />

                <Input
                    label="Data series search query"
                    required={true}
                    as={InsightQueryInput}
                    repoQuery={null}
                    repositories={repositories.input.value}
                    patternType={getQueryPatternTypeFilter(query.input.value, defaultPatternType)}
                    placeholder="Example: patternType:regexp const\s\w+:\s(React\.)?FunctionComponent"
                    {...getDefaultInputProps(query)}
                    className="mt-3 mb-0"
                />

                <Label htmlFor="repositories-id" className="mt-3">
                    Repositories
                </Label>
                <RepositoriesField
                    id="repositories-id"
                    description="Find and choose at least 1 repository to run insight"
                    placeholder="Search repositories..."
                    status={repositoryValueLoading ? 'loading' : repositoryStatus}
                    {...repositoryProps}
                />
            </form>

            <div>
                <CodeInsightsDescription />
                <footer className={styles.footer}>
                    <Button variant="primary" as={Link} to="/insights/create" onClick={handleGetStartedClick}>
                        <Icon aria-hidden={true} svgPath={mdiPlus} /> Create your first insight
                    </Button>
                </footer>
                <CalloutArrow className={styles.calloutBlockHorizontal} />
            </div>

            <CalloutArrow className={styles.calloutBlockVertical} />
        </Card>
    )
}

const CalloutArrow: FC<{ className?: string }> = props => (
    <Text className={classNames(styles.calloutBlock, props.className)}>
        <svg
            width="59"
            height="41"
            viewBox="0 0 59 41"
            fill="none"
            xmlns="http://www.w3.org/2000/svg"
            className={styles.calloutArrow}
        >
            <path
                d="M3.23717 0.288488C2.84421 0.157502 2.41947 0.369872 2.28849 0.762829L0.15395 7.16644C0.0229642 7.5594 0.235334 7.98414 0.628292 8.11512C1.02125 8.24611 1.44599 8.03374 1.57698 7.64078L3.47434 1.94868L9.16644 3.84605C9.5594 3.97704 9.98414 3.76467 10.1151 3.37171C10.2461 2.97875 10.0337 2.55401 9.64078 2.42302L3.23717 0.288488ZM57.9254 40.7463C58.3375 40.7875 58.7051 40.4868 58.7463 40.0746C58.7875 39.6625 58.4868 39.2949 58.0746 39.2537L57.9254 40.7463ZM2.32918 1.33541C14.452 25.5811 37.6871 38.7224 57.9254 40.7463L58.0746 39.2537C38.3129 37.2776 15.548 24.4189 3.67082 0.66459L2.32918 1.33541Z"
                fill="#A6B6D9"
            />
        </svg>
        <span className="text-muted">This insight is interactive! Type any search query or change the repo.</span>
    </Text>
)

export const GET_EXAMPLE_REPOSITORY = gql`
    query GetExampleRepository {
        todoRepo: search(patternType: literal, version: V2, query: "select:repo TODO count:1") {
            results {
                repositories {
                    name
                }
            }
        }
        firstRepo: search(patternType: literal, version: V2, query: "select:repo count:1") {
            results {
                repositories {
                    name
                }
            }
        }
    }
`

interface UseExampleRepositoryUrlReturn {
    repositoryUrl?: string
    loading: boolean
}

function useExampleRepositoryUrl(): UseExampleRepositoryUrlReturn {
    const { data, loading, error } = useQuery<GetExampleRepositoryResult, GetExampleRepositoryVariables>(
        GET_EXAMPLE_REPOSITORY,
        { fetchPolicy: 'cache-and-network' }
    )

    if (loading || !data) {
        return { loading: true }
    }

    if (error) {
        return { repositoryUrl: 'github.com/sourcegraph/sourcegraph', loading: false }
    }

    const firstRepository = data.firstRepo?.results?.repositories[0]?.name
    const todoRepository = data.todoRepo?.results?.repositories[0]?.name

    return {
        loading: false,
        repositoryUrl: todoRepository ?? firstRepository ?? 'github.com/sourcegraph/sourcegraph',
    }
}
