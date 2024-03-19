import type { ReactElement } from 'react'
import React, { useEffect } from 'react'

import { mdiArrowLeft, mdiInformationOutline, mdiTrendingUp, mdiCreditCardOutline } from '@mdi/js'
import classNames from 'classnames'
import { useNavigate } from 'react-router-dom'

import { useQuery } from '@sourcegraph/http-client'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import {
    Badge,
    Button,
    ButtonLink,
    Link,
    H1,
    H2,
    Icon,
    PageHeader,
    Text,
    Tooltip,
    useSearchParameters,
} from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../auth'
import { Page } from '../../components/Page'
import { PageTitle } from '../../components/PageTitle'
import { CodySubscriptionPlan } from '../../graphql-operations'
import type { UserCodyPlanResult, UserCodyPlanVariables } from '../../graphql-operations'
import { eventLogger } from '../../tracking/eventLogger'
import { EventName } from '../../util/constants'
import { CodyColorIcon } from '../chat/CodyPageIcon'
import { useIsCodyPaymentsTestingMode } from '../featureFlags'
import { isCodyEnabled } from '../isCodyEnabled'

import { USER_CODY_PLAN } from './queries'

import styles from './CodySubscriptionPage.module.scss'

interface CodySubscriptionPageProps extends TelemetryV2Props {
    isSourcegraphDotCom: boolean
    authenticatedUser?: AuthenticatedUser | null
}

export const useCodyPaymentsUrl = (): string => {
    const isCodyPaymentsTestingMode = useIsCodyPaymentsTestingMode()

    if (isCodyPaymentsTestingMode) {
        return 'https://accounts.sgdev.org'
    }

    return 'https://accounts.sourcegraph.com'
}

export const CodySubscriptionPage: React.FunctionComponent<CodySubscriptionPageProps> = ({
    isSourcegraphDotCom,
    authenticatedUser,
    telemetryRecorder,
}) => {
    const parameters = useSearchParameters()

    const utm_source = parameters.get('utm_source')

    const codyPaymentsUrl = useCodyPaymentsUrl()
    const manageSubscriptionRedirectURL = `${codyPaymentsUrl}/cody/subscription`

    useEffect(() => {
        eventLogger.log(EventName.CODY_SUBSCRIPTION_PAGE_VIEWED, { utm_source }, { utm_source })
        telemetryRecorder.recordEvent('cody.planSelection', 'view')
    }, [utm_source, telemetryRecorder])

    const { data, error: dataError } = useQuery<UserCodyPlanResult, UserCodyPlanVariables>(USER_CODY_PLAN, {})

    const navigate = useNavigate()

    useEffect(() => {
        if (!!data && !data?.currentUser) {
            navigate('/sign-in?returnTo=/cody/subscription')
        }
    }, [data, navigate])

    if (dataError) {
        throw dataError
    }

    if (!isCodyEnabled() || !isSourcegraphDotCom || !data?.currentUser || !authenticatedUser) {
        return null
    }

    const isProUser = data.currentUser.codySubscription?.plan === CodySubscriptionPlan.PRO

    return (
        <>
            <Page className={classNames('d-flex flex-column')}>
                <PageTitle title="Cody Subscription" />
                <PageHeader
                    className="mb-4"
                    actions={
                        isProUser && (
                            <Button
                                variant="primary"
                                onClick={() => {
                                    eventLogger.log(EventName.CODY_MANAGE_SUBSCRIPTION_CLICKED)
                                    window.location.href = manageSubscriptionRedirectURL
                                }}
                            >
                                <Icon svgPath={mdiCreditCardOutline} className="mr-1" aria-hidden={true} />
                                Manage subscription
                            </Button>
                        )
                    }
                >
                    <PageHeader.Heading as="h2" styleAs="h1">
                        <div className="d-inline-flex align-items-center">
                            <CodyColorIcon width={40} height={40} className="mr-2" /> Subscription plans
                        </div>
                    </PageHeader.Heading>
                </PageHeader>

                <Link to="/cody/manage" className="my-4">
                    <Icon className="mr-1 text-link" svgPath={mdiArrowLeft} aria-hidden={true} />
                    Back to Cody Dashboard
                </Link>

                <div className={classNames('d-flex mt-4', styles.responsiveContainer)}>
                    <div className="border d-flex flex-column flex-1 bg-1 rounded">
                        <div className="p-4">
                            <div className="border-bottom pb-3">
                                <H1 className="mb-1">Free</H1>
                                <Text className="mb-0 text-muted" size="small">
                                    Best for hobbyists or light usage
                                </Text>
                            </div>
                            <div className="border-bottom py-4">
                                <H1 className="mb-3 py-4">Free</H1>
                            </div>
                            <div className="border-bottom py-4">
                                <Text weight="bold" className="d-inline">
                                    500
                                </Text>{' '}
                                <Text className="d-inline text-muted">autocompletions per month</Text>
                            </div>
                            <div className="border-bottom py-4">
                                <Text weight="bold" className="d-inline">
                                    20
                                </Text>{' '}
                                <Text className="d-inline text-muted">messages and commands per month</Text>
                            </div>
                            <div className="border-bottom py-4">
                                <Text className="text-muted mb-0">Built-in and custom commands</Text>
                            </div>
                            <div className="border-bottom py-4">
                                <Text weight="bold" className="mb-3 d-inline-block">
                                    Code context and personalization
                                </Text>
                                <Text className="mb-0 text-muted">Personalization for small codebases</Text>
                            </div>
                            <div className="border-bottom py-4">
                                <Text weight="bold" className="mb-3 d-inline-block">
                                    LLM support
                                </Text>
                                <Text className="mb-0 text-muted">
                                    Default LLMs for chat, commands, and autocomplete
                                </Text>
                            </div>
                            <div className="border-bottom py-4">
                                <Text weight="bold" className="mb-3">
                                    Compatibility
                                </Text>
                                <Text className="text-muted mb-1">VS Code, JetBrains IDEs, and Neovim</Text>
                                <Text className="text-muted mb-1">
                                    All popular coding languages
                                    <Tooltip content="JavaScript, TypeScript, HTML/CSS, Python, Java, C/C++, C#, PHP, and more">
                                        <Icon
                                            className="ml-1 text-muted"
                                            svgPath={mdiInformationOutline}
                                            aria-hidden={true}
                                        />
                                    </Tooltip>
                                </Text>
                                <Text className="text-muted mb-1">
                                    Many human languages
                                    <Tooltip content="English, Spanish, French, German, Italian, Chinese, Japanese, Korean, and more">
                                        <Icon
                                            className="ml-1 text-muted"
                                            svgPath={mdiInformationOutline}
                                            aria-hidden={true}
                                        />
                                    </Tooltip>
                                </Text>
                                <Text className="text-muted mb-1">
                                    All major code hosts
                                    <Tooltip content="GitHub, GitLab, Bitbucket, Gerrit, Azure DevOps">
                                        <Icon
                                            className="ml-1 text-muted"
                                            svgPath={mdiInformationOutline}
                                            aria-hidden={true}
                                        />
                                    </Tooltip>
                                </Text>
                            </div>
                            <div className="border-bottom py-4">
                                <Text weight="bold" className="mb-3">
                                    Support
                                </Text>
                                <Text className="d-inline text-muted">Community support through Discord</Text>
                            </div>
                        </div>
                    </div>
                    <div className={classNames('border d-flex flex-column flex-1 bg-1 rounded', styles.proContainer)}>
                        <div className={styles.proBorderTop} />
                        <div className="p-4">
                            <div className="border-bottom pb-2">
                                <H1 className={classNames('mb-1', styles.proTitle)}>Pro</H1>
                                <Text className={classNames('mb-1', styles.proDescription)} size="base">
                                    Best for professional developers and small teams
                                </Text>
                            </div>
                            <div className="d-flex flex-column border-bottom py-4">
                                <div className="mb-1">
                                    <H2 className="text-muted d-inline mb-0">$9</H2>
                                    <Text className="mb-0 text-muted d-inline">/month</Text>
                                </div>
                                {isProUser ? (
                                    <Text
                                        className="mb-0 text-muted d-inline cursor-pointer"
                                        size="small"
                                        onClick={() => {
                                            eventLogger.log(EventName.CODY_MANAGE_SUBSCRIPTION_CLICKED)
                                            telemetryRecorder.recordEvent('cody.planSelection', 'click', {
                                                metadata: { tier: 0 },
                                            })
                                            window.location.href = manageSubscriptionRedirectURL
                                        }}
                                    >
                                        Manage subscription
                                    </Text>
                                ) : (
                                    <Button
                                        className="flex-1"
                                        variant="primary"
                                        onClick={() => {
                                            eventLogger.log(
                                                EventName.CODY_SUBSCRIPTION_PLAN_CLICKED,
                                                { tier: 'pro' },
                                                { tier: 'pro' }
                                            )
                                            telemetryRecorder.recordEvent('cody.planSelection', 'click', {
                                                metadata: { tier: 1 },
                                            })
                                            window.location.href = manageSubscriptionRedirectURL
                                        }}
                                    >
                                        <Icon svgPath={mdiTrendingUp} className="mr-1" aria-hidden={true} />
                                        <span>Purchase Cody Pro</span>
                                    </Button>
                                )}
                            </div>
                            <div className="border-bottom py-4">
                                <Text weight="bold" className="d-inline">
                                    Unlimited
                                </Text>{' '}
                                <Text className="d-inline text-muted">autocompletions per month</Text>
                            </div>
                            <div className="border-bottom py-4">
                                <Text weight="bold" className="d-inline">
                                    Unlimited
                                </Text>{' '}
                                <Text className="d-inline text-muted">messages and commands per month</Text>
                            </div>
                            <div className="border-bottom py-4">
                                <Text className="text-muted mb-0">Built-in and custom commands</Text>
                            </div>
                            <div className="border-bottom py-4">
                                <Text weight="bold" className="mb-3 d-inline-block">
                                    Code context and personalization
                                </Text>
                                <Text className="mb-0 text-muted">Personalization for larger codebases</Text>
                            </div>
                            <div className="border-bottom py-4">
                                <Text weight="bold" className="mb-3 d-inline-block">
                                    LLM support
                                </Text>
                                <Text className="mb-1 text-muted">
                                    Multiple LLM choices for chat
                                    <Tooltip content="Claude Instant 1.2, Claude 2, ChatGPT 3.5 Turbo, ChatGPT 4 Turbo Preview">
                                        <Icon
                                            className="ml-1 text-muted"
                                            svgPath={mdiInformationOutline}
                                            aria-hidden={true}
                                        />
                                    </Tooltip>
                                </Text>
                                <Text className="mb-0 text-muted">Default LLMs for commands and autocomplete</Text>
                            </div>
                            <div className="border-bottom py-4">
                                <Text weight="bold" className="mb-3">
                                    Compatibility
                                </Text>
                                <Text className="text-muted mb-1">VS Code, JetBrains IDEs, and Neovim</Text>
                                <Text className="text-muted mb-1">
                                    All popular coding languages
                                    <Tooltip content="JavaScript, TypeScript, HTML/CSS, Python, Java, C/C++, C#, PHP, and more">
                                        <Icon
                                            className="ml-1 text-muted"
                                            svgPath={mdiInformationOutline}
                                            aria-hidden={true}
                                        />
                                    </Tooltip>
                                </Text>
                                <Text className="text-muted mb-1">
                                    Many human languages
                                    <Tooltip content="English, Spanish, French, German, Italian, Chinese, Japanese, Korean, and more">
                                        <Icon
                                            className="ml-1 text-muted"
                                            svgPath={mdiInformationOutline}
                                            aria-hidden={true}
                                        />
                                    </Tooltip>
                                </Text>
                                <Text className="text-muted mb-1">
                                    All major code hosts
                                    <Tooltip content="GitHub, GitLab, Bitbucket, Gerrit, Azure DevOps">
                                        <Icon
                                            className="ml-1 text-muted"
                                            svgPath={mdiInformationOutline}
                                            aria-hidden={true}
                                        />
                                    </Tooltip>
                                </Text>
                            </div>
                            <div className="border-bottom py-4">
                                <Text weight="bold" className="mb-3">
                                    Support
                                </Text>
                                <Text className="d-inline text-muted">Email support with limited SLAs</Text>
                            </div>
                        </div>
                    </div>
                    <div className="border d-flex flex-column flex-1 bg-1 border p-3 rounded">
                        <div className="border-bottom pb-4">
                            <H1 className="mb-1 d-flex align-items-center">Enterprise</H1>
                            <Text className="mb-0" size="small">
                                Best for large teams and enterprises
                            </Text>
                        </div>
                        <div className="d-flex flex-column border-bottom py-4">
                            <div className="mb-1">
                                <H2 className="text-muted d-inline mb-0">$19</H2>
                                <Text className="mb-0 text-muted d-inline">/user/month</Text>
                            </div>
                            <ButtonLink
                                className="flex-1 mt-3"
                                variant="secondary"
                                outline={true}
                                to="https://sourcegraph.com/contact/request-info?utm_source=cody_subscription_page"
                                target="_blank"
                                onClick={() => {
                                    eventLogger.log(
                                        EventName.CODY_SUBSCRIPTION_PLAN_CLICKED,
                                        { tier: 'enterprise' },
                                        { tier: 'enterprise' }
                                    )
                                    telemetryRecorder.recordEvent('cody.planSelection', 'click', {
                                        metadata: { tier: 2 },
                                    })
                                }}
                            >
                                Request info
                            </ButtonLink>
                        </div>
                        <div className="border-bottom py-4">
                            <Text weight="bold" className="d-inline">
                                Unlimited
                            </Text>{' '}
                            <Text className="d-inline text-muted">autocompletions per month</Text>
                        </div>
                        <div className="border-bottom py-4">
                            <Text weight="bold" className="d-inline">
                                Unlimited
                            </Text>{' '}
                            <Text className="d-inline text-muted">messages and commands per month</Text>
                        </div>
                        <div className="border-bottom py-4">
                            <Text className="text-muted mb-0">Built-in and custom commands</Text>
                        </div>
                        <div className="border-bottom py-4">
                            <Text weight="bold" className="mb-3 d-inline-block">
                                Code context and personalization
                            </Text>
                            <Text className="mb-0 text-muted">Advanced personalization for Enterprise codebases</Text>
                        </div>
                        <div className="border-bottom py-4">
                            <Text weight="bold" className="mb-3 d-inline-block">
                                LLM support
                            </Text>
                            <Text className="mb-1 text-muted">
                                Flexible LLM choices
                                <Tooltip content="Claude Instant 1.2, Claude 2, ChatGPT 3.5 Turbo, ChatGPT 4 Turbo Preview">
                                    <Icon
                                        className="ml-1 text-muted"
                                        svgPath={mdiInformationOutline}
                                        aria-hidden={true}
                                    />
                                </Tooltip>
                            </Text>
                            <Text className="mb-1 text-muted">
                                Bring your own LLM key
                                <Tooltip content="Bring your own LLM key with Azure OpenAI or Amazon Bedrock">
                                    <Icon
                                        className="ml-1 text-muted"
                                        svgPath={mdiInformationOutline}
                                        aria-hidden={true}
                                    />
                                </Tooltip>
                            </Text>
                            <Text className="mb-0 text-muted">
                                Bring your own LLM <Badge variant="secondary">coming soon</Badge>
                            </Text>
                        </div>
                        <div className="border-bottom py-4">
                            <Text weight="bold" className="mb-3">
                                Enterprise features
                            </Text>
                            <Text className="mb-1 text-muted">Everything in Pro, plus:</Text>
                            <Text className="mb-1 text-muted">Enterprise support</Text>
                            <Text className="mb-1 text-muted">Flexible deployment options</Text>
                            <Text className="mb-1 text-muted">
                                Enterprise admin and security features (SSO, SAML, SCIM, audit logs, etc.)
                            </Text>
                            <Text className="mb-1 text-muted">
                                Guardrails
                                <Tooltip content="We scan Cody's output for OSS code, reducing the risk of copyrighted code in suggestions">
                                    <Icon
                                        className="ml-1 text-muted"
                                        svgPath={mdiInformationOutline}
                                        aria-hidden={true}
                                    />
                                </Tooltip>
                            </Text>
                        </div>
                    </div>
                </div>
            </Page>
        </>
    )
}

export const ProTierIcon = ({ className }: { className?: string }): ReactElement => (
    <svg
        xmlns="http://www.w3.org/2000/svg"
        width="56"
        height="31"
        fill="none"
        viewBox="0 0 56 31"
        className={className}
    >
        <g filter="url(#filter0_d_2692_3595)">
            <path
                fill="color(display-p3 0.9373 0.9490 0.9608)"
                d="M1.902 28V.364h10.903c2.097 0 3.882.4 5.358 1.2 1.475.792 2.6 1.894 3.373 3.307.783 1.403 1.174 3.022 1.174 4.858 0 1.835-.396 3.454-1.187 4.858-.792 1.403-1.939 2.496-3.441 3.279-1.494.782-3.302 1.174-5.425 1.174h-6.95v-4.683h6.005c1.125 0 2.052-.193 2.78-.58.738-.396 1.287-.94 1.646-1.633.37-.701.554-1.507.554-2.415 0-.918-.185-1.719-.553-2.402-.36-.693-.91-1.228-1.647-1.606-.738-.387-1.673-.58-2.807-.58h-3.94V28H1.902zm21.495 0V7.273h5.574v3.616h.216c.377-1.286 1.012-2.258 1.902-2.915.89-.665 1.916-.998 3.077-.998.288 0 .598.018.931.054.333.036.625.085.877.148v5.101a7.524 7.524 0 00-1.12-.216 10.056 10.056 0 00-1.309-.094c-.845 0-1.601.184-2.267.553a4.074 4.074 0 00-1.565 1.511c-.378.648-.567 1.395-.567 2.24V28h-5.748zm20.95.405c-2.096 0-3.91-.446-5.439-1.336-1.52-.9-2.694-2.15-3.522-3.752-.827-1.61-1.241-3.477-1.241-5.6 0-2.14.414-4.012 1.241-5.613.828-1.61 2.002-2.861 3.522-3.752 1.53-.9 3.343-1.35 5.439-1.35s3.904.45 5.424 1.35c1.53.89 2.708 2.141 3.536 3.752.828 1.601 1.241 3.472 1.241 5.613 0 2.123-.413 3.99-1.241 5.6-.828 1.602-2.006 2.852-3.536 3.752-1.52.89-3.328 1.336-5.424 1.336zm.027-4.453c.953 0 1.75-.27 2.388-.81.639-.549 1.12-1.295 1.444-2.24.333-.945.5-2.02.5-3.225 0-1.206-.167-2.28-.5-3.225-.324-.945-.805-1.692-1.444-2.24-.639-.55-1.435-.823-2.388-.823-.963 0-1.773.274-2.43.823-.647.548-1.137 1.295-1.47 2.24-.324.944-.486 2.02-.486 3.225s.162 2.28.486 3.225c.333.945.823 1.691 1.47 2.24.657.54 1.467.81 2.43.81z"
            />
            <path
                fill="url(#paint0_linear_2692_3595)"
                d="M1.902 28V.364h10.903c2.097 0 3.882.4 5.358 1.2 1.475.792 2.6 1.894 3.373 3.307.783 1.403 1.174 3.022 1.174 4.858 0 1.835-.396 3.454-1.187 4.858-.792 1.403-1.939 2.496-3.441 3.279-1.494.782-3.302 1.174-5.425 1.174h-6.95v-4.683h6.005c1.125 0 2.052-.193 2.78-.58.738-.396 1.287-.94 1.646-1.633.37-.701.554-1.507.554-2.415 0-.918-.185-1.719-.553-2.402-.36-.693-.91-1.228-1.647-1.606-.738-.387-1.673-.58-2.807-.58h-3.94V28H1.902zm21.495 0V7.273h5.574v3.616h.216c.377-1.286 1.012-2.258 1.902-2.915.89-.665 1.916-.998 3.077-.998.288 0 .598.018.931.054.333.036.625.085.877.148v5.101a7.524 7.524 0 00-1.12-.216 10.056 10.056 0 00-1.309-.094c-.845 0-1.601.184-2.267.553a4.074 4.074 0 00-1.565 1.511c-.378.648-.567 1.395-.567 2.24V28h-5.748zm20.95.405c-2.096 0-3.91-.446-5.439-1.336-1.52-.9-2.694-2.15-3.522-3.752-.827-1.61-1.241-3.477-1.241-5.6 0-2.14.414-4.012 1.241-5.613.828-1.61 2.002-2.861 3.522-3.752 1.53-.9 3.343-1.35 5.439-1.35s3.904.45 5.424 1.35c1.53.89 2.708 2.141 3.536 3.752.828 1.601 1.241 3.472 1.241 5.613 0 2.123-.413 3.99-1.241 5.6-.828 1.602-2.006 2.852-3.536 3.752-1.52.89-3.328 1.336-5.424 1.336zm.027-4.453c.953 0 1.75-.27 2.388-.81.639-.549 1.12-1.295 1.444-2.24.333-.945.5-2.02.5-3.225 0-1.206-.167-2.28-.5-3.225-.324-.945-.805-1.692-1.444-2.24-.639-.55-1.435-.823-2.388-.823-.963 0-1.773.274-2.43.823-.647.548-1.137 1.295-1.47 2.24-.324.944-.486 2.02-.486 3.225s.162 2.28.486 3.225c.333.945.823 1.691 1.47 2.24.657.54 1.467.81 2.43.81z"
            />
        </g>
        <defs>
            <filter
                id="filter0_d_2692_3595"
                width="54.647"
                height="30.041"
                x="0.902"
                y="0.364"
                colorInterpolationFilters="sRGB"
                filterUnits="userSpaceOnUse"
            >
                <feFlood floodOpacity="0" result="BackgroundImageFix" />
                <feColorMatrix in="SourceAlpha" result="hardAlpha" values="0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 0 127 0" />
                <feOffset dy="1" />
                <feGaussianBlur stdDeviation="0.5" />
                <feComposite in2="hardAlpha" operator="out" />
                <feColorMatrix values="0 0 0 0 0.278089 0 0 0 0 0.267405 0 0 0 0 0.267405 0 0 0 0.25 0" />
                <feBlend in2="BackgroundImageFix" result="effect1_dropShadow_2692_3595" />
                <feBlend in="SourceGraphic" in2="effect1_dropShadow_2692_3595" result="shape" />
            </filter>
            <linearGradient
                id="paint0_linear_2692_3595"
                x1="16.5"
                x2="46.674"
                y1="32"
                y2="3.137"
                gradientUnits="userSpaceOnUse"
            >
                <stop stopColor="color(display-p3 0.9266 0.3028 0.2851)" />
                <stop offset="0.492" stopColor="color(display-p3 0.4392 0.2824 0.9098)" />
                <stop offset="1" stopColor="color(display-p3 0.2902 0.7569 0.9098)" />
            </linearGradient>
        </defs>
    </svg>
)
