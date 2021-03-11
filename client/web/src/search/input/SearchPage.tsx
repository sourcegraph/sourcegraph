import * as H from 'history'
import React, { useCallback, useEffect, useMemo } from 'react'
import {
    PatternTypeProps,
    CaseSensitivityProps,
    CopyQueryButtonProps,
    RepogroupHomepageProps,
    OnboardingTourProps,
    HomePanelsProps,
    ShowQueryBuilderProps,
    ParsedSearchQueryProps,
    SearchContextProps,
} from '..'
import { ActivationProps } from '../../../../shared/src/components/activation/Activation'
import { SettingsCascadeProps } from '../../../../shared/src/settings/settings'
import { Settings } from '../../schema/settings.schema'
import { ThemeProps } from '../../../../shared/src/theme'
import { ThemePreferenceProps } from '../../theme'
import { ExtensionsControllerProps } from '../../../../shared/src/extensions/controller'
import { PlatformContextProps } from '../../../../shared/src/platform/context'
import { Link } from '../../../../shared/src/components/Link'
import { BrandLogo } from '../../components/branding/BrandLogo'
import { VersionContextProps } from '../../../../shared/src/search/util'
import { VersionContext } from '../../schema/site.schema'
import { ViewGrid } from '../../repo/tree/ViewGrid'
import { useObservable } from '../../../../shared/src/util/useObservable'
import { isErrorLike } from '../../../../shared/src/util/errors'
import { ContributableViewContainer } from '../../../../shared/src/api/protocol'
import { EMPTY } from 'rxjs'
import classNames from 'classnames'
import { repogroupList, homepageLanguageList } from '../../repogroups/HomepageConfig'
import { SearchPageInput } from './SearchPageInput'
import { KeyboardShortcutsProps } from '../../keyboardShortcuts/keyboardShortcuts'
import { PrivateCodeCta } from './PrivateCodeCta'
import { AuthenticatedUser } from '../../auth'
import { TelemetryProps } from '../../../../shared/src/telemetry/telemetryService'
import { HomePanels } from '../panels/HomePanels'
import { SearchPageFooter } from './SearchPageFooter'
import { SyntaxHighlightedSearchQuery } from '../../components/SyntaxHighlightedSearchQuery'
import { getCombinedViews } from '../../insights/backend'

export interface SearchPageProps
    extends SettingsCascadeProps<Settings>,
        ThemeProps,
        ThemePreferenceProps,
        ActivationProps,
        Pick<ParsedSearchQueryProps, 'parsedSearchQuery'>,
        PatternTypeProps,
        CaseSensitivityProps,
        KeyboardShortcutsProps,
        TelemetryProps,
        ExtensionsControllerProps<'executeCommand' | 'services'>,
        PlatformContextProps<'forceUpdateTooltip' | 'settings' | 'sourcegraphURL'>,
        CopyQueryButtonProps,
        VersionContextProps,
        SearchContextProps,
        RepogroupHomepageProps,
        OnboardingTourProps,
        HomePanelsProps,
        ShowQueryBuilderProps {
    authenticatedUser: AuthenticatedUser | null
    location: H.Location
    history: H.History
    isSourcegraphDotCom: boolean
    setVersionContext: (versionContext: string | undefined) => void
    availableVersionContexts: VersionContext[] | undefined
    autoFocus?: boolean

    // Whether globbing is enabled for filters.
    globbing: boolean

    // Whether to additionally highlight or provide hovers for tokens, e.g., regexp character sets.
    enableSmartQuery: boolean
}

/**
 * The search page
 */
export const SearchPage: React.FunctionComponent<SearchPageProps> = props => {
    const SearchExampleClicked = useCallback(
        (url: string) => (): void => props.telemetryService.log('ExampleSearchClicked', { url }),
        [props.telemetryService]
    )
    const LanguageExampleClicked = useCallback(
        (language: string) => (): void => props.telemetryService.log('ExampleLanguageSearchClicked', { language }),
        [props.telemetryService]
    )

    useEffect(() => props.telemetryService.logViewEvent('Home'), [props.telemetryService])

    const showCodeInsights =
        !isErrorLike(props.settingsCascade.final) &&
        !!props.settingsCascade.final?.experimentalFeatures?.codeInsights &&
        props.settingsCascade.final['insights.displayLocation.homepage'] !== false

    const views = useObservable(
        useMemo(
            () =>
                showCodeInsights
                    ? getCombinedViews(
                          ContributableViewContainer.Homepage,
                          {},
                          props.extensionsController.services.view
                      )
                    : EMPTY,
            [showCodeInsights, props.extensionsController.services.view]
        )
    )
    return (
        <div className="web-content search-page d-flex flex-column align-items-center pb-5">
            <BrandLogo className="search-page__logo" isLightTheme={props.isLightTheme} variant="logo" />
            {props.isSourcegraphDotCom && <div className="text-muted mt-3">Search public code</div>}
            <div
                className={classNames('search-page__search-container', {
                    'search-page__search-container--with-content-below':
                        props.isSourcegraphDotCom || props.showEnterpriseHomePanels,
                })}
            >
                <SearchPageInput {...props} source="home" />
                {views && <ViewGrid {...props} className="mt-5" views={views} />}
            </div>
            {props.isSourcegraphDotCom &&
                props.showRepogroupHomepage &&
                (!props.authenticatedUser || !props.showEnterpriseHomePanels) && (
                    <>
                        <div className="search-page__repogroup-content container-fluid mt-5">
                            <div className="d-flex align-items-baseline mb-3">
                                <h3 className="search-page__help-content-header mr-2">Search in repository groups</h3>
                                <small className="text-monospace font-weight-normal small">
                                    <span className="search-filter-keyword">repogroup:</span>
                                    <i>name</i>
                                </small>
                            </div>
                            <div className="search-page__repogroup-list-cards">
                                {repogroupList.map(repogroup => (
                                    <div className="d-flex" key={repogroup.name}>
                                        <img
                                            className="search-page__repogroup-list-icon mr-2"
                                            src={repogroup.homepageIcon}
                                            alt={`${repogroup.name} icon`}
                                        />
                                        <div className="d-flex flex-column">
                                            <Link
                                                to={repogroup.url}
                                                className="search-page__repogroup-listing-title font-weight-bold"
                                            >
                                                {repogroup.title}
                                            </Link>
                                            <p>{repogroup.homepageDescription}</p>
                                        </div>
                                    </div>
                                ))}
                            </div>
                            <div className="search-page__help-content row mt-5">
                                <div className="col-xs-12 col-lg-5 col-xl-6">
                                    <h3 className="search-page__help-content-header">Example searches</h3>
                                    <ul className="list-group-flush p-0 mt-2">
                                        <li className="list-group-item px-0 pt-3 pb-2">
                                            <Link
                                                to="/search?q=lang:javascript+alert%28:%5Bvariable%5D%29&patternType=structural"
                                                className="search-query-link text-monospace mb-2"
                                                onClick={SearchExampleClicked(
                                                    '/search?q=lang:javascript+alert%28:%5Bvariable%5D%29&patternType=structural'
                                                )}
                                            >
                                                <SyntaxHighlightedSearchQuery query="lang:javascript alert(:[variable])" />
                                            </Link>
                                            <p className="mt-2">
                                                Find usages of the alert() method that displays an alert box.
                                            </p>
                                        </li>
                                        <li className="list-group-item px-0 pt-3 pb-2">
                                            <Link
                                                to="/search?q=repogroup:python+from+%5CB%5C.%5Cw%2B+import+%5Cw%2B&patternType=regexp"
                                                className="search-query-link text-monospace mb-2"
                                                onClick={SearchExampleClicked(
                                                    '/search?q=repogroup:python+from+%5CB%5C.%5Cw%2B+import+%5Cw%2B&patternType=regexp'
                                                )}
                                            >
                                                <SyntaxHighlightedSearchQuery query="repogroup:python from \B\.\w+ import \w+" />
                                            </Link>
                                            <p className="mt-2">
                                                Search for explicit imports with one or more leading dots that indicate
                                                current and parent packages involved, across popular Python
                                                repositories.
                                            </p>
                                        </li>
                                        <li className="list-group-item px-0 pt-3 pb-2">
                                            <Link
                                                to='/search?q=repo:%5Egithub%5C.com/golang/go%24+type:diff+after:"1+week+ago"&patternType=literal"'
                                                className="search-query-link text-monospace mb-2"
                                                onClick={SearchExampleClicked(
                                                    '/search?q=repo:%5Egithub%5C.com/golang/go%24+type:diff+after:"1+week+ago"&patternType=literal"'
                                                )}
                                            >
                                                <SyntaxHighlightedSearchQuery query='repo:^github\.com/golang/go$ type:diff after:"1 week ago"' />
                                            </Link>
                                            <p className="mt-2">
                                                Browse diffs for recent code changes in the 'golang/go' GitHub
                                                repository.
                                            </p>
                                        </li>
                                        <li className="list-group-item px-0 pt-3 pb-2">
                                            <Link
                                                to='/search?q=file:pod.yaml+content:"kind:+ReplicationController"&patternType=literal'
                                                className="search-query-link text-monospace mb-2"
                                                onClick={SearchExampleClicked(
                                                    '/search?q=repo:%5Egithub%5C.com/golang/go%24+type:diff+after:"1+week+ago"&patternType=literal"'
                                                )}
                                            >
                                                <SyntaxHighlightedSearchQuery query='file:pod.yaml content:"kind: ReplicationController"' />
                                            </Link>
                                            <p className="mt-2">
                                                Use a ReplicationController configuration to ensure specified number of
                                                pod replicas are running at any one time.
                                            </p>
                                        </li>
                                    </ul>
                                </div>
                                <div className="search-page__search-a-language col-xs-12 col-md-6 col-lg-3 col-xl-2">
                                    <div className="align-items-baseline mb-4">
                                        <h3 className="search-page__help-content-header">
                                            Search a language{' '}
                                            <small className="text-monospace font-weight-normal">
                                                <span className="search-filter-keyword ml-1">lang:</span>
                                                <i>name</i>
                                            </small>
                                        </h3>
                                    </div>
                                    <div className="d-flex row-cols-2 mt-2">
                                        <div className="d-flex flex-column col mr-auto">
                                            {homepageLanguageList
                                                .slice(0, Math.ceil(homepageLanguageList.length / 2))
                                                .map(language => (
                                                    <Link
                                                        className="search-filter-keyword search-page__lang-link text-monospace mb-3"
                                                        to={`/search?q=lang:${language.filterName}`}
                                                        key={language.name}
                                                    >
                                                        {language.name}
                                                    </Link>
                                                ))}
                                        </div>
                                        <div className="d-flex flex-column col">
                                            {homepageLanguageList
                                                .slice(
                                                    Math.ceil(homepageLanguageList.length / 2),
                                                    homepageLanguageList.length
                                                )
                                                .map(language => (
                                                    <Link
                                                        className="search-filter-keyword search-page__lang-link text-monospace mb-3"
                                                        to={`/search?q=lang:${language.filterName}`}
                                                        key={language.name}
                                                        onClick={LanguageExampleClicked(language.filterName)}
                                                    >
                                                        {language.name}
                                                    </Link>
                                                ))}
                                        </div>
                                    </div>
                                </div>
                                <div className="search-page__search-syntax col-xs-12 col-md-6  col-lg-4">
                                    <h3 className="search-page__help-content-header">Search syntax</h3>
                                    <div className="mt-3 row">
                                        <dl className="col-xs-12 col-lg-6 mb-4">
                                            <dt className="search-page__help-content-subheading">
                                                <h5>Common search keywords</h5>
                                            </dt>
                                            <dd className="text-monospace">
                                                <p>repo:my/repo</p>
                                            </dd>
                                            <dd className="text-monospace">
                                                <p>repo:github.com/myorg/</p>
                                            </dd>
                                            <dd className="text-monospace">
                                                <p>file:my/file</p>
                                            </dd>
                                            <dd className="text-monospace">
                                                <p>lang:javascript</p>
                                            </dd>
                                            <dt className="search-page__help-content-subheading mt-5">
                                                <h5>Diff/commit search keywords</h5>
                                            </dt>
                                            <dd className="text-monospace">
                                                <p>type:diff or type:commit</p>
                                            </dd>
                                            <dd className="text-monospace">
                                                <p>after:"2 weeks ago"</p>
                                            </dd>
                                            <dd className="text-monospace">
                                                <p>author:alice@example.com</p>
                                            </dd>{' '}
                                            <dd className="text-monospace">
                                                <p>repo:r@*refs/heads/ (all branches)</p>
                                            </dd>
                                        </dl>
                                        <dl className="col-xs-12 col-xl-6">
                                            <dt className="search-page__help-content-subheading">
                                                <h5>Finding matches</h5>
                                            </dt>
                                            <dd>
                                                <p>
                                                    <strong>Regexp:</strong>{' '}
                                                    <span className="text-monospace">(read|write)File</span>
                                                </p>
                                            </dd>{' '}
                                            <dd>
                                                <p>
                                                    <strong>Exact:</strong>{' '}
                                                    <span className="text-monospace">"fs.open(f)"</span>
                                                </p>
                                            </dd>
                                            <dd>
                                                <p>
                                                    <strong>Structural:</strong>{' '}
                                                    <span className="text-monospace">if(:[my_match])</span>
                                                </p>
                                            </dd>
                                        </dl>
                                    </div>
                                </div>
                            </div>
                            <div className="row justify-content-center">
                                <div className="mx-auto col-sm-12 col-md-8 col-lg-8 col-xl-6">
                                    <PrivateCodeCta />
                                </div>
                            </div>
                        </div>
                    </>
                )}

            {props.showEnterpriseHomePanels && props.authenticatedUser && <HomePanels {...props} />}

            <SearchPageFooter className="search-page__footer" />
        </div>
    )
}
