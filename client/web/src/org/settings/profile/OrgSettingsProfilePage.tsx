import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import React, { useCallback, useEffect, useState } from 'react'
import { ORG_DISPLAY_NAME_MAX_LENGTH } from '../..'
import { Form } from '../../../../../branded/src/components/Form'
import { PageTitle } from '../../../components/PageTitle'
import { eventLogger } from '../../../tracking/eventLogger'
import { OrgAreaPageProps } from '../../area/OrgArea'
import { updateOrganization } from '../../backend'
import { ErrorAlert } from '../../../components/alerts'
import { asError, isErrorLike } from '../../../../../shared/src/util/errors'
import { Timestamp } from '../../../components/time/Timestamp'
import { PageHeader } from '../../../components/PageHeader'

interface Props extends Pick<OrgAreaPageProps, 'org' | 'onOrganizationUpdate'> {}

/**
 * The organization profile settings page.
 */
export const OrgSettingsProfilePage: React.FunctionComponent<Props> = ({ org, onOrganizationUpdate }) => {
    useEffect(() => {
        eventLogger.logViewEvent('OrgSettingsProfile')
    }, [org.id])

    const [displayName, setDisplayName] = useState<string>(org.displayName ?? '')
    const onDisplayNameFieldChange = useCallback<React.ChangeEventHandler<HTMLInputElement>>(event => {
        setDisplayName(event.target.value)
    }, [])
    const [isLoading, setIsLoading] = useState<boolean | Error>(false)
    const [updated, setIsUpdated] = useState<boolean>(false)
    const [updateResetTimer, setUpdateResetTimer] = useState<NodeJS.Timer>()

    useEffect(
        () => () => {
            if (updateResetTimer) {
                clearTimeout(updateResetTimer)
            }
        },
        [updateResetTimer]
    )

    const onSubmit = useCallback<React.FormEventHandler>(
        async event => {
            event.preventDefault()
            setIsLoading(true)
            try {
                await updateOrganization(org.id, displayName)
                onOrganizationUpdate()
                // Reenable submit button, flash "updated" text
                setIsLoading(false)
                setIsUpdated(true)
                setUpdateResetTimer(
                    setTimeout(() => {
                        // Hide "updated" text again after 1s
                        setIsUpdated(false)
                    }, 1000)
                )
            } catch (error) {
                setIsLoading(asError(error))
            }
        },
        [displayName, onOrganizationUpdate, org.id]
    )

    return (
        <div className="org-settings-profile-page">
            <PageTitle title={org.name} />
            <PageHeader
                path={[{ text: 'Organisation profile' }]}
                headingElement="h2"
                className="org-settings-profile-page__heading"
            />
            <p>
                {org.displayName ? (
                    <>
                        {org.displayName} ({org.name})
                    </>
                ) : (
                    org.name
                )}{' '}
                was created <Timestamp date={org.createdAt} />.
            </p>
            <Form className="org-settings-profile-page" onSubmit={onSubmit}>
                <div className="form-group">
                    <label>Display name</label>
                    <input
                        type="text"
                        className="form-control org-settings-profile-page__display-name"
                        placeholder="Organization name"
                        onChange={onDisplayNameFieldChange}
                        value={displayName}
                        spellCheck={false}
                        maxLength={ORG_DISPLAY_NAME_MAX_LENGTH}
                    />
                </div>
                <button
                    type="submit"
                    disabled={isLoading === true}
                    className="btn btn-primary org-settings-profile-page__submit-button"
                >
                    Update
                </button>
                {isLoading === true && <LoadingSpinner className="icon-inline" />}
                <div
                    className={
                        'org-settings-profile-page__updated-text' +
                        (updated ? ' org-settings-profile-page__updated-text--visible' : '')
                    }
                >
                    <small>Updated!</small>
                </div>
                {isErrorLike(isLoading) && <ErrorAlert error={isLoading} />}
            </Form>
        </div>
    )
}
