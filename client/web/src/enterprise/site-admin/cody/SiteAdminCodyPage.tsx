import { type FC, useCallback, useEffect, useState, useMemo } from 'react'

import { mdiMapSearch } from '@mdi/js'
import { capitalize } from 'lodash'
import { useLocation } from 'react-router-dom'

import { RepoEmbeddingJobState } from '@sourcegraph/shared/src/graphql-operations'
import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import {
    Button,
    Container,
    getDefaultInputProps,
    Label,
    PageHeader,
    useField,
    useForm,
    H3,
    type Validator,
    ErrorAlert,
    Form,
    useDebounce,
    Icon,
} from '@sourcegraph/wildcard'

import { CodyColorIcon } from '../../../cody/chat/CodyPageIcon'
import type { FilteredConnectionFilter, FilteredConnectionFilterValue } from '../../../components/FilteredConnection'
import {
    ConnectionContainer,
    ConnectionError,
    ConnectionForm,
    ConnectionList,
    ConnectionLoading,
    ConnectionSummary,
    ShowMoreButton,
    SummaryContainer,
} from '../../../components/FilteredConnection/ui'
import { getFilterFromURL } from '../../../components/FilteredConnection/utils'
import { PageTitle } from '../../../components/PageTitle'
import { RepositoriesField } from '../../insights/components'

import { useCancelRepoEmbeddingJob, useRepoEmbeddingJobsConnection, useScheduleRepoEmbeddingJobs } from './backend'
import { RepoEmbeddingJobNode } from './RepoEmbeddingJobNode'

import styles from './SiteAdminCodyPage.module.scss'

export interface SiteAdminCodyPageProps extends TelemetryProps, TelemetryV2Props {}

interface RepoEmbeddingJobsFormValues {
    repositories: string[]
}

const INITIAL_REPOSITORIES = { repositories: [] }

const repositoriesValidator: Validator<string[]> = value => {
    if (value !== undefined && value.length === 0) {
        return 'Repositories is a required field.'
    }
    return
}

// Helper function to convert an enum to a list of FilteredConnectionFilterValue
const enumToFilterValues = <T extends string>(enumeration: { [key in T]: T }): FilteredConnectionFilterValue[] => {
    const values: FilteredConnectionFilterValue[] = []
    for (const key of Object.keys(enumeration)) {
        values.push({
            value: key.toLowerCase(),
            label: capitalize(key),
            args: {},
            tooltip: `Show ${key.toLowerCase()} jobs`,
        })
    }
    return values
}

export const SiteAdminCodyPage: FC<SiteAdminCodyPageProps> = ({ telemetryService, telemetryRecorder }) => {
    useEffect(() => {
        telemetryService.logPageView('SiteAdminCodyPage')
        telemetryRecorder.recordEvent('admin.cody.embeddingsJobs', 'view')
    }, [telemetryService, telemetryRecorder])

    const location = useLocation()
    const searchParams = useMemo(() => new URLSearchParams(location.search), [location.search])
    const queryParam = searchParams.get('query')

    const [searchValue, setSearchValue] = useState(queryParam || '')
    const query = useDebounce(searchValue, 200)

    const defaultStateFilterValue = 'all'
    const filters: FilteredConnectionFilter[] = [
        {
            id: 'state',
            label: 'State',
            type: 'select',
            values: [
                {
                    label: 'All',
                    value: defaultStateFilterValue,
                    tooltip: 'Show all jobs',
                    args: {},
                },
                ...enumToFilterValues(RepoEmbeddingJobState),
            ],
        },
    ]

    const [filterValues, setFilterValues] = useState<Map<string, FilteredConnectionFilterValue>>(() =>
        getFilterFromURL(searchParams, filters)
    )

    const getStateFilterValue = (filterValues: Map<string, FilteredConnectionFilterValue>): string | null => {
        const val = filterValues.get('state')?.value || defaultStateFilterValue
        return val === defaultStateFilterValue ? null : val
    }

    const { loading, hasNextPage, fetchMore, refetchAll, refetchFirst, connection, error } =
        useRepoEmbeddingJobsConnection(query, getStateFilterValue(filterValues))

    const [scheduleRepoEmbeddingJobs, { loading: repoEmbeddingJobsLoading, error: repoEmbeddingJobsError }] =
        useScheduleRepoEmbeddingJobs()

    const onSubmit = useCallback(
        async (repoNames: string[]) => {
            telemetryRecorder.recordEvent('admin.cody.embeddingsJobs', 'scheduleJob')
            await scheduleRepoEmbeddingJobs({ variables: { repoNames } })
            refetchFirst()
        },
        [refetchFirst, scheduleRepoEmbeddingJobs, telemetryRecorder]
    )

    const form = useForm<RepoEmbeddingJobsFormValues>({
        initialValues: INITIAL_REPOSITORIES,
        touched: false,
        onSubmit: values => onSubmit(values.repositories),
    })

    const repositories = useField({
        name: 'repositories',
        formApi: form.formAPI,
        validators: { sync: repositoriesValidator },
    })

    const updateQueryParams = (key: string, value: string): void => {
        if (value === '') {
            searchParams.delete(key)
        } else {
            searchParams.set(key, value)
        }

        const queryString = searchParams.toString()
        const newUrl = queryString === '' ? window.location.pathname : `${window.location.pathname}?${queryString}`
        window.history.replaceState(null, '', newUrl)
    }

    const [cancelRepoEmbeddingJob, { error: cancelRepoEmbeddingJobError }] = useCancelRepoEmbeddingJob()

    const onCancel = useCallback(
        async (id: string) => {
            await cancelRepoEmbeddingJob({ variables: { id } })
            refetchAll()
        },
        [cancelRepoEmbeddingJob, refetchAll]
    )

    return (
        <>
            <PageTitle title="Embeddings jobs" />
            <PageHeader
                path={[{ icon: CodyColorIcon, text: 'Cody' }, { text: 'Embeddings jobs' }]}
                className="mb-3"
                headingElement="h2"
            />
            <Container className="mb-3">
                <H3>Schedule repositories for embedding</H3>
                <Form ref={form.ref} noValidate={true} onSubmit={form.handleSubmit}>
                    <Label htmlFor="repositories-id" className="mt-1">
                        Repositories
                    </Label>
                    <div className="d-flex">
                        <RepositoriesField
                            id="repositories-id"
                            description="Schedule repositories for embedding at latest revision on the default branch."
                            placeholder="Add repositories to schedule..."
                            className="flex-1 mr-2"
                            {...getDefaultInputProps(repositories)}
                        />
                        <div>
                            <Button
                                type="submit"
                                variant="secondary"
                                className={styles.scheduleButton}
                                disabled={repoEmbeddingJobsLoading}
                            >
                                {repoEmbeddingJobsLoading ? 'Scheduling...' : 'Schedule Embedding'}
                            </Button>
                        </div>
                    </div>
                </Form>
                {(repoEmbeddingJobsError || cancelRepoEmbeddingJobError) && (
                    <div className="mt-1">
                        <ErrorAlert
                            prefix="Error scheduling embeddings jobs"
                            error={repoEmbeddingJobsError || cancelRepoEmbeddingJobError}
                        />
                    </div>
                )}
            </Container>
            <Container>
                <H3 className="mt-3">Repository embeddings jobs</H3>
                <ConnectionContainer>
                    <ConnectionForm
                        formClassName="mb-2"
                        inputClassName="flex-1 ml-2"
                        inputValue={searchValue}
                        onInputChange={event => {
                            setSearchValue(event.target.value)
                            updateQueryParams('query', event.target.value)
                        }}
                        inputPlaceholder="Filter embeddings jobs..."
                        filters={filters}
                        filterValues={filterValues}
                        onFilterSelect={(filter: FilteredConnectionFilter, value: FilteredConnectionFilterValue) => {
                            setFilterValues(values => {
                                const newValues = new Map(values)
                                newValues.set(filter.id, value)
                                return newValues
                            })
                            updateQueryParams(filter.id, value.value)
                        }}
                    />
                    {error && <ConnectionError errors={[error.message]} />}
                    {loading && !connection && <ConnectionLoading />}
                    <ConnectionList as="ul" className="list-group" aria-label="Repository embeddings jobs">
                        {connection?.nodes?.map(node => (
                            <RepoEmbeddingJobNode key={node.id} {...node} onCancel={onCancel} />
                        ))}
                    </ConnectionList>
                    {connection && (
                        <SummaryContainer className="mt-2" centered={true}>
                            <ConnectionSummary
                                noSummaryIfAllNodesVisible={false}
                                first={connection.totalCount ?? 0}
                                centered={true}
                                connection={connection}
                                connectionQuery={query}
                                noun="repository embeddings job"
                                pluralNoun="repository embeddings jobs"
                                hasNextPage={hasNextPage}
                                emptyElement={<EmptyList />}
                            />
                            {hasNextPage && <ShowMoreButton centered={true} onClick={fetchMore} />}
                        </SummaryContainer>
                    )}
                </ConnectionContainer>
            </Container>
        </>
    )
}

const EmptyList: FC<{}> = () => (
    <div className="text-muted text-center mb-3 w-100">
        <Icon className="mb-2" svgPath={mdiMapSearch} inline={false} aria-hidden={true} />
        <div className="pt-2">No repository embeddings jobs found.</div>
    </div>
)
