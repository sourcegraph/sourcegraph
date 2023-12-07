import React, { useEffect } from 'react'
import type { ReactElement } from 'react'

import { mdiHelpCircleOutline, mdiTrendingUp, mdiDownload, mdiInformation } from '@mdi/js'
import classNames from 'classnames'

import { useQuery, useMutation } from '@sourcegraph/http-client'
import {
    Icon,
    PageHeader,
    Link,
    H4,
    H2,
    Text,
    ButtonLink,
    Modal,
    useSearchParameters,
    LoadingSpinner,
} from '@sourcegraph/wildcard'

import { Page } from '../../components/Page'
import { PageTitle } from '../../components/PageTitle'
import { useFeatureFlag } from '../../featureFlags/useFeatureFlag'
import type {
    UserCodyPlanResult,
    UserCodyPlanVariables,
    UserCodyUsageResult,
    UserCodyUsageVariables,
    ChangeCodyPlanResult,
    ChangeCodyPlanVariables,
} from '../../graphql-operations'
import { eventLogger } from '../../tracking/eventLogger'
import { EventName } from '../../util/constants'
import { CodyColorIcon } from '../chat/CodyPageIcon'
import { isCodyEnabled } from '../isCodyEnabled'
import { CodyOnboarding, editorGroups, type IEditor } from '../onboarding/CodyOnboarding'
import { ProTierIcon } from '../subscription/CodySubscriptionPage'
import { USER_CODY_PLAN, USER_CODY_USAGE, CHANGE_CODY_PLAN } from '../subscription/queries'

import styles from './CodyManagementPage.module.scss'

interface CodyManagementPageProps {
    isSourcegraphDotCom: boolean
}

export const CodyManagementPage: React.FunctionComponent<CodyManagementPageProps> = ({ isSourcegraphDotCom }) => {
    const parameters = useSearchParameters()

    const utm_source = parameters.get('utm_source')

    useEffect(() => {
        eventLogger.log(EventName.CODY_MANAGEMENT_PAGE_VIEWED, { utm_source })
    }, [utm_source])

    const { data } = useQuery<UserCodyPlanResult, UserCodyPlanVariables>(USER_CODY_PLAN, {})

    const { data: usageData } = useQuery<UserCodyUsageResult, UserCodyUsageVariables>(USER_CODY_USAGE, {})

    const [changeCodyPlan] = useMutation<ChangeCodyPlanResult, ChangeCodyPlanVariables>(CHANGE_CODY_PLAN)

    const [isEnabled] = useFeatureFlag('cody-pro', false)

    const [selectedEditor, setSelectedEditor] = React.useState<IEditor | null>(null)
    const [selectedEditorStep, setSelectedEditorStep] = React.useState<number | null>(null)

    const enrollPro = parameters.get('pro') === 'true'

    useEffect(() => {
        if (enrollPro && data?.currentUser && !data?.currentUser?.codyProEnabled) {
            changeCodyPlan({ variables: { pro: true, id: data?.currentUser?.id } })
        }
    }, [data?.currentUser, changeCodyPlan, enrollPro])

    if (!isCodyEnabled() || !isSourcegraphDotCom || !isEnabled || !data?.currentUser) {
        return null
    }

    const { codyProEnabled } = data.currentUser

    return (
        <>
            <Page className={classNames('d-flex flex-column')}>
                <PageTitle title="Dashboard" />
                <PageHeader className="mb-4">
                    <PageHeader.Heading as="h2" styleAs="h1">
                        <div className="d-inline-flex align-items-center">
                            <CodyColorIcon width={40} height={40} className="mr-2" /> Dashboard
                        </div>
                    </PageHeader.Heading>
                </PageHeader>

                <div className="p-3 border bg-1 mt-4">
                    <div className="d-flex justify-content-between align-items-center border-bottom pb-3">
                        <div>
                            <H2>My Subscription</H2>
                            <Text className="text-muted mb-0">
                                You are on a {codyProEnabled ? 'pro' : 'community'} tier.
                            </Text>
                        </div>
                        {codyProEnabled ? (
                            <div>
                                <ButtonLink to="/cody/subscription" variant="secondary" outline={true} size="sm">
                                    Manage Subscription
                                </ButtonLink>
                            </div>
                        ) : (
                            <div>
                                <ButtonLink to="/cody/subscription" variant="primary" size="sm">
                                    <Icon svgPath={mdiTrendingUp} className="mr-1" aria-hidden={true} />
                                    Upgrade for free
                                </ButtonLink>
                            </div>
                        )}
                    </div>
                    <div className={classNames('d-flex align-items-center mt-3', styles.responsiveContainer)}>
                        <div className="d-flex flex-column align-items-center flex-grow-1 p-3">
                            {codyProEnabled ? (
                                <ProTierIcon />
                            ) : (
                                <Text className={classNames(styles.planName, 'mb-0')}>Community</Text>
                            )}
                            <Text className="text-muted mb-0" size="small">
                                tier
                            </Text>
                        </div>
                        <div className="d-flex flex-column align-items-center flex-grow-1 p-3 border-left border-right">
                            <AutocompletesIcon />
                            <div className="mb-3 mt-2">
                                {codyProEnabled ? (
                                    <Text weight="bold" className={classNames('d-inline mb-0', styles.counter)}>
                                        Unlimited
                                    </Text>
                                ) : usageData?.currentUser ? (
                                    <>
                                        <Text weight="bold" className={classNames('d-inline mb-0', styles.counter)}>
                                            {Math.min(
                                                usageData?.currentUser?.codyCurrentPeriodCodeUsage || 0,
                                                usageData?.currentUser?.codyCurrentPeriodCodeLimit || 0
                                            )}{' '}
                                            /
                                        </Text>{' '}
                                        <Text className="text-muted d-inline b-0" size="small">
                                            {usageData?.currentUser?.codyCurrentPeriodCodeLimit || 0}
                                        </Text>
                                    </>
                                ) : (
                                    <LoadingSpinner />
                                )}
                            </div>
                            <H4 className="mb-0">Autocompletes</H4>
                            {!codyProEnabled && (
                                <Text className="text-muted mb-0" size="small">
                                    this month
                                </Text>
                            )}
                        </div>
                        <div className="d-flex flex-column align-items-center flex-grow-1 p-3">
                            <ChatMessagesIcon />
                            <div className="mb-3 mt-2">
                                {codyProEnabled ? (
                                    <Text weight="bold" className={classNames('d-inline mb-0', styles.counter)}>
                                        Unlimited
                                    </Text>
                                ) : usageData?.currentUser ? (
                                    <>
                                        <Text weight="bold" className={classNames('d-inline mb-0', styles.counter)}>
                                            {Math.min(
                                                usageData?.currentUser?.codyCurrentPeriodChatUsage || 0,
                                                usageData?.currentUser?.codyCurrentPeriodChatLimit || 0
                                            )}{' '}
                                            /
                                        </Text>{' '}
                                        <Text className="text-muted d-inline b-0" size="small">
                                            {usageData?.currentUser?.codyCurrentPeriodChatLimit || 0}
                                        </Text>
                                    </>
                                ) : (
                                    <LoadingSpinner />
                                )}
                            </div>
                            <H4 className="mb-0">Chat Messages</H4>
                            {!codyProEnabled && (
                                <Text className="text-muted mb-0" size="small">
                                    this month
                                </Text>
                            )}
                        </div>
                        {codyProEnabled && (
                            <div className="d-flex flex-column align-items-center flex-grow-1 p-3 border-left">
                                <TrialPeriodIcon />
                                <div className="mb-3 mt-2">
                                    <Text weight="bold" className={classNames('d-inline mb-0', styles.counter)}>
                                        Free trial
                                    </Text>
                                </div>
                                <Text className="text-muted mb-0" size="small">
                                    Until 14th of February 2023
                                </Text>
                            </div>
                        )}
                    </div>
                </div>

                <div className="p-3 border bg-1 mt-4">
                    <div className="d-flex justify-content-between align-items-center border-bottom pb-3">
                        <div>
                            <H2>Extensions & Plugins</H2>
                            <Text className="text-muted mb-0">Cody integrates with your workflow.</Text>
                        </div>
                        <div>
                            <ButtonLink to="/community" variant="secondary" outline={true} size="sm">
                                <Icon svgPath={mdiHelpCircleOutline} className="mr-1" aria-hidden={true} />
                                Have feedback? Join our community Discord to let us know!
                            </ButtonLink>
                        </div>
                    </div>
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
                                    className={classNames('d-flex flex-column flex-1 p-3', {
                                        'border-left': index !== 0,
                                    })}
                                >
                                    <div className="d-flex mb-3">
                                        <div>
                                            <img
                                                alt={editor.name}
                                                src={`https://storage.googleapis.com/sourcegraph-assets/ideIcons/ideIcon${editor.icon}.svg`}
                                                width={34}
                                                className="mr-2"
                                            />
                                        </div>
                                        <div>
                                            <Text className="text-muted mb-0" size="small">
                                                {editor.publisher}
                                            </Text>
                                            <Text className={classNames('mb-0', styles.ideName)}>{editor.name}</Text>
                                        </div>
                                    </div>
                                    <Link
                                        to="#"
                                        className={!editor.instructions ? 'text-muted' : ''}
                                        onClick={() => {
                                            setSelectedEditor(editor)
                                            setSelectedEditorStep(0)
                                        }}
                                    >
                                        <Text size="small" className="mb-2">
                                            <Icon svgPath={mdiDownload} aria-hidden={true} /> How to install
                                        </Text>
                                    </Link>
                                    <Link
                                        to="#"
                                        className={!editor.instructions ? 'text-muted' : ''}
                                        onClick={() => {
                                            setSelectedEditor(editor)
                                            setSelectedEditorStep(1)
                                        }}
                                    >
                                        <Text size="small">
                                            <Icon svgPath={mdiInformation} aria-hidden={true} /> How to use
                                        </Text>
                                    </Link>
                                    {selectedEditor?.name === editor.name &&
                                        selectedEditorStep !== null &&
                                        editor.instructions && (
                                            <Modal
                                                key={index + '-modal'}
                                                isOpen={true}
                                                aria-label={`${editor.name} Info`}
                                                className={styles.modal}
                                                position="center"
                                            >
                                                <editor.instructions
                                                    showStep={selectedEditorStep}
                                                    onClose={() => {
                                                        setSelectedEditor(null)
                                                        setSelectedEditorStep(null)
                                                    }}
                                                />
                                            </Modal>
                                        )}
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
            </Page>
            <CodyOnboarding />
        </>
    )
}

const AutocompletesIcon = (): ReactElement => (
    <svg width="33" height="34" viewBox="0 0 33 34" fill="none" xmlns="http://www.w3.org/2000/svg">
        <rect width="33" height="34" rx="16.5" fill="#6B47D6" />
        <rect width="33" height="34" rx="16.5" fill="url(#paint0_linear_2692_3962)" />
        <path
            d="M18.0723 24.8147L14.9142 21.6566L15.9658 20.5943L18.0723 22.7008L22.4826 18.2799L23.5343 19.3421L18.0723 24.8147ZM9.5166 20.1419L13.331 10.2329H14.924L18.7277 20.1419H17.1161L16.1305 17.5438H11.9834L11.0084 20.1419H9.5166ZM12.3829 16.2867H15.7334L14.1079 11.7981H14.006L12.3829 16.2867Z"
            fill="white"
        />
        <defs>
            <linearGradient
                id="paint0_linear_2692_3962"
                x1="16.5"
                y1="0"
                x2="16.5"
                y2="34"
                gradientUnits="userSpaceOnUse"
            >
                <stop stopColor="#FF3424" />
                <stop offset="1" stopColor="#CF275B" />
            </linearGradient>
        </defs>
    </svg>
)

const ChatMessagesIcon = (): ReactElement => (
    <svg width="34" height="34" viewBox="0 0 34 34" fill="none" xmlns="http://www.w3.org/2000/svg">
        <rect x="0.5" width="33" height="34" rx="16.5" fill="#6B47D6" />
        <rect x="0.5" width="33" height="34" rx="16.5" fill="url(#paint0_linear_2692_3970)" />
        <path
            d="M12.4559 18.5188H18.4046V17.3938H12.4559V18.5188ZM12.4559 16.0267H21.544V14.9017H12.4559V16.0267ZM12.4559 13.5533H21.544V12.4283H12.4559V13.5533ZM9.14697 24.8832V10.6683C9.14697 10.2466 9.3022 9.87948 9.61265 9.56695C9.92311 9.25441 10.2877 9.09814 10.7065 9.09814H23.2934C23.7151 9.09814 24.0822 9.25441 24.3948 9.56695C24.7073 9.87948 24.8635 10.2466 24.8635 10.6683V20.2495C24.8635 20.6683 24.7073 21.0329 24.3948 21.3433C24.0822 21.6538 23.7151 21.809 23.2934 21.809H12.2211L9.14697 24.8832ZM11.7035 20.2495H23.2934V10.6683H10.7065V21.359L11.7035 20.2495Z"
            fill="white"
        />
        <defs>
            <linearGradient id="paint0_linear_2692_3970" x1="17" y1="0" x2="17" y2="34" gradientUnits="userSpaceOnUse">
                <stop stopColor="#03C9ED" />
                <stop offset="1" stopColor="#536AEA" />
            </linearGradient>
        </defs>
    </svg>
)

const TrialPeriodIcon = (): ReactElement => (
    <svg width="34" height="34" viewBox="0 0 34 34" fill="none" xmlns="http://www.w3.org/2000/svg">
        <rect x="0.5" width="33" height="34" rx="16.5" fill="#6B47D6" />
        <rect x="0.5" width="33" height="34" rx="16.5" fill="url(#paint0_linear_2898_1552)" />
        <path
            d="M17 27C14.7 27 12.6958 26.2375 10.9875 24.7125C9.27917 23.1875 8.3 21.2833 8.05 19H10.1C10.3333 20.7333 11.1042 22.1667 12.4125 23.3C13.7208 24.4333 15.25 25 17 25C18.95 25 20.6042 24.3208 21.9625 22.9625C23.3208 21.6042 24 19.95 24 18C24 16.05 23.3208 14.3958 21.9625 13.0375C20.6042 11.6792 18.95 11 17 11C15.85 11 14.775 11.2667 13.775 11.8C12.775 12.3333 11.9333 13.0667 11.25 14H14V16H8V10H10V12.35C10.85 11.2833 11.8875 10.4583 13.1125 9.875C14.3375 9.29167 15.6333 9 17 9C18.25 9 19.4208 9.2375 20.5125 9.7125C21.6042 10.1875 22.5542 10.8292 23.3625 11.6375C24.1708 12.4458 24.8125 13.3958 25.2875 14.4875C25.7625 15.5792 26 16.75 26 18C26 19.25 25.7625 20.4208 25.2875 21.5125C24.8125 22.6042 24.1708 23.5542 23.3625 24.3625C22.5542 25.1708 21.6042 25.8125 20.5125 26.2875C19.4208 26.7625 18.25 27 17 27ZM19.8 22.2L16 18.4V13H18V17.6L21.2 20.8L19.8 22.2Z"
            fill="white"
        />
        <defs>
            <linearGradient
                id="paint0_linear_2898_1552"
                x1="17"
                y1="34"
                x2="17"
                y2="-1.57923e-07"
                gradientUnits="userSpaceOnUse"
            >
                <stop stopColor="#F59F00" />
                <stop offset="1" stopColor="#FBD999" />
            </linearGradient>
        </defs>
    </svg>
)
