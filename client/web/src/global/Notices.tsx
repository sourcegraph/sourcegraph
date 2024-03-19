import React from 'react'

import classNames from 'classnames'

import { renderMarkdown } from '@sourcegraph/common'
import type { Notice } from '@sourcegraph/shared/src/schema/settings.schema'
import { useSettings } from '@sourcegraph/shared/src/settings/settings'
import { Alert, type AlertProps, Markdown } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../auth'
import { isEmailVerificationNeededForCody } from '../cody/isCodyEnabled'
import { DismissibleAlert } from '../components/DismissibleAlert'

import styles from './Notices.module.scss'

const getAlertVariant = (location: Notice['location']): AlertProps['variant'] =>
    location === 'top' ? 'info' : undefined

interface NoticeAlertProps {
    notice: Notice
    className?: string
    testId?: string
}

const NoticeAlert: React.FunctionComponent<React.PropsWithChildren<NoticeAlertProps>> = ({
    notice,
    className = '',
    testId,
}) => {
    const content = <Markdown dangerousInnerHTML={renderMarkdown(notice.message)} />

    const sharedProps = {
        'data-testid': testId,
        variant: getAlertVariant(notice.location),
        className: classNames(notice.location !== 'top' && 'bg transparent border p-2', className),
    }

    return notice.dismissible ? (
        <DismissibleAlert {...sharedProps} partialStorageKey={`notice.${notice.message}`}>
            {content}
        </DismissibleAlert>
    ) : (
        <Alert {...sharedProps}>{content}</Alert>
    )
}

interface Props {
    className?: string

    /** Apply this class name to each notice (alongside .alert). */
    alertClassName?: string

    /** Display notices for this location. */
    location: Notice['location']
}

/**
 * Displays notices from settings for a specific location.
 */
export const Notices: React.FunctionComponent<React.PropsWithChildren<Props>> = ({
    className = '',
    alertClassName,
    location,
}) => {
    const settings = useSettings()

    if (!settings?.notices || !Array.isArray(settings?.notices)) {
        return null
    }

    const notices = settings.notices.filter(notice => notice.location === location)

    if (notices.length === 0) {
        return null
    }

    return (
        <div className={classNames(styles.notices, className)}>
            {notices.map((notice, index) => (
                <NoticeAlert key={index} testId="notice-alert" className={alertClassName} notice={notice} />
            ))}
        </div>
    )
}

interface VerifyEmailNoticesProps {
    className?: string
    alertClassName?: string
    authenticatedUser: AuthenticatedUser | null
}

/**
 * Displays notices from settings for a specific location.
 */
export const VerifyEmailNotices: React.FunctionComponent<VerifyEmailNoticesProps> = ({
    className,
    alertClassName,
    authenticatedUser,
}) => {
    if (isEmailVerificationNeededForCody() && authenticatedUser) {
        return (
            <div className={classNames(styles.notices, className)}>
                <NoticeAlert
                    className={alertClassName}
                    notice={{
                        location: 'top',
                        message: `**NEW**: Cody, our new AI Assistant is available to use for free, simply verify your email address. Didn't get an email? [Resend verification email](${authenticatedUser?.settingsURL}/emails)`,
                        dismissible: true,
                    }}
                />
            </div>
        )
    }

    return null
}
