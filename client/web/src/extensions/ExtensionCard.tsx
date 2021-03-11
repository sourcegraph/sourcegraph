import React, { useState, useCallback, useMemo, memo } from 'react'
import WarningIcon from 'mdi-react/WarningIcon'
import classNames from 'classnames'
import { ConfiguredRegistryExtension, splitExtensionID } from '../../../shared/src/extensions/extension'
import * as GQL from '../../../shared/src/graphql/schema'
import { PlatformContextProps } from '../../../shared/src/platform/context'
import { ExtensionManifest } from '../../../shared/src/schema/extensionSchema'
import { SettingsCascadeProps } from '../../../shared/src/settings/settings'
import { isErrorLike } from '../../../shared/src/util/errors'
import { isExtensionAdded } from './extension/extension'
import { ExtensionConfigurationState } from './extension/ExtensionConfigurationState'
import { ExtensionToggle, OptimisticUpdateFailure } from './ExtensionToggle'
import { isEncodedImage } from '../../../shared/src/util/icon'
import { Link } from 'react-router-dom'
import { DefaultIconEnabled, DefaultIcon, SourcegraphExtensionIcon } from './icons'
import { ThemeProps } from '../../../shared/src/theme'
import { useTimeoutManager } from '../../../shared/src/util/useTimeoutManager'
import { ExtensionStatusBadge } from './extension/ExtensionStatusBadge'

interface Props extends SettingsCascadeProps, PlatformContextProps<'updateSettings'>, ThemeProps {
    node: Pick<
        ConfiguredRegistryExtension<
            Pick<
                GQL.IRegistryExtension,
                'id' | 'extensionIDWithoutRegistry' | 'isWorkInProgress' | 'viewerCanAdminister' | 'url'
            >
        >,
        'id' | 'manifest' | 'registryExtension'
    >
    subject: Pick<GQL.SettingsSubject, 'id' | 'viewerCanAdminister'>
    enabled: boolean
    settingsURL: string | null | undefined
}

const stopPropagation: React.MouseEventHandler<HTMLElement> = event => {
    event.stopPropagation()
}

/** ms after which to remove visual feedback */
const FEEDBACK_DELAY = 5000

/** Displays an extension as a card. */
export const ExtensionCard = memo<Props>(function ExtensionCard({
    node: extension,
    settingsCascade,
    platformContext,
    subject,
    enabled,
    settingsURL,
    isLightTheme,
}) {
    const manifest: ExtensionManifest | undefined =
        extension.manifest && !isErrorLike(extension.manifest) ? extension.manifest : undefined

    const icon = React.useMemo(() => {
        let url: string | undefined

        if (isLightTheme) {
            if (manifest?.icon && isEncodedImage(manifest.icon)) {
                url = manifest.icon
            }
        } else if (manifest?.iconDark && isEncodedImage(manifest.iconDark)) {
            url = manifest.iconDark
        } else if (manifest?.icon && isEncodedImage(manifest.icon)) {
            // fallback: show default icon on dark theme if dark icon isn't specified
            url = manifest.icon
        }

        return url
    }, [manifest?.icon, manifest?.iconDark, isLightTheme])

    const { name, publisher, isSourcegraphExtension } = useMemo(
        () =>
            splitExtensionID(
                extension.registryExtension ? extension.registryExtension.extensionIDWithoutRegistry : extension.id
            ),
        [extension]
    )

    const actionableErrorMessage = (error: Error): JSX.Element => {
        let errorMessage

        if (error.message.startsWith('invalid settings') && settingsURL) {
            errorMessage = (
                <>
                    Could not enable / disable {name}. Edit your <Link to={settingsURL}>user settings</Link> to fix this
                    error. <br />
                    <br /> ({error.message})
                </>
            )
        } else {
            errorMessage = <>{error.message}</>
        }

        return errorMessage
    }

    /**
     * When extension enablement state changes, display visual feedback for $delay seconds.
     * Clear the timeout when the component unmounts or the extension is toggled again.
     */
    const [change, setChange] = useState<'enabled' | 'disabled' | null>(null)
    const feedbackManager = useTimeoutManager()

    // Add class that triggers box shadow animation .3s after enabled, and remove it 1s later
    const [showShadow, setShowShadow] = React.useState(false)
    const startAnimationManager = useTimeoutManager()
    const endAnimationManager = useTimeoutManager()

    const [optimisticFailure, setOptimisticFailure] = useState<OptimisticUpdateFailure<boolean> | null>(null)
    const optimisticFailureManager = useTimeoutManager()

    const onToggleChange = useCallback(
        (enabled: boolean): void => {
            if (enabled) {
                setChange('enabled')
                // not using transition-delay so the shadow will immediately disappear on disable
                startAnimationManager.setTimeout(() => {
                    setShowShadow(true)
                }, 300)
                endAnimationManager.setTimeout(() => {
                    setShowShadow(false)
                }, 1000)
            } else {
                setChange('disabled')
                setShowShadow(false)
                startAnimationManager.cancelTimeout()
                endAnimationManager.cancelTimeout()
            }
            // Common: clear possible error, queue timeout to clear change feedback
            setOptimisticFailure(null)
            optimisticFailureManager.cancelTimeout()
            feedbackManager.setTimeout(() => {
                setChange(null)
            }, FEEDBACK_DELAY)
        },
        [feedbackManager, startAnimationManager, endAnimationManager, optimisticFailureManager]
    )

    /**
     * When an optimistic update results in an error, we want to show different
     * feedback and cancel current feedback/animations
     */
    const onToggleError = useCallback(
        (optimisticUpdateFailure: OptimisticUpdateFailure<boolean>) => {
            // Cancel all timeouts
            startAnimationManager.cancelTimeout()
            endAnimationManager.cancelTimeout()
            feedbackManager.cancelTimeout()
            // Revert state
            setChange(null)
            setShowShadow(false)
            // Set error state and timeout to clear
            setOptimisticFailure(optimisticUpdateFailure)
            optimisticFailureManager.setTimeout(() => setOptimisticFailure(null), FEEDBACK_DELAY)
        },
        [startAnimationManager, endAnimationManager, feedbackManager, optimisticFailureManager]
    )

    return (
        <div className="d-flex">
            <div
                className={classNames('extension-card card position-relative flex-1', {
                    'alert alert-success p-0 m-0 extension-card--enabled': change === 'enabled',
                })}
            >
                {/* Visual feedback: shadow when extension is enabled */}
                <div
                    className={classNames('extension-card__shadow rounded', {
                        'extension-card__shadow--show': showShadow,
                    })}
                />
                <div
                    className="card-body extension-card__body d-flex position-relative"
                    // Prevent toggle clicks from propagating to the stretched-link (and
                    // navigating to the extension detail page).
                    onClick={stopPropagation}
                >
                    {/* Item 1: Icon */}
                    <div className="flex-shrink-0 mr-2">
                        {icon ? (
                            <img className="extension-card__icon" src={icon} />
                        ) : isSourcegraphExtension ? (
                            change === 'enabled' ? (
                                <DefaultIconEnabled isLightTheme={isLightTheme} />
                            ) : (
                                <DefaultIcon />
                            )
                        ) : null}
                    </div>
                    {/* Item 2: Text */}
                    <div className="text-truncate w-100">
                        <div className="d-flex align-items-center">
                            <span className="mb-0 mr-1 text-truncate flex-1">
                                <Link
                                    to={`/extensions/${extension.id}`}
                                    className={classNames('font-weight-bold', change === 'enabled' ? 'alert-link' : '')}
                                >
                                    {name}
                                </Link>
                                <span
                                    className={classNames({
                                        'text-muted': change !== 'enabled',
                                    })}
                                >
                                    {' '}
                                    by{' '}
                                    <span
                                        className={classNames({
                                            'font-weight-bold': isSourcegraphExtension,
                                        })}
                                    >
                                        {publisher}
                                    </span>
                                </span>
                                {isSourcegraphExtension && (
                                    <SourcegraphExtensionIcon className="icon-inline extension-card__logo" />
                                )}
                            </span>
                            {extension.registryExtension?.isWorkInProgress && (
                                <ExtensionStatusBadge
                                    viewerCanAdminister={extension.registryExtension.viewerCanAdminister}
                                />
                            )}
                        </div>
                        <div className="mt-1">
                            {extension.manifest ? (
                                isErrorLike(extension.manifest) ? (
                                    <span className="text-danger small" title={extension.manifest.message}>
                                        <WarningIcon className="icon-inline" /> Invalid manifest
                                    </span>
                                ) : (
                                    extension.manifest.description && (
                                        <div className="text-truncate">{extension.manifest.description}</div>
                                    )
                                )
                            ) : (
                                <span className="text-warning small">
                                    <WarningIcon className="icon-inline" /> No manifest
                                </span>
                            )}
                        </div>
                    </div>
                    {/* Item 3: Toggle */}
                    {subject &&
                        (subject.viewerCanAdminister ? (
                            <ExtensionToggle
                                extensionID={extension.id}
                                enabled={enabled}
                                settingsCascade={settingsCascade}
                                platformContext={platformContext}
                                className="extension-card__toggle flex-shrink-0 align-self-start"
                                onToggleChange={onToggleChange}
                                onToggleError={onToggleError}
                            />
                        ) : (
                            <ExtensionConfigurationState
                                isAdded={isExtensionAdded(settingsCascade.final, extension.id)}
                                isEnabled={enabled}
                                enabledIconOnly={true}
                                className="small"
                            />
                        ))}
                </div>

                {/* Visual feedback: alert when extension is disabled */}
                {change === 'disabled' && (
                    <div className="alert alert-secondary px-2 py-1 extension-card__disabled-feedback">
                        <span className="font-weight-semibold">{name}</span> is disabled
                    </div>
                )}
                {/* Visual feedback: alert when optimistic update fails */}
                {optimisticFailure && (
                    <div className="alert alert-danger px-2 py-1 extension-card__disabled-feedback">
                        <span className="font-weight-semibold">Error:</span>{' '}
                        {actionableErrorMessage(optimisticFailure.error)}
                    </div>
                )}
            </div>
        </div>
    )
},
areEqual)

/** Custom compareFunction for ExtensionCard */
function areEqual(oldProps: Props, newProps: Props): boolean {
    return oldProps.enabled === newProps.enabled && oldProps.isLightTheme === newProps.isLightTheme
}
