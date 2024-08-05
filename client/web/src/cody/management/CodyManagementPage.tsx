import React, { useCallback, useEffect } from 'react'

import { mdiCreditCardOutline, mdiHelpCircleOutline, mdiPlusThick } from '@mdi/js'
import classNames from 'classnames'
import { useNavigate } from 'react-router-dom'

import { useQuery } from '@sourcegraph/http-client'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Button, ButtonLink, H2, H3, Icon, Link, PageHeader, Text, useSearchParameters } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../auth'
import { Page } from '../../components/Page'
import { PageTitle } from '../../components/PageTitle'
import {
    CodySubscriptionPlan,
    type UserCodyPlanResult,
    type UserCodyPlanVariables,
    type UserCodyUsageResult,
    type UserCodyUsageVariables,
} from '../../graphql-operations'
import { CodyProRoutes } from '../codyProRoutes'
import { CodyAlert } from '../components/CodyAlert'
import { PageHeaderIcon } from '../components/PageHeaderIcon'
import { AcceptInviteBanner } from '../invites/AcceptInviteBanner'
import { InviteUsers } from '../invites/InviteUsers'
import { USER_CODY_PLAN, USER_CODY_USAGE } from '../subscription/queries'
import { getManageSubscriptionPageURL, isEmbeddedCodyProUIEnabled } from '../util'

import { useSubscriptionSummary } from './api/react-query/subscriptions'
import { SubscriptionStats } from './SubscriptionStats'
import { CodyEditorsAndClients } from './UseCodyInEditorSection'

import styles from './CodyManagementPage.module.scss'

interface CodyManagementPageProps extends TelemetryV2Props {
    authenticatedUser: AuthenticatedUser | null
}

export const CodyManagementPage: React.FunctionComponent<CodyManagementPageProps> = ({
    authenticatedUser,
    telemetryRecorder,
}) => {
    const navigate = useNavigate()
    const parameters = useSearchParameters()

    const utm_source = parameters.get('utm_source')
    useEffect(() => {
        telemetryRecorder.recordEvent('cody.management', 'view')
    }, [utm_source, telemetryRecorder])

    // The cody_client_user URL query param is added by the VS Code & JetBrains
    // extensions. We redirect them to a "switch account" screen if they are
    // logged into their IDE as a different user account than their browser.
    const codyClientUser = parameters.get('cody_client_user')
    const accountSwitchRequired = !!codyClientUser && authenticatedUser && authenticatedUser.username !== codyClientUser
    useEffect(() => {
        if (accountSwitchRequired) {
            navigate(`/cody/switch-account/${codyClientUser}`)
        }
    }, [accountSwitchRequired, codyClientUser, navigate])

    const welcomeToPro = parameters.get('welcome') === '1'

    const { data, error: dataError, refetch } = useQuery<UserCodyPlanResult, UserCodyPlanVariables>(USER_CODY_PLAN, {})

    const { data: usageData, error: usageDateError } = useQuery<UserCodyUsageResult, UserCodyUsageVariables>(
        USER_CODY_USAGE,
        {}
    )

    const subscriptionSummaryQueryResult = useSubscriptionSummary()

    const subscription = data?.currentUser?.codySubscription

    useEffect(() => {
        if (!!data && !data?.currentUser) {
            navigate(`/sign-in?returnTo=${CodyProRoutes.Manage}`)
        }
    }, [data, navigate])

    const onClickUpgradeToProCTA = useCallback(() => {
        telemetryRecorder.recordEvent('cody.management.upgradeToProCTA', 'click')
    }, [telemetryRecorder])

    if (accountSwitchRequired) {
        return null
    }

    if (dataError || usageDateError) {
        throw dataError || usageDateError
    }

    if (!window.context?.codyEnabledForCurrentUser || !subscription) {
        return null
    }

    const isUserOnProTier = subscription.plan === CodySubscriptionPlan.PRO

    return (
        <Page className={classNames('d-flex flex-column')}>
            <PageTitle title="Dashboard" />

            {inviteWidgets.banner}

            {welcomeToPro && (
                <CodyAlert title="Welcome to Cody Pro" variant="green" badge="CodyPro">
                    <Text>You now have Cody Pro with access to unlimited autocomplete, chats, and commands.</Text>
                </CodyAlert>
            )}
            <PageHeader
                className="my-4 d-inline-flex align-items-center"
                actions={
                    <div className="d-flex flex-column flex-gap-2">
                        {pageHeaderLink}
                        <Link
                            to="https://help.sourcegraph.com"
                            target="_blank"
                            rel="noreferrer"
                            className="text-muted text-center text-sm"
                        >
                            <Icon svgPath={mdiHelpCircleOutline} className="mr-1" aria-hidden={true} />
                            Help &amp; community
                        </Link>
                    </div>
                }
            >
                <PageHeader.Heading as="h1" className="text-3xl font-medium">
                    <PageHeaderIcon name="dashboard" className="mr-3" />
                    <Text as="span">Cody dashboard</Text>
                </PageHeader.Heading>
            </PageHeader>

                {isAdmin && !!subscriptionSummaryQueryResult.data && (
                    <InviteUsers
                        telemetryRecorder={telemetryRecorder}
                        subscriptionSummary={subscriptionSummaryQueryResult.data}
                    />
                )}

                {!isUserOnProTier && <UpgradeToProBanner onClick={onClickUpgradeToProCTA} />}

                <div className={classNames('p-4 border bg-1 mt-3', styles.container)}>
                    <div className="d-flex justify-content-between align-items-center border-bottom pb-3">
                        <div>
                            <H2>My subscription</H2>
                            <Text className="text-muted mb-0">
                                {isUserOnProTier ? (
                                    'You are on the Pro tier.'
                                ) : (
                                    <span>
                                        You are on the Free tier.{' '}
                                        <Link to={CodyProRoutes.Subscription}>Upgrade to the Pro tier.</Link>
                                    </span>
                                )}
                            </Text>
                        </div>
                        {isUserOnProTier && (
                            <div>
                                <ButtonLink
                                    variant="primary"
                                    size="sm"
                                    to={getManageSubscriptionPageURL()}
                                    onClick={() => {
                                        telemetryRecorder.recordEvent('cody.manageSubscription', 'click')
                                    }}
                                >
                                    <Icon svgPath={mdiCreditCardOutline} className="mr-1" aria-hidden={true} />
                                    Manage subscription
                                </ButtonLink>
                            </div>
                        )}
                    </div>
                }
            >
                <PageHeader.Heading as="h1" className="text-3xl font-medium">
                    <PageHeaderIcon name="dashboard" className="mr-3" />
                    <Text as="span">Cody dashboard</Text>
                </PageHeader.Heading>
            </PageHeader>

            {inviteWidgets.form}

            <div className={classNames('border bg-1 mb-2', styles.container)}>
                <SubscriptionStats
                    subscription={subscription}
                    usageData={usageData}
                    onClickUpgradeToProCTA={onClickUpgradeToProCTA}
                />
            </div>

            <H3 className="mt-3 text-muted">Use Cody...</H3>
            <div className={classNames('border bg-1 mb-2', styles.container)}>
                <CodyEditorsAndClients telemetryRecorder={telemetryRecorder} />
            </div>
        </div>b
    </CodyAlert>
)
}
