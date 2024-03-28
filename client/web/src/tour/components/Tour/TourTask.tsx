import React, { useCallback, useContext, useMemo } from 'react'
import type { FC } from 'react'

import { mdiCheckCircle, mdiMagnify, mdiPuzzleOutline } from '@mdi/js'
import classNames from 'classnames'
import { CircularProgressbar } from 'react-circular-progressbar'

import { ModalVideo } from '@sourcegraph/branded'
import { AskCodyIcon } from '@sourcegraph/cody-ui'
import { TourIcon, type TourTaskStepType, type TourTaskType } from '@sourcegraph/shared/src/settings/temporary'
import { Button, Icon, Link, Text } from '@sourcegraph/wildcard'

import { TourContext } from './context'
import { SearchTask } from './SearchTask'
import { TourNewTabLink } from './TourNewTabLink'
import { buildURIMarkers } from './utils'

import styles from './Tour.module.scss'

type TourTaskProps = TourTaskType & {
    variant?: 'small'
}

/**
 * Tour task smart component. Handles all TourTaskStepType.type options.
 */
export const TourTask: React.FunctionComponent<React.PropsWithChildren<TourTaskProps>> = ({
    title,
    steps,
    completed,
    icon,
    variant,
    dataAttributes = {},
}) => {
    const { onStepClick, onRestart } = useContext(TourContext)

    const handleLinkClick = useCallback(
        (step: TourTaskStepType) => {
            onStepClick(step)
        },
        [onStepClick]
    )

    const handleVideoToggle = useCallback(
        (isOpen: boolean, step: TourTaskStepType) => {
            if (!isOpen) {
                onStepClick(step)
            }
        },
        [onStepClick]
    )

    const attributes = useMemo(
        () =>
            Object.entries(dataAttributes).reduce(
                (result, [key, value]) => ({ ...result, [`data-${key}`]: value }),
                {}
            ),
        [dataAttributes]
    )

    const isMultiStep = steps.length > 1
    return (
        <div
            className={classNames(
                icon && [styles.task, variant === 'small' && styles.isSmall],
                !title && styles.noTitleTask
            )}
            {...attributes}
        >
            {variant !== 'small' && icon && <TaskIcon icon={icon} />}
            <div className={classNames('flex-grow-1', variant !== 'small' && 'h-100 d-flex flex-column')}>
                {title && (
                    <div className="d-flex justify-content-between position-relative">
                        {variant === 'small' && icon && <TaskIcon icon={icon} />}
                        <Text className={styles.title}>{title}</Text>
                        {completed === 100 && (
                            <Icon size="sm" className="text-success" aria-label="Completed" svgPath={mdiCheckCircle} />
                        )}
                        {typeof completed === 'number' && completed < 100 && (
                            <CircularProgressbar
                                className={styles.progressBar}
                                strokeWidth={10}
                                value={completed || 0}
                            />
                        )}
                    </div>
                )}
                <ul
                    className={classNames(
                        styles.stepList,
                        'm-0',
                        variant !== 'small' && 'flex-grow-1 d-flex flex-column',
                        isMultiStep && styles.isMultiStep
                    )}
                >
                    {steps.map(step => (
                        <li key={step.id} className={classNames(styles.stepListItem, 'd-flex align-items-center')}>
                            {step.action.type === 'search-query' && (
                                <SearchTask
                                    label={step.label}
                                    template={step.action.query}
                                    snippets={step.action.snippets}
                                    handleLinkClick={() => handleLinkClick(step)}
                                />
                            )}
                            {step.action.type === 'link' && (
                                <Link
                                    className="flex-grow-1"
                                    to={buildURIMarkers(step.action.value, step.id)}
                                    onClick={() => handleLinkClick(step)}
                                >
                                    {step.label}
                                </Link>
                            )}
                            {step.action.type === 'new-tab-link' && (
                                <TourNewTabLink
                                    step={step}
                                    variant={step.action.variant === 'button-primary' ? 'button' : 'link'}
                                    className={classNames('flex-grow-1')}
                                    to={buildURIMarkers(step.action.value, step.id)}
                                    onClick={handleLinkClick}
                                />
                            )}
                            {step.action.type === 'restart' && (
                                <div className="flex-grow">
                                    <Text className="m-0">{step.label}</Text>
                                    <div className="d-flex flex-column">
                                        <Button
                                            variant="link"
                                            className="align-self-start text-left pl-0 font-weight-normal"
                                            onClick={() => onRestart(step)}
                                        >
                                            {step.action.value}
                                        </Button>
                                    </div>
                                </div>
                            )}
                            {step.action.type === 'video' && (
                                <ModalVideo
                                    id={step.id}
                                    showCaption={true}
                                    title={step.label}
                                    className="flex-grow-1"
                                    titleClassName="shadow-none text-left p-0 m-0"
                                    src={buildURIMarkers(step.action.value, step.id)}
                                    onToggle={isOpen => handleVideoToggle(isOpen, step)}
                                />
                            )}
                            {(isMultiStep || !title) && step.isCompleted && (
                                <Icon
                                    size="md"
                                    className="text-success"
                                    aria-label="Completed step"
                                    svgPath={mdiCheckCircle}
                                />
                            )}
                        </li>
                    ))}
                </ul>
            </div>
        </div>
    )
}

const TaskIcon: FC<{ icon: TourIcon }> = ({ icon }) => {
    if (icon === TourIcon.Cody) {
        return (
            <span className={styles.taskIcon}>
                <AskCodyIcon />
            </span>
        )
    }

    let svgPath: string
    let className = ''

    switch (icon) {
        case TourIcon.Search: {
            svgPath = mdiMagnify
            break
        }
        case TourIcon.Extension: {
            svgPath = mdiPuzzleOutline
            break
        }
        case TourIcon.Check: {
            className = 'text-success'
            svgPath = mdiCheckCircle
            break
        }
    }

    return (
        <span className={styles.taskIcon}>
            <Icon
                className={className}
                svgPath={svgPath}
                inline={false}
                aria-hidden={true}
                height="2.3rem"
                width="2.3rem"
            />
        </span>
    )
}
