import React, { type VideoHTMLAttributes } from 'react'

import { mdiOpenInNew, mdiGithub, mdiCheck, mdiGitlab, mdiBitbucket, mdiLock, mdiBookOpenPageVariant } from '@mdi/js'
import classNames from 'classnames'

import { SourcegraphLogo } from '@sourcegraph/branded/src/components/SourcegraphLogo'
import { PhabricatorIcon } from '@sourcegraph/shared/src/components/icons'
import { Link, Icon, Code, H1, H2, H3, Text } from '@sourcegraph/wildcard'

import { getPlatformName } from '../../shared/util/context'

import styles from './AfterInstallPageContent.module.scss'

interface VideoProps extends Pick<VideoHTMLAttributes<HTMLVideoElement>, 'width' | 'height'> {
    name: string
    isLightTheme: boolean
}

const Video: React.FC<VideoProps> = ({ name, isLightTheme, width, height }) => {
    const suffix = isLightTheme ? 'Light' : 'Dark'
    return (
        <video
            className="w-100 h-auto cursor-pointer"
            width={width}
            height={height}
            autoPlay={true}
            loop={true}
            muted={true}
            playsInline={true}
            onClick={event => event.currentTarget.requestFullscreen()}
            // Add a key on the theme to force React to render a new <video> element when the theme changes
            key={name + suffix}
        >
            <source
                src={`https://storage.googleapis.com/sourcegraph-assets/code-host-integration/${name}${suffix}.webm`}
                type="video/webm"
            />
            <source
                src={`https://storage.googleapis.com/sourcegraph-assets/code-host-integration/${name}${suffix}.mp4`}
                type="video/mp4"
            />
        </video>
    )
}

interface AfterInstallPageContentProps {
    isLightTheme: boolean
}

export const AfterInstallPageContent: React.FC<AfterInstallPageContentProps> = props => {
    // Safari does not support the search shortcut. So don't show the feature.
    const isSafari = getPlatformName() === 'safari-extension'
    const showSearchShortcut = !isSafari

    return (
        <div className="after-install-page-content" data-testid="after-install-page-content">
            <div className="d-flex w-100 p-3 justify-content-between align-items-center">
                <Link to="https://sourcegraph.com/search" target="_blank" rel="noopener">
                    <SourcegraphLogo className={styles.sourcegraphLogo} />
                </Link>
                <Link to="https://sourcegraph.com/docs/integration/browser_extension" target="_blank" rel="noopener">
                    Browser extension docs <Icon aria-hidden={true} svgPath={mdiOpenInNew} />
                </Link>
            </div>

            <div className="container mt-3">
                <H1>🎉 You’ve just installed the Sourcegraph browser extension!</H1>
                <Text className="lead mb-0">
                    We’ve gathered the most important information that will get your started:
                </Text>
            </div>

            <section className="border-bottom py-5">
                <div className="container">
                    <H2 className="mb-4">How do I use the extension?</H2>
                    <div className="row">
                        <div className="col-md-6">
                            <H3>Code navigation on your code host</H3>
                            <Text>
                                Sourcegraph browser extension adds code navigation to files and diffs on GitHub, GitHub
                                Enterprise, GitLab, Phabricator, Bitbucket Server, and Gerrit.
                            </Text>
                            <Video {...props} name="CodeIntelligenceOnCodeHost" width={1760} height={1060} />
                        </div>
                        {showSearchShortcut && (
                            <div className="col-md-6 mt-4 mt-md-0">
                                <H3>Search shortcut in the URL location bar</H3>
                                <Text>
                                    Type <Code>src</Code>
                                    <kbd>tab</kbd> in the address bar of your browser to search for queries on
                                    Sourcegraph.
                                </Text>
                                <Video {...props} name="BrowserShortcut" width={1196} height={720} />
                            </div>
                        )}
                    </div>
                </div>
            </section>

            <section className="border-bottom py-5">
                <div className="container">
                    <div className="row">
                        <div className="col-md-6 d-flex flex-column">
                            <H2 className="mb-4">Make it work on your codehost</H2>
                            <div className="bg-2 rounded p-3 mb-3 d-flex flex-column justify-content-center">
                                <H3 className={classNames('mb-3', styles.codeHostTitles)}>
                                    <Icon className={styles.codeHostLogo} aria-hidden={true} svgPath={mdiGithub} />{' '}
                                    github.com
                                </H3>
                                <Text className="m-0">
                                    <Icon aria-hidden={true} svgPath={mdiCheck} /> No action required. Your extension
                                    works here by default.
                                </Text>
                            </div>
                            <div className="bg-2 rounded p-3 d-flex flex-column justify-content-center">
                                <H3 className={classNames('d-flex flex-wrap', styles.codeHostTitles)}>
                                    <div className="mr-5 mb-3">
                                        <Icon className={styles.codeHostLogo} aria-hidden={true} svgPath={mdiGithub} />{' '}
                                        GitHub Enterprise
                                    </div>
                                    <div className="mr-5 mb-3">
                                        <Icon className={styles.codeHostLogo} aria-hidden={true} svgPath={mdiGitlab} />{' '}
                                        GitLab
                                    </div>
                                    <div className="mr-5 mb-3">
                                        <Icon
                                            className={styles.codeHostLogo}
                                            aria-hidden={true}
                                            svgPath={mdiBitbucket}
                                        />{' '}
                                        Bitbucket Server
                                    </div>
                                    <div className="mr-5 mb-3">
                                        <Icon className={styles.codeHostLogo} as={PhabricatorIcon} aria-hidden={true} />{' '}
                                        Phabricator
                                    </div>
                                </H3>
                                <Text>Your extension needs explicit permissions to your code host:</Text>
                                <ol className="m-0">
                                    <li>Navigate to any page on your code host.</li>
                                    <li>
                                        Click the{' '}
                                        <q>
                                            <strong>Grant permissions</strong>
                                        </q>{' '}
                                        button.
                                    </li>
                                    <li>
                                        Click{' '}
                                        <q>
                                            <strong>Allow</strong>
                                        </q>{' '}
                                        in the permissions request popup.
                                    </li>
                                </ol>
                            </div>
                        </div>
                        <div className="col-md-6 mt-4 mt-md-0">
                            <Video {...props} name="GrantPermissions" width={1762} height={1384} />
                        </div>
                    </div>
                </div>
            </section>

            <section className="border-bottom py-5">
                <div className="container">
                    <div className="row">
                        <div className="col-md-6 d-flex flex-column">
                            <H2 className="mb-4">Make it work for private code</H2>
                            <Text>By default, the browser extension works only for public code.</Text>
                            <div className="d-flex align-items-center">
                                <div className="bg-3 rounded-circle p-2">
                                    <Icon aria-hidden={true} svgPath={mdiLock} />
                                </div>
                                <Text className="m-0 ml-3">
                                    To use the browser extension with your private repositories, you need to set up a{' '}
                                    <strong>private Sourcegraph instance</strong> and connect the extension to it.
                                </Text>
                            </div>
                            <div className="bg-2 rounded p-3 mt-4 d-flex flex-column justify-content-around">
                                <Text>Follow these instructions:</Text>
                                <ol className="m-0 d-flex flex-column justify-content-around">
                                    <li>
                                        <strong>Install Sourcegraph</strong> (
                                        <Link
                                            to="https://sourcegraph.com/docs/admin/install"
                                            target="_blank"
                                            rel="noopener"
                                        >
                                            visit our docs for instructions
                                        </Link>
                                        ).Skip this step if you already have a private Sourcegraph instance.
                                    </li>
                                    <li>
                                        Click the Sourcegraph extension icon in the browser toolbar to{' '}
                                        <Link to="./options.html" rel="noopener" target="_blank">
                                            open the settings page
                                        </Link>
                                        .
                                    </li>
                                    <li>
                                        Enter the <strong>URL</strong> (including the protocol) of your Sourcegraph
                                        instance (such as <q>https://sourcegraph.example.com</q>).
                                    </li>
                                    <li>Make sure a green checkmark appears in the input field.</li>
                                </ol>
                            </div>
                        </div>
                        <div className="col-md-6 mt-4 mt-md-0">
                            <Video {...props} name="PrivateInstance" width={1764} height={1390} />
                        </div>
                    </div>
                </div>
            </section>

            <section className="py-5">
                <div className="container">
                    <H2 className="mb-4">Additional resources</H2>
                    <div className="d-flex w-100 align-items-center">
                        <div className="bg-3 rounded-circle p-2">
                            <Icon aria-hidden={true} svgPath={mdiBookOpenPageVariant} />
                        </div>
                        <Text className="m-0 ml-3">
                            Read the{' '}
                            <Link
                                to="https://sourcegraph.com/docs/integration/browser_extension"
                                rel="noopener"
                                target="_blank"
                            >
                                Sourcegraph docs
                            </Link>{' '}
                            to learn more about how we respect your privacy, troubleshooting and extension features.
                        </Text>
                    </div>
                </div>
            </section>
        </div>
    )
}
