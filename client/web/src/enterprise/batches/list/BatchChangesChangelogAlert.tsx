import React from 'react'

import classNames from 'classnames'

import { CardBody, Card, H3, H4, Link } from '@sourcegraph/wildcard'

import { DismissibleAlert } from '../../../components/DismissibleAlert'

import styles from './BatchChangesListIntro.module.scss'

/**
 * CURRENT_VERSION is meant to be updated by release tooling to the current in-progress version.
 * Ie. After 5.0 is cut, this should be bumped to 5.1, and so on. This ensures we
 * always render the right changelog.
 */
const CURRENT_VERSION = '5.1'
/**
 * SHOW_CHANGELOG has to be set to true when a changelog entry is added for a release.
 * After every release, this will be set back to `false`. Chromatic will also verify
 * changes to this variable via visual regression testing.
 */
const SHOW_CHANGELOG = true

interface BatchChangesChangelogAlertProps {
    className?: string
    viewerIsAdmin: boolean
}

export const BatchChangesChangelogAlert: React.FunctionComponent<
    React.PropsWithChildren<BatchChangesChangelogAlertProps>
> = ({ className, viewerIsAdmin }) => {
    // IMPORTANT!! If you add an entry, make sure to set SHOW_CHANGELOG to true!
    if (!SHOW_CHANGELOG) {
        return null
    }
    return (
        <DismissibleAlert
            className={classNames(styles.batchChangesListIntroAlert, className)}
            partialStorageKey={`batch-changes-list-intro-changelog-${CURRENT_VERSION}`}
        >
            <Card className={classNames(styles.batchChangesListIntroCard, 'h-100')}>
                <CardBody>
                    <H4 as={H3}>Batch Changes updates in version {CURRENT_VERSION}</H4>
                    <ul className="mb-0 pl-3">
                        <li>
                            Batch Changes can now sign commits created on GitHub{' '}
                            <Link
                                to="/help/admin/executors/deploy_executors#using-private-registries"
                                rel="noopener"
                                target="_blank"
                            >
                                via GitHub Apps
                            </Link>
                            .{' '}
                            {viewerIsAdmin ? (
                                <>
                                    {' '}
                                    Site admins can{' '}
                                    <Link to="/site-admin/batch-changes" target="_blank">
                                        configure a GitHub App integration
                                    </Link>{' '}
                                    to enable this feature.
                                </>
                            ) : (
                                <>
                                    GitHub App integrations can be configured by site admins and viewed from{' '}
                                    <Link to="/user/settings/batch-changes" target="_blank">
                                        your user settings
                                    </Link>
                                    .
                                </>
                            )}
                        </li>
                    </ul>
                </CardBody>
            </Card>
        </DismissibleAlert>
    )
}
