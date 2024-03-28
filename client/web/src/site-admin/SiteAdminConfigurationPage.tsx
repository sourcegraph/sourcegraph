import * as React from 'react'
import type { FC } from 'react'

import { type ApolloClient, useApolloClient } from '@apollo/client'
import classNames from 'classnames'
import * as jsonc from 'jsonc-parser'
import { lastValueFrom, timer, Subject, Subscription } from 'rxjs'
import { delay, mergeMap, retry, tap, timeout } from 'rxjs/operators'

import { logger } from '@sourcegraph/common'
import type { SiteConfiguration } from '@sourcegraph/shared/src/schema/site.schema'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import type { TelemetryProps } from '@sourcegraph/shared/src/telemetry/telemetryService'
import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import {
    Button,
    LoadingSpinner,
    Link,
    Alert,
    Code,
    Text,
    PageHeader,
    Container,
    ErrorAlert,
} from '@sourcegraph/wildcard'

import siteSchemaJSON from '../../../../schema/site.schema.json'
import { PageTitle } from '../components/PageTitle'
import type { SiteResult } from '../graphql-operations'
import { DynamicallyImportedMonacoSettingsEditor } from '../settings/DynamicallyImportedMonacoSettingsEditor'
import { refreshSiteFlags } from '../site/backend'
import { eventLogger } from '../tracking/eventLogger'

import { fetchSite, reloadSite, updateSiteConfiguration } from './backend'
import { SiteConfigurationChangeList } from './SiteConfigurationChangeList'

import styles from './SiteAdminConfigurationPage.module.scss'

const defaultModificationOptions: jsonc.ModificationOptions = {
    formattingOptions: {
        eol: '\n',
        insertSpaces: true,
        tabSize: 2,
    },
}

function editWithComments(
    config: string,
    path: jsonc.JSONPath,
    value: any,
    comments: { [key: string]: string }
): jsonc.Edit {
    const edit = jsonc.modify(config, path, value, defaultModificationOptions)[0]
    for (const commentKey of Object.keys(comments)) {
        edit.content = edit.content.replace(`"${commentKey}": true,`, comments[commentKey])
        edit.content = edit.content.replace(`"${commentKey}": true`, comments[commentKey])
    }
    return edit
}

const quickConfigureActions: {
    id: string
    label: string
    run: (config: string) => { edits: jsonc.Edit[]; selectText: string }
}[] = [
    {
        id: 'setExternalURL',
        label: 'Set external URL',
        run: config => {
            const value = '<external URL>'
            const edits = jsonc.modify(config, ['externalURL'], value, defaultModificationOptions)
            return { edits, selectText: '<external URL>' }
        },
    },
    {
        id: 'setLicenseKey',
        label: 'Set license key',
        run: config => {
            const value = '<license key>'
            const edits = jsonc.modify(config, ['licenseKey'], value, defaultModificationOptions)
            return { edits, selectText: '<license key>' }
        },
    },
    {
        id: 'addGitLabAuth',
        label: 'Add GitLab sign-in',
        run: config => {
            const edits = [
                editWithComments(
                    config,
                    ['auth.providers', -1],
                    {
                        COMMENT: true,
                        type: 'gitlab',
                        displayName: 'GitLab',
                        url: '<GitLab URL>',
                        clientID: '<client ID>',
                        clientSecret: '<client secret>',
                    },
                    {
                        COMMENT: '// See https://sourcegraph.com/docs/admin/auth#gitlab for instructions',
                    }
                ),
            ]
            return { edits, selectText: '<GitLab URL>' }
        },
    },
    {
        id: 'addGitHubAuth',
        label: 'Add GitHub sign-in',
        run: config => {
            const edits = [
                editWithComments(
                    config,
                    ['auth.providers', -1],
                    {
                        COMMENT: true,
                        type: 'github',
                        displayName: 'GitHub',
                        url: 'https://github.com/',
                        allowSignup: true,
                        clientID: '<client ID>',
                        clientSecret: '<client secret>',
                    },
                    { COMMENT: '// See https://sourcegraph.com/docs/admin/auth#github for instructions' }
                ),
            ]
            return { edits, selectText: '<client ID>' }
        },
    },
    {
        id: 'useOneLoginSAML',
        label: 'Add OneLogin SAML',
        run: config => {
            const edits = [
                editWithComments(
                    config,
                    ['auth.providers', -1],
                    {
                        COMMENT: true,

                        type: 'saml',
                        displayName: 'OneLogin',
                        identityProviderMetadataURL: '<identity provider metadata URL>',
                    },
                    {
                        COMMENT: '// See https://sourcegraph.com/docs/admin/auth/saml/one_login for instructions',
                    }
                ),
            ]
            return { edits, selectText: '<identity provider metadata URL>' }
        },
    },
    {
        id: 'useOktaSAML',
        label: 'Add Okta SAML',
        run: config => {
            const value = {
                COMMENT: true,
                type: 'saml',
                displayName: 'Okta',
                identityProviderMetadataURL: '<identity provider metadata URL>',
            }
            const edits = [
                editWithComments(config, ['auth.providers', -1], value, {
                    COMMENT: '// See https://sourcegraph.com/docs/admin/auth/saml/okta for instructions',
                }),
            ]
            return { edits, selectText: '<identity provider metadata URL>' }
        },
    },
    {
        id: 'useSAML',
        label: 'Add other SAML',
        run: config => {
            const edits = [
                editWithComments(
                    config,
                    ['auth.providers', -1],
                    {
                        COMMENT: true,
                        type: 'saml',
                        displayName: 'SAML',
                        identityProviderMetadataURL: '<SAML IdP metadata URL>',
                    },
                    { COMMENT: '// See https://sourcegraph.com/docs/admin/auth/saml for instructions' }
                ),
            ]
            return { edits, selectText: '<SAML IdP metadata URL>' }
        },
    },
    {
        id: 'useOIDC',
        label: 'Add OpenID Connect',
        run: config => {
            const edits = [
                editWithComments(
                    config,
                    ['auth.providers', -1],
                    {
                        COMMENT: true,
                        type: 'openidconnect',
                        displayName: 'OpenID Connect',
                        issuer: '<identity provider URL>',
                        clientID: '<client ID>',
                        clientSecret: '<client secret>',
                    },
                    { COMMENT: '// See https://sourcegraph.com/docs/admin/auth#openid-connect for instructions' }
                ),
            ]
            return { edits, selectText: '<identity provider URL>' }
        },
    },
]

interface Props extends TelemetryProps, TelemetryV2Props {
    isLightTheme: boolean
    client: ApolloClient<{}>
}

interface State {
    site?: SiteResult['site']
    loading: boolean
    error?: Error

    saving?: boolean
    restartToApply: boolean
    reloadStartedAt?: number
    enabledCompletions?: boolean
}

const EXPECTED_RELOAD_WAIT = 7 * 1000 // 7 seconds

export const SiteAdminConfigurationPage: FC<TelemetryProps & TelemetryV2Props> = props => {
    const client = useApolloClient()
    return <SiteAdminConfigurationContent {...props} isLightTheme={useIsLightTheme()} client={client} />
}

/**
 * A page displaying the site configuration.
 */
class SiteAdminConfigurationContent extends React.Component<Props, State> {
    public state: State = {
        loading: true,
        restartToApply: window.context.needServerRestart,
    }

    private remoteRefreshes = new Subject<void>()
    private siteReloads = new Subject<void>()
    private subscriptions = new Subscription()

    public componentDidMount(): void {
        eventLogger.logViewEvent('SiteAdminConfiguration')
        this.props.telemetryRecorder.recordEvent('admin.configuration', 'view')

        this.subscriptions.add(
            this.remoteRefreshes.pipe(mergeMap(() => fetchSite())).subscribe(
                site => {
                    this.setState({
                        site,
                        error: undefined,
                        loading: false,
                    })
                },
                error => this.setState({ error, loading: false })
            )
        )
        this.remoteRefreshes.next()

        this.subscriptions.add(
            this.siteReloads
                .pipe(
                    tap(() => this.setState({ reloadStartedAt: Date.now(), error: undefined })),
                    mergeMap(reloadSite),
                    delay(2000),
                    mergeMap(() =>
                        // wait for server to restart
                        fetchSite().pipe(
                            retry({
                                delay: () => {
                                    this.forceUpdate()
                                    return timer(500)
                                },
                            }),
                            timeout(10000)
                        )
                    ),
                    tap(() => this.remoteRefreshes.next())
                )
                .subscribe(
                    () => {
                        this.setState({ reloadStartedAt: undefined })
                        window.location.reload() // brute force way to reload view state
                    },
                    error => this.setState({ reloadStartedAt: undefined, error })
                )
        )
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element | null {
        const alerts: JSX.Element[] = []
        if (this.state.error) {
            alerts.push(<ErrorAlert key="error" className={styles.alert} error={this.state.error} />)
        }
        if (this.state.reloadStartedAt) {
            alerts.push(
                <Alert key="error" className={styles.alert} variant="primary">
                    <Text>
                        <LoadingSpinner /> Waiting for site to reload...
                    </Text>
                    {Date.now() - this.state.reloadStartedAt > EXPECTED_RELOAD_WAIT && (
                        <Text>
                            <small>It's taking longer than expected. Check the server logs for error messages.</small>
                        </Text>
                    )}
                </Alert>
            )
        }
        if (this.state.restartToApply) {
            alerts.push(
                <Alert key="remote-dirty" className={classNames(styles.alert, styles.alertFlex)} variant="warning">
                    Server restart is required for the configuration to take effect.
                    {(this.state.site === undefined || this.state.site?.canReloadSite) && (
                        <Button onClick={this.reloadSite} variant="primary" size="sm">
                            Restart server
                        </Button>
                    )}
                </Alert>
            )
        }
        if (
            this.state.site?.configuration?.validationMessages &&
            this.state.site.configuration.validationMessages.length > 0
        ) {
            alerts.push(
                <Alert key="validation-messages" className={styles.alert} variant="danger">
                    <Text>The server reported issues in the last-saved config:</Text>
                    <ul>
                        {this.state.site.configuration.validationMessages.map((message, index) => (
                            <li key={index} className={styles.alertItem}>
                                {message}
                            </li>
                        ))}
                    </ul>
                </Alert>
            )
        }

        // Avoid user confusion with values.yaml properties mixed in with site config properties.
        const contents = this.state.site?.configuration?.effectiveContents
        const legacyKubernetesConfigProps = [
            'alertmanagerConfig',
            'alertmanagerURL',
            'authProxyIP',
            'authProxyPassword',
            'deploymentOverrides',
            'gitoliteIP',
            'gitserverCount',
            'gitserverDiskSize',
            'gitserverSSH',
            'httpNodePort',
            'httpsNodePort',
            'indexedSearchDiskSize',
            'langGo',
            'langJava',
            'langJavaScript',
            'langPHP',
            'langPython',
            'langSwift',
            'langTypeScript',
            'nodeSSDPath',
            'phabricatorIP',
            'prometheus',
            'pyPIIP',
            'rbac',
            'storageClass',
            'useAlertManager',
        ].filter(property => contents?.includes(`"${property}"`))
        if (legacyKubernetesConfigProps.length > 0) {
            alerts.push(
                <Alert key="legacy-cluster-props-present" className={styles.alert} variant="info">
                    The configuration contains properties that are valid only in the
                    <Code>values.yaml</Code> config file used for Kubernetes cluster deployments of Sourcegraph:{' '}
                    <Code>{legacyKubernetesConfigProps.join(' ')}</Code>. You can disregard the validation warnings for
                    these properties reported by the configuration editor.
                </Alert>
            )
        }

        if (this.state.enabledCompletions) {
            alerts.push(
                <Alert key="cody-beta-notice" className={styles.alert} variant="info">
                    By turning on completions for "Cody beta," you have read the{' '}
                    <Link to="/help/cody">Cody Documentation</Link> and agree to the{' '}
                    <Link to="https://sourcegraph.com/terms/cody-notice">Cody Notice and Usage Policy</Link>. In
                    particular, some code snippets will be sent to a third-party language model provider when you use
                    Cody questions.
                </Alert>
            )
        }

        const isReloading = typeof this.state.reloadStartedAt === 'number'

        return (
            <div>
                <PageTitle title="Configuration - Admin" />
                <PageHeader
                    path={[{ text: 'Site configuration' }]}
                    headingElement="h2"
                    description={
                        <>
                            View and edit the Sourcegraph site configuration. See{' '}
                            <Link target="_blank" to="/help/admin/config/site_config">
                                documentation
                            </Link>{' '}
                            for more information.
                        </>
                    }
                    className="mb-3"
                />
                <Container className="mb-3">
                    <div>{alerts}</div>
                    {this.state.loading && <LoadingSpinner />}
                    {this.state.site?.configuration && (
                        <div>
                            <DynamicallyImportedMonacoSettingsEditor
                                value={contents || ''}
                                jsonSchema={siteSchemaJSON}
                                canEdit={true}
                                saving={this.state.saving}
                                loading={isReloading || this.state.saving}
                                height={600}
                                isLightTheme={this.props.isLightTheme}
                                onSave={this.onSave}
                                actions={quickConfigureActions}
                                telemetryService={this.props.telemetryService}
                                telemetryRecorder={this.props.telemetryRecorder}
                                explanation={
                                    <Text className="form-text text-muted">
                                        <small>
                                            Use Ctrl+Space for completion, and hover over JSON properties for
                                            documentation. For more information, see the{' '}
                                            <Link to="/help/admin/config/site_config">documentation</Link>.
                                        </small>
                                    </Text>
                                }
                            />
                        </div>
                    )}
                </Container>
                <SiteConfigurationChangeList />
            </div>
        )
    }

    private onSave = async (newContents: string): Promise<string> => {
        eventLogger.log('SiteConfigurationSaved')
        this.props.telemetryRecorder.recordEvent('admin.configuration', 'save')

        this.setState({ saving: true, error: undefined })

        const lastConfiguration = this.state.site?.configuration
        const lastConfigurationID = lastConfiguration?.id || 0

        let restartToApply = false
        try {
            restartToApply = await lastValueFrom(updateSiteConfiguration(lastConfigurationID, newContents))
        } catch (error) {
            logger.error(error)
            this.setState({
                saving: false,
                error: new Error(
                    String(error) +
                        '\nError occured while attempting to save site configuration. Please backup changes before reloading the page.'
                ),
            })
            throw error
        }

        const oldContents = lastConfiguration?.effectiveContents || ''
        const oldConfiguration = jsonc.parse(oldContents) as SiteConfiguration
        const newConfiguration = jsonc.parse(newContents) as SiteConfiguration

        // Flipping these feature flags require a reload for the
        // UI to be rendered correctly in the navbar and the sidebar.
        const keys: (keyof SiteConfiguration)[] = ['batchChanges.enabled', 'codeIntelAutoIndexing.enabled']

        if (!keys.every(key => Boolean(oldConfiguration?.[key]) === Boolean(newConfiguration?.[key]))) {
            window.location.reload()
        }

        this.setState({
            enabledCompletions:
                !oldConfiguration?.completions?.enabled && Boolean(newConfiguration?.completions?.enabled),
        })

        if (restartToApply) {
            window.context.needServerRestart = restartToApply
        } else {
            // Refresh site flags so that global site alerts
            // reflect the latest configuration.
            try {
                await refreshSiteFlags(this.props.client)
            } catch (error) {
                logger.error(error)
            }
        }
        this.setState({ restartToApply })

        try {
            const site = await lastValueFrom(fetchSite())

            this.setState({
                site,
                error: undefined,
                loading: false,
            })

            this.setState({ saving: false })

            return site.configuration.effectiveContents
        } catch (error) {
            this.setState({ error, loading: false })
            throw error
        }
    }

    private reloadSite = (): void => {
        eventLogger.log('SiteReloaded')
        this.props.telemetryRecorder.recordEvent('admin.configuration.site', 'reload')
        this.siteReloads.next()
    }
}
