import { useEffect } from 'react'

import { mdiChevronRight, mdiCodeBracesBox, mdiGit } from '@mdi/js'
import classNames from 'classnames'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Theme, useTheme } from '@sourcegraph/shared/src/theme'
import { Badge, H1, H2, H3, H4, Icon, Link, PageHeader, Text } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../../auth'
import { ExternalsAuth } from '../../../auth/components/ExternalsAuth'
import { MarketingBlock } from '../../../components/MarketingBlock'
import { Page } from '../../../components/Page'
import { PageTitle } from '../../../components/PageTitle'
import type { SourcegraphContext } from '../../../jscontext'
import { MeetCodySVG } from '../../../repo/components/TryCodyWidget/WidgetIcons'
import { eventLogger } from '../../../tracking/eventLogger'
import { EventName } from '../../../util/constants'
import { CodyColorIcon, CodyHelpIcon, CodyWorkIcon } from '../../chat/CodyPageIcon'

import styles from './CodyMarketingPage.module.scss'

interface CodyPlatformCardProps {
    icon: string | JSX.Element
    title: string | JSX.Element
    description: string | JSX.Element
    illustration: string
}

/* eslint-disable  @sourcegraph/sourcegraph/check-help-links */

const onSpeakToAnEngineer = (): void => eventLogger.log(EventName.SPEAK_TO_AN_ENGINEER_CTA)

const IDEIcon: React.FunctionComponent<{}> = () => (
    <svg viewBox="-4 -4 31 31" fill="none" xmlns="http://www.w3.org/2000/svg" className={styles.codyPlatformCardIcon}>
        <rect x="0.811523" y="0.366669" width="25" height="25" rx="3" fill="#4D52F4" />
        <path
            d="M13.8115 20.1583C13.8115 20.4346 13.9169 20.6996 14.1044 20.8949C14.292 21.0903 14.5463 21.2 14.8115 21.2H16.8115V23.2833H14.3115C13.7615 23.2833 12.8115 22.8146 12.8115 22.2417C12.8115 22.8146 11.8615 23.2833 11.3115 23.2833H8.81152V21.2H10.8115C11.0767 21.2 11.3311 21.0903 11.5186 20.8949C11.7062 20.6996 11.8115 20.4346 11.8115 20.1583V5.57501C11.8115 5.29874 11.7062 5.03379 11.5186 4.83844C11.3311 4.64309 11.0767 4.53335 10.8115 4.53335H8.81152V2.45001H11.3115C11.8615 2.45001 12.8115 2.91876 12.8115 3.49168C12.8115 2.91876 13.7615 2.45001 14.3115 2.45001H16.8115V4.53335H14.8115C14.5463 4.53335 14.292 4.64309 14.1044 4.83844C13.9169 5.03379 13.8115 5.29874 13.8115 5.57501V20.1583Z"
            fill="white"
        />
    </svg>
)

const codyPlatformCardItems = (
    isSourcegraphDotCom: boolean
): {
    title: string | JSX.Element
    description: string | JSX.Element
    icon: string | JSX.Element
    illustration: { dark: string; light: string }
}[] => [
    {
        title: 'Knows your code',
        description:
            'Cody knows about your codebase and can use that knowledge to explain, generate, and improve your code.',
        icon: mdiCodeBracesBox,
        illustration: {
            dark: 'https://storage.googleapis.com/sourcegraph-assets/app-images/cody-knows-your-code-illustration-dark.png',
            light: 'https://storage.googleapis.com/sourcegraph-assets/app-images/cody-knows-your-code-illustration-light.png',
        },
    },
    {
        title: 'Cody in your editor',
        description: (
            <>
                The extensions combine an LLM with the context of your code to help you generate and fix code more
                accurately. <Link to="/help/cody#get-cody">View supported editors.</Link>
            </>
        ),
        icon: <IDEIcon />,
        illustration: {
            dark: 'https://storage.googleapis.com/sourcegraph-assets/app-images/cody-vs-code-illustration-dark.png',
            light: 'https://storage.googleapis.com/sourcegraph-assets/app-images/cody-vs-code-illustration-light.png',
        },
    },
    ...(isSourcegraphDotCom
        ? [
              {
                  title: (
                      <>
                          Try it on sourcegraph.com{' '}
                          <Badge variant="info" className="d-inline">
                              Experimental
                          </Badge>
                      </>
                  ),
                  description:
                      'Cody explains, generates, convert code, and more within the context of public repositories.',
                  icon: mdiGit,
                  illustration: {
                      dark: 'https://storage.googleapis.com/sourcegraph-assets/app-images/cody-com-illustration-dark.png',
                      light: 'https://storage.googleapis.com/sourcegraph-assets/app-images/cody-com-illustration-light.png',
                  },
              },
          ]
        : [
              {
                  title: 'Recipes accelerate your flow',
                  description:
                      'Cody explains, generates, convert code, and more within the context of your repositories.',
                  icon: mdiGit,
                  illustration: {
                      dark: 'https://storage.googleapis.com/sourcegraph-assets/app-images/cody-com-illustration-dark.png',
                      light: 'https://storage.googleapis.com/sourcegraph-assets/app-images/cody-com-illustration-light.png',
                  },
              },
          ]),
]

export interface CodyMarketingPageProps extends TelemetryV2Props {
    isSourcegraphDotCom: boolean
    context: Pick<SourcegraphContext, 'externalURL'>
    authenticatedUser: AuthenticatedUser | null
}

export const CodyMarketingPage: React.FunctionComponent<CodyMarketingPageProps> = ({
    context,
    isSourcegraphDotCom,
    authenticatedUser,
    telemetryRecorder,
}) => {
    const { theme } = useTheme()
    const isDarkTheme = theme === Theme.Dark

    useEffect(() => telemetryRecorder.recordEvent('cody.marketing', 'view'), [telemetryRecorder])

    return (
        <Page>
            <PageTitle title="Cody" />
            <PageHeader
                description={
                    <>
                        Cody answers code questions and writes code for you by reading your entire codebase and the code
                        graph.
                    </>
                }
                className={classNames('mb-3', styles.pageHeader)}
            >
                <PageHeader.Heading as="h2" styleAs="h1">
                    <PageHeader.Breadcrumb icon={CodyColorIcon}>
                        <div className={classNames('d-inline-flex align-items-center', styles.pageHeaderBreadcrumb)}>
                            Cody
                        </div>
                    </PageHeader.Breadcrumb>
                </PageHeader.Heading>
            </PageHeader>

            {/* Page content */}
            <div className={styles.headerSection}>
                <div>
                    {isSourcegraphDotCom && <H1>Meet Cody, your AI assistant</H1>}
                    <div className="ml-3">
                        <Text className={styles.codyConversation}>AI-powered chat for you code</Text>
                        <Text className={styles.codyConversation}>Autocomplete</Text>
                        <Text className={styles.codyConversation}>Find, fix and explain code</Text>
                        <Text className={styles.codyConversation}>Create documentation</Text>
                        <Text className={styles.codyConversation}>Generate unit tests</Text>
                        <Text className={styles.codyConversation}>Build custom commands</Text>
                    </div>
                    <CodyHelpIcon />
                </div>

                {isSourcegraphDotCom ? (
                    <MarketingBlock
                        contentClassName={styles.codySignUpPanel}
                        wrapperClassName={styles.codySignUpPanelWrapper}
                    >
                        <H2>Sign up to get free access</H2>
                        <Text className="mt-3">
                            Cody answers technical questions and writes code directly in your IDE, using your code graph
                            for context and accuracy. Sign up with:
                        </Text>
                        <div className={styles.buttonWrapper}>
                            <ExternalsAuth
                                page="cody-marketing-page"
                                context={context}
                                githubLabel="GitHub"
                                gitlabLabel="GitLab"
                                googleLabel="Google"
                                withCenteredText={true}
                                onClick={() => {}}
                                ctaClassName={styles.authButton}
                                iconClassName={styles.buttonIcon}
                                telemetryRecorder={telemetryRecorder}
                                telemetryService={eventLogger}
                            />
                        </div>
                        <Text className="mt-3 mb-0">
                            By registering, you agree to our{' '}
                            <Link
                                className={styles.termsPrivacyLink}
                                to="https://sourcegraph.com/terms"
                                target="_blank"
                                rel="noopener"
                            >
                                Terms of Service
                            </Link>{' '}
                            and{' '}
                            <Link
                                className={styles.termsPrivacyLink}
                                to="https://sourcegraph.com/privacy"
                                target="_blank"
                                rel="noopener"
                            >
                                Privacy Policy
                            </Link>
                            .
                        </Text>
                    </MarketingBlock>
                ) : (
                    <MarketingBlock
                        contentClassName={styles.codySignUpPanel}
                        wrapperClassName={styles.codySignUpPanelWrapper}
                    >
                        <H2>Meet Cody, your AI assistant</H2>
                        <Text className="mt-3">
                            Cody is an AI assistant that leverages the code graph to know more about your code. Use it
                            to:
                        </Text>
                        <ul>
                            <li>Onboard to new codebases</li>
                            <li>Evaluate and fix code</li>
                            <li>Write code faster</li>
                        </ul>
                        <Text className="mb-0">
                            <Link to="https://sourcegraph.com/cody">Learn more about Cody &rarr;</Link>
                            {authenticatedUser?.siteAdmin && (
                                <>
                                    {' '}
                                    or <Link to="/help/cody/explanations/enabling_cody_enterprise">enable it now</Link>.
                                </>
                            )}
                        </Text>
                    </MarketingBlock>
                )}
            </div>

            <div className={styles.learnMoreSection}>
                <H2>Enhancing productivity with Cody</H2>

                <div
                    className={classNames(
                        'd-flex flex-row flex-wrap mt-3 justify-content-center',
                        styles.startingPointWrapper
                    )}
                >
                    {codyPlatformCardItems(isSourcegraphDotCom).map((item, index) => (
                        <CodyPlatformCard
                            key={index}
                            title={item.title}
                            description={item.description}
                            illustration={isDarkTheme ? item.illustration.dark : item.illustration.light}
                            icon={item.icon}
                        />
                    ))}
                </div>

                <div className={styles.learnMoreItemsWrapper}>
                    {isSourcegraphDotCom ? (
                        <div className={styles.learnMoreItems}>
                            <H4 className={styles.learnMoreItemsTitle}>Overview</H4>
                            <Text className="mb-0">
                                Visit the{' '}
                                <Link to="https://sourcegraph.com/cody" target="_blank" rel="noopener">
                                    product page
                                </Link>{' '}
                                and see what devs are building with Cody.
                            </Text>
                        </div>
                    ) : (
                        <div className="d-flex align-items-center">
                            <div>
                                <MeetCodySVG />
                            </div>
                            <Text className="ml-3">
                                <Link to="https://sourcegraph.com/cody">Learn about Cody</Link>, Sourcegraph's AI coding
                                assistant.
                            </Text>
                        </div>
                    )}

                    <div className={styles.learnMoreItems}>
                        <H4 className={styles.learnMoreItemsTitle}>Documentation</H4>
                        <Text className="mb-0">
                            Learn about Cody’s use cases, commands, and FAQs on the{' '}
                            <Link to="/help/cody" target="_blank" rel="noopener">
                                documentation page
                            </Link>
                            .
                        </Text>
                    </div>
                </div>
            </div>

            {isSourcegraphDotCom && (
                <div className={styles.footer}>
                    <CodyWorkIcon />
                    <div>
                        <H1 className="mb-2">Get Cody for work</H1>
                        <Text className={styles.footerDescription}>
                            Cody for Sourcegraph Enterprise utilizes Sourcegraph's code graph to deliver context-aware
                            answers based on your private codebase, enabling enhanced code comprehension and
                            productivity.
                        </Text>
                        <div className="mb-2">
                            <Link
                                to="https://sourcegraph.com/demo"
                                className={classNames('d-inline-flex align-items-center', styles.footerCtaLink)}
                                onClick={onSpeakToAnEngineer}
                            >
                                Speak to an engineer
                                <Icon svgPath={mdiChevronRight} aria-hidden={true} />
                            </Link>
                        </div>
                    </div>
                </div>
            )}
        </Page>
    )
}

const CodyPlatformCard: React.FunctionComponent<CodyPlatformCardProps> = ({
    icon,
    title,
    description,
    illustration,
}) => (
    <div className={styles.codyPlatformCardWrapper}>
        <div className="d-flex flex-row align-items-center">
            {typeof icon === 'string' ? (
                <Icon svgPath={icon} aria-hidden={true} className={styles.codyPlatformCardIcon} />
            ) : (
                <>{icon}</>
            )}
            <H3 className="ml-2 mb-0">{title}</H3>
        </div>
        <Text className={classNames('mt-2', styles.codyPlatformCardDescription)}>{description}</Text>
        <img src={illustration} alt="Cody platform card" className={styles.codyPlatformCardImage} />
    </div>
)
