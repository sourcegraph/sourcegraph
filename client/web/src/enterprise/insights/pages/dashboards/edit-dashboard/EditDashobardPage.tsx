import { type FC, useContext, useMemo } from 'react'

import classNames from 'classnames'
import MapSearchIcon from 'mdi-react/MapSearchIcon'
import { useParams, useNavigate } from 'react-router-dom'
import { lastValueFrom } from 'rxjs'

import {
    Button,
    Container,
    LoadingSpinner,
    PageHeader,
    useObservable,
    Link,
    type SubmissionErrors,
} from '@sourcegraph/wildcard'

import { HeroPage } from '../../../../../components/HeroPage'
import { LoaderButton } from '../../../../../components/LoaderButton'
import { PageTitle } from '../../../../../components/PageTitle'
import { CodeInsightsIcon, CodeInsightsPage } from '../../../components'
import {
    CodeInsightsBackendContext,
    type CustomInsightDashboard,
    type InsightsDashboardOwner,
    InsightsDashboardOwnerType,
    isGlobalOwner,
    isPersonalOwner,
    isVirtualDashboard,
    useInsightDashboard,
} from '../../../core'
import {
    type DashboardCreationFields,
    InsightsDashboardCreationContent,
} from '../creation/components/InsightsDashboardCreationContent'

import styles from './EditDashboardPage.module.scss'

/**
 * Displays the edit (configure) dashboard page.
 */
export const EditDashboardPage: FC = props => {
    const navigate = useNavigate()
    const { dashboardId } = useParams()

    const { getDashboardOwners, updateDashboard } = useContext(CodeInsightsBackendContext)

    // Load edit dashboard information
    const owners = useObservable(useMemo(() => getDashboardOwners(), [getDashboardOwners]))

    const { dashboard, loading } = useInsightDashboard({ id: dashboardId })

    // Loading state
    if (owners === undefined || dashboard === undefined || loading) {
        return <LoadingSpinner />
    }

    // In case if we got null that means we couldn't find this dashboard
    if (dashboard === null || isVirtualDashboard(dashboard)) {
        return <HeroPage icon={MapSearchIcon} title="Oops, we couldn't find the dashboard" />
    }

    const handleSubmit = async (dashboardValues: DashboardCreationFields): Promise<SubmissionErrors> => {
        if (!dashboard) {
            return
        }

        const { name, owner } = dashboardValues

        if (!owner) {
            throw new Error('You have to specify a dashboard visibility')
        }

        const updatedDashboard = await lastValueFrom(
            updateDashboard({
                id: dashboard.id,
                nextDashboardInput: {
                    name,
                    owners: [owner],
                },
            })
        )

        navigate(`/insights/dashboards/${updatedDashboard.id}`)
    }

    const handleCancel = (): void => navigate(-1)

    return (
        <CodeInsightsPage className={classNames('col-8', styles.page)}>
            <PageTitle title={`Configure ${dashboard.title} - Code Insights`} />

            <PageHeader path={[{ icon: CodeInsightsIcon }, { text: 'Configure dashboard' }]} />

            <span className="text-muted d-block mt-2">
                Dashboards group your insights and let you share them with others.{' '}
                <Link to="/help/code_insights/explanations/viewing_code_insights" target="_blank" rel="noopener">
                    Learn more.
                </Link>
            </span>

            <Container className="mt-4">
                <InsightsDashboardCreationContent
                    initialValues={getDashboardInitialValues(dashboard, owners)}
                    owners={owners}
                    onSubmit={handleSubmit}
                >
                    {formAPI => (
                        <>
                            <Button
                                type="button"
                                variant="secondary"
                                outline={true}
                                className="mb-2"
                                onClick={handleCancel}
                            >
                                Cancel
                            </Button>

                            <LoaderButton
                                alwaysShowLabel={true}
                                data-testid="insight-save-button"
                                loading={formAPI.submitting}
                                label={formAPI.submitting ? 'Saving' : 'Save changes'}
                                type="submit"
                                disabled={formAPI.submitting}
                                className="ml-2 mb-2"
                                variant="primary"
                            />
                        </>
                    )}
                </InsightsDashboardCreationContent>
            </Container>
        </CodeInsightsPage>
    )
}

function getDashboardInitialValues(
    dashboard: CustomInsightDashboard,
    availableOwners: InsightsDashboardOwner[]
): DashboardCreationFields | undefined {
    const { title } = dashboard

    const isGlobal = dashboard.owners.some(isGlobalOwner)
    const availableGlobalOwner = availableOwners.find(
        availableOwner => availableOwner.type === InsightsDashboardOwnerType.Global
    )

    if (isGlobal && availableGlobalOwner) {
        // Pick any global owner from the list
        return {
            name: title,
            owner: availableGlobalOwner,
        }
    }

    const owner = dashboard.owners.find(owner => availableOwners.some(availableOwner => availableOwner.id === owner.id))

    return {
        name: title,
        owner: owner ?? availableOwners.find(isPersonalOwner)!,
    }
}
