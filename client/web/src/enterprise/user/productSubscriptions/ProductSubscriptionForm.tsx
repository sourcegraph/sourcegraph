import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import React, { useState, useMemo, useEffect, useCallback } from 'react'
import { Link } from 'react-router-dom'
import { ReactStripeElements } from 'react-stripe-elements'
import { from, of, throwError, Observable } from 'rxjs'
import { catchError, startWith, switchMap } from 'rxjs/operators'
import * as GQL from '../../../../../shared/src/graphql/schema'
import { asError, ErrorLike, isErrorLike } from '../../../../../shared/src/util/errors'
import { Form } from '../../../../../branded/src/components/Form'
import { StripeWrapper } from '../../dotcom/billing/StripeWrapper'
import { ProductPlanFormControl } from '../../dotcom/productPlans/ProductPlanFormControl'
import {
    ProductSubscriptionUserCountFormControl,
    MIN_USER_COUNT,
} from '../../dotcom/productPlans/ProductSubscriptionUserCountFormControl'
import { LicenseGenerationKeyWarning } from '../../productSubscription/LicenseGenerationKeyWarning'
import { NewProductSubscriptionPaymentSection } from './NewProductSubscriptionPaymentSection'
import { PaymentTokenFormControl } from './PaymentTokenFormControl'
import { productSubscriptionInputForLocationHash } from './UserSubscriptionsNewProductSubscriptionPage'
import { ThemeProps } from '../../../../../shared/src/theme'
import { ErrorAlert } from '../../../components/alerts'
import { useEventObservable } from '../../../../../shared/src/util/useObservable'
import * as H from 'history'
import { Scalars } from '../../../../../shared/src/graphql-operations'

export enum PaymentValidity {
    Valid = 'Valid',
    Invalid = 'Invalid',
    NoPaymentRequired = 'NoPaymentRequired',
}

/**
 * The form data that is submitted by the ProductSubscriptionForm component.
 */
export interface ProductSubscriptionFormData {
    /** The customer account (user) owning the product subscription. */
    accountID: Scalars['ID']
    productSubscription: GQL.IProductSubscriptionInput
    paymentToken: string | null
}

const LOADING = 'loading' as const

interface Props extends ThemeProps {
    /**
     * The ID of the account associated with the subscription, or null if there is none (in which case this form
     * can only be used to price out a subscription, not to buy).
     */
    accountID: Scalars['ID'] | null

    /**
     * The existing product subscription to edit, if this form is editing an existing subscription,
     * or null if this is a new subscription.
     */
    subscriptionID: Scalars['ID'] | null

    /** Called when the user submits the form (to buy or update the subscription). */
    onSubmit: (args: ProductSubscriptionFormData) => void

    /** The initial value of the form. */
    initialValue?: GQL.IProductSubscriptionInput

    /**
     * The state of the form submission (the operation triggered by onSubmit): undefined when it
     * hasn't been submitted yet, loading, or an error. The parent is expected to redirect to
     * another page when the submission is successful, so this component doesn't handle the form
     * submission success state.
     */
    submissionState: undefined | typeof LOADING | ErrorLike

    /** The text for the form's primary button. */
    primaryButtonText: string

    /**
     * The text for the form's primary button when no payment is required. Defaults to
     * `primaryButtonText` if not set.
     */
    primaryButtonTextNoPaymentRequired?: string

    /** A fragment to render below the form's primary button. */
    afterPrimaryButton?: React.ReactFragment

    history: H.History
}

const DEFAULT_USER_COUNT = MIN_USER_COUNT

/**
 * Displays a form for a product subscription.
 */
const _ProductSubscriptionForm: React.FunctionComponent<Props & ReactStripeElements.InjectedStripeProps> = ({
    accountID,
    subscriptionID,
    onSubmit: parentOnSubmit,
    initialValue,
    submissionState,
    primaryButtonText,
    primaryButtonTextNoPaymentRequired = primaryButtonText,
    afterPrimaryButton,
    isLightTheme,
    stripe,
    history,
}) => {
    if (!stripe) {
        throw new Error('billing service is not available')
    }

    /** The selected product plan. */
    const [billingPlanID, setBillingPlanID] = useState<string | null>(initialValue?.billingPlanID || null)

    /** The user count input by the user. */
    const [userCount, setUserCount] = useState<number | null>(initialValue?.userCount || DEFAULT_USER_COUNT)

    /** The validity of the payment and billing information. */
    const [paymentValidity, setPaymentValidity] = useState<PaymentValidity>(PaymentValidity.Invalid)

    // When Props#initialValue changes, clobber our values. It's unlikely that this prop would
    // change without the component being unmounted, but handle this case for completeness
    // anyway.
    useEffect(() => {
        setBillingPlanID(initialValue?.billingPlanID || null)
        setUserCount(initialValue?.userCount || DEFAULT_USER_COUNT)
    }, [initialValue])

    /**
     * The result of creating the billing token (which refers to the payment method chosen by the
     * user): undefined if successful or not yet started, loading, or an error.
     */
    const [nextSubmit, paymentToken] = useEventObservable(
        useCallback(
            (submits: Observable<void>) =>
                submits.pipe(
                    switchMap(() =>
                        // TODO(sqs): store name, address, company, etc., in token
                        (paymentValidity !== PaymentValidity.NoPaymentRequired
                            ? from(stripe.createToken())
                            : of({ token: undefined, error: undefined })
                        ).pipe(
                            switchMap(({ token, error }) => {
                                if (error) {
                                    return throwError(error)
                                }
                                if (!accountID) {
                                    return throwError(new Error('no account (unauthenticated user)'))
                                }
                                if (!billingPlanID) {
                                    return throwError(new Error('no product plan selected'))
                                }
                                if (userCount === null) {
                                    return throwError(new Error('invalid user count'))
                                }
                                if (!token && paymentValidity !== PaymentValidity.NoPaymentRequired) {
                                    return throwError(new Error('invalid payment and billing'))
                                }
                                parentOnSubmit({
                                    accountID,
                                    productSubscription: {
                                        billingPlanID,
                                        userCount,
                                    },
                                    paymentToken: token ? token.id : null,
                                })
                                return of(undefined)
                            }),
                            catchError(error => [asError(error)]),
                            startWith(LOADING)
                        )
                    )
                ),
            [accountID, billingPlanID, parentOnSubmit, paymentValidity, stripe, userCount]
        )
    )
    const onSubmit = useCallback<React.FormEventHandler>(
        event => {
            event.preventDefault()
            nextSubmit()
        },
        [nextSubmit]
    )

    const disableForm = Boolean(
        submissionState === LOADING ||
            userCount === null ||
            paymentValidity === PaymentValidity.Invalid ||
            paymentToken === LOADING ||
            (paymentToken && !isErrorLike(paymentToken))
    )

    const productSubscriptionInput = useMemo<GQL.IProductSubscriptionInput | null>(
        () =>
            billingPlanID !== null && userCount !== null
                ? {
                      billingPlanID,
                      userCount,
                  }
                : null,
        [billingPlanID, userCount]
    )

    return (
        <div className="product-subscription-form">
            <LicenseGenerationKeyWarning />
            <Form onSubmit={onSubmit}>
                <div className="row">
                    <div className="col-md-6">
                        <ProductSubscriptionUserCountFormControl value={userCount} onChange={setUserCount} />
                        <h4 className="mt-2 mb-0">Plan</h4>
                        <ProductPlanFormControl value={billingPlanID} onChange={setBillingPlanID} />
                    </div>
                    <div className="col-md-6 mt-3 mt-md-0">
                        <h3 className="mt-2 mb-0">Billing</h3>
                        <NewProductSubscriptionPaymentSection
                            productSubscription={productSubscriptionInput}
                            accountID={accountID}
                            subscriptionID={subscriptionID}
                            onValidityChange={setPaymentValidity}
                        />
                        {!accountID && (
                            <div className="form-group mt-3">
                                <Link
                                    to={`/sign-up?returnTo=${encodeURIComponent(
                                        `/subscriptions/new${productSubscriptionInputForLocationHash(
                                            productSubscriptionInput
                                        )}`
                                    )}`}
                                    className="btn btn-lg btn-primary w-100 center"
                                >
                                    Create account or sign in to continue
                                </Link>
                                <small className="form-text text-muted">
                                    A user account on Sourcegraph.com is required to create a subscription so you can
                                    view the license key and invoice.
                                </small>
                                <hr className="my-3" />
                                <small className="form-text text-muted">
                                    Next, you'll enter payment information and buy the subscription.
                                </small>
                            </div>
                        )}
                        <PaymentTokenFormControl
                            disabled={
                                disableForm || !accountID || paymentValidity === PaymentValidity.NoPaymentRequired
                            }
                            isLightTheme={isLightTheme}
                        />
                        <div className="form-group mt-3">
                            <button
                                type="submit"
                                disabled={disableForm || !accountID}
                                className={`btn btn-lg btn-${
                                    disableForm || !accountID ? 'secondary' : 'success'
                                } w-100 d-flex align-items-center justify-content-center`}
                            >
                                {paymentToken === LOADING || submissionState === LOADING ? (
                                    <>
                                        <LoadingSpinner className="icon-inline mr-2" /> Processing...
                                    </>
                                ) : paymentValidity !== PaymentValidity.NoPaymentRequired ? (
                                    primaryButtonText
                                ) : (
                                    primaryButtonTextNoPaymentRequired
                                )}
                            </button>
                            {afterPrimaryButton}
                        </div>
                    </div>
                </div>
            </Form>
            {isErrorLike(paymentToken) && <ErrorAlert className="mt-3" error={paymentToken} />}
            {isErrorLike(submissionState) && <ErrorAlert className="mt-3" error={submissionState} />}
        </div>
    )
}

export const ProductSubscriptionForm: React.FunctionComponent<Props> = props => (
    <StripeWrapper<Props> component={_ProductSubscriptionForm} {...props} />
)
