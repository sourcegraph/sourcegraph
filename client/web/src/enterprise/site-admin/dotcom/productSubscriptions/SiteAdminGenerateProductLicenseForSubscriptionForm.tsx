import addDays from 'date-fns/addDays'
import endOfDay from 'date-fns/endOfDay'
import React, { useState, useCallback } from 'react'
import { Observable } from 'rxjs'
import { catchError, map, startWith, switchMap, tap } from 'rxjs/operators'
import { gql } from '../../../../../../shared/src/graphql/graphql'
import * as GQL from '../../../../../../shared/src/graphql/schema'
import { asError, createAggregateError, isErrorLike } from '../../../../../../shared/src/util/errors'
import { mutateGraphQL } from '../../../../backend/graphql'
import { Form } from '../../../../../../branded/src/components/Form'
import { ExpirationDate } from '../../../productSubscription/ExpirationDate'
import { ErrorAlert } from '../../../../components/alerts'
import { useEventObservable } from '../../../../../../shared/src/util/useObservable'
import { Scalars } from '../../../../../../shared/src/graphql-operations'

interface Props {
    subscriptionID: Scalars['ID']
    onGenerate: () => void
}

const LOADING = 'loading' as const

interface FormData {
    /** Comma-separated license tags. */
    tags: string

    userCount: number
    validDays: number | null
    expiresAt: number | null
}

const EMPTY_FORM_DATA: FormData = {
    tags: '',
    userCount: 1,
    validDays: 1,
    expiresAt: addDaysAndRoundToEndOfDay(1),
}

const DURATION_LINKS = [
    { label: '7 days', days: 7 },
    { label: '14 days', days: 14 },
    { label: '30 days', days: 30 },
    { label: '60 days', days: 60 },
    { label: '1 year', days: 366 }, // 366 not 365 to account for leap year
]

/**
 * Displays a form to generate a new product license for a product subscription.
 *
 * For use on Sourcegraph.com by Sourcegraph teammates only.
 */
export const SiteAdminGenerateProductLicenseForSubscriptionForm: React.FunctionComponent<Props> = ({
    subscriptionID,
    onGenerate,
}) => {
    const [formData, setFormData] = useState<FormData>(EMPTY_FORM_DATA)

    const onPlanChange = useCallback<React.ChangeEventHandler<HTMLInputElement>>(
        event => setFormData(formData => ({ ...formData, tags: event.currentTarget.value })),
        []
    )

    const onUserCountChange = useCallback<React.ChangeEventHandler<HTMLInputElement>>(
        event => setFormData(formData => ({ ...formData, userCount: event.currentTarget.valueAsNumber })),
        []
    )

    const setValidDays = useCallback((validDays: number | null): void => {
        setFormData(formData => ({
            ...formData,
            validDays,
            expiresAt: validDays !== null ? addDaysAndRoundToEndOfDay(validDays || 0) : null,
        }))
    }, [])
    const onValidDaysChange = useCallback<React.ChangeEventHandler<HTMLInputElement>>(
        event =>
            setValidDays(Number.isNaN(event.currentTarget.valueAsNumber) ? null : event.currentTarget.valueAsNumber),
        [setValidDays]
    )

    const dismissAlert = useCallback((): void => setFormData(EMPTY_FORM_DATA), [])

    /**
     * The result of creating the product subscription, or undefined when not pending or complete,
     * or loading, or an error.
     */
    const [nextCreation, creation] = useEventObservable(
        useCallback(
            (creations: Observable<void>) =>
                creations.pipe(
                    switchMap(() => {
                        if (formData.expiresAt === null) {
                            throw new Error('invalid expiresAt')
                        }
                        return generateProductLicenseForSubscription({
                            productSubscriptionID: subscriptionID,
                            license: {
                                tags: formData.tags ? formData.tags.split(',') : [],
                                userCount: formData.userCount,
                                expiresAt: Math.ceil(formData.expiresAt / 1000),
                            },
                        }).pipe(
                            tap(() => onGenerate()),
                            catchError(error => [asError(error)]),
                            startWith(LOADING)
                        )
                    })
                ),
            [formData.expiresAt, formData.tags, formData.userCount, onGenerate, subscriptionID]
        )
    )
    const onSubmit = useCallback<React.FormEventHandler>(
        event => {
            event.preventDefault()
            nextCreation()
        },
        [nextCreation]
    )

    const disableForm = Boolean(creation === LOADING || (creation && !isErrorLike(creation)))

    return (
        <div className="site-admin-generate-product-license-for-subscription-form">
            {creation && !isErrorLike(creation) && creation !== LOADING ? (
                <div className="border rounded border-success mb-5">
                    <div className="border-top-0 border-left-0 border-right-0 rounded-0 alert alert-success mb-0 d-flex align-items-center justify-content-between px-3 py-2">
                        <span>Generated product license.</span>
                        <button type="button" className="btn btn-primary" onClick={dismissAlert} autoFocus={true}>
                            Dismiss
                        </button>
                    </div>
                </div>
            ) : (
                <Form onSubmit={onSubmit}>
                    <div className="form-group">
                        <label htmlFor="site-admin-create-product-subscription-page__tags">Tags</label>
                        <input
                            type="text"
                            className="form-control"
                            id="site-admin-create-product-subscription-page__tags"
                            disabled={disableForm}
                            value={formData.tags}
                            list="knownPlans"
                            onChange={onPlanChange}
                        />
                        <datalist id="knownPlans">
                            <option value="true-up" />
                            <option value="trial" />
                            <option value="starter,trial" />
                            <option value="starter,true-up" />
                            <option value="dev" />
                        </datalist>
                        <small className="form-text text-muted">
                            Tags restrict a license. Please refer to{' '}
                            <a href="https://about.sourcegraph.com/handbook/ce/license_keys#how-to-create-a-license-key-for-a-new-prospect-or-new-customer">
                                How to create a license key for a new prospect or new customer
                            </a>{' '}
                            for a complete guide.
                        </small>
                        <small className="form-text text-muted mt-2">
                            To find the exact license tags used for licenses generated by self-service payment, view the{' '}
                            <code>licenseTags</code> product plan metadata item in the billing system.
                        </small>
                    </div>
                    <div className="form-group">
                        <label htmlFor="site-admin-create-product-subscription-page__userCount">Users</label>
                        <input
                            type="number"
                            min={1}
                            className="form-control"
                            id="site-admin-create-product-subscription-page__userCount"
                            disabled={disableForm}
                            value={formData.userCount || ''}
                            onChange={onUserCountChange}
                        />
                    </div>
                    <div className="form-group">
                        <label htmlFor="site-admin-create-product-subscription-page__validDays">Valid for (days)</label>
                        <input
                            type="number"
                            className="form-control"
                            id="site-admin-create-product-subscription-page__validDays"
                            disabled={disableForm}
                            value={formData.validDays || ''}
                            min={1}
                            max={2000} // avoid overflowing int32
                            onChange={onValidDaysChange}
                        />
                        <small className="form-text text-muted">
                            {formData.expiresAt !== null ? (
                                <ExpirationDate
                                    date={formData.expiresAt}
                                    showTime={true}
                                    showRelative={true}
                                    showPrefix={true}
                                />
                            ) : (
                                <>&nbsp;</>
                            )}
                        </small>
                        <small className="form-text text-muted d-block mt-1">
                            Set to{' '}
                            {DURATION_LINKS.map(({ label, days }) => (
                                <a
                                    href="#"
                                    key={days}
                                    className="mr-2"
                                    onClick={event => {
                                        event.preventDefault()
                                        setValidDays(days)
                                    }}
                                >
                                    {label}
                                </a>
                            ))}
                        </small>
                    </div>
                    <button
                        type="submit"
                        disabled={disableForm}
                        className={`btn btn-${disableForm ? 'secondary' : 'primary'}`}
                    >
                        Generate license
                    </button>
                </Form>
            )}
            {isErrorLike(creation) && <ErrorAlert className="mt-3" error={creation} />}
        </div>
    )
}

/**
 * Adds 1 day to the current date, then rounds it up to midnight in the client's timezone. This is a
 * generous interpretation of "valid for N days" to avoid confusion over timezones or "will it
 * expire at the beginning of the day or at the end of the day?"
 */
function addDaysAndRoundToEndOfDay(amount: number): number {
    return endOfDay(addDays(Date.now(), amount)).getTime()
}

function generateProductLicenseForSubscription(
    args: GQL.IGenerateProductLicenseForSubscriptionOnDotcomMutationArguments
): Observable<Pick<GQL.IProductSubscription, 'id'>> {
    return mutateGraphQL(
        gql`
            mutation GenerateProductLicenseForSubscription(
                $productSubscriptionID: ID!
                $license: ProductLicenseInput!
            ) {
                dotcom {
                    generateProductLicenseForSubscription(
                        productSubscriptionID: $productSubscriptionID
                        license: $license
                    ) {
                        id
                    }
                }
            }
        `,
        args
    ).pipe(
        map(({ data, errors }) => {
            if (
                !data ||
                !data.dotcom ||
                !data.dotcom.generateProductLicenseForSubscription ||
                (errors && errors.length > 0)
            ) {
                throw createAggregateError(errors)
            }
            return data.dotcom.generateProductLicenseForSubscription
        })
    )
}
