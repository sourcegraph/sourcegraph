import React, { useEffect, useCallback } from 'react'

import { mdiMinusThick, mdiPlusThick, mdiChevronUp, mdiChevronDown } from '@mdi/js'
import { AddressElement, useStripe, useElements, CardNumberElement } from '@stripe/react-stripe-js'
import type { Stripe, StripeCardNumberElement } from '@stripe/stripe-js'
import type { StripeAddressElementChangeEvent } from '@stripe/stripe-js/dist/stripe-js/elements/address'
import classNames from 'classnames'
import { useNavigate, useSearchParams } from 'react-router-dom'

import { pluralize } from '@sourcegraph/common'
import {
    Form,
    Link,
    Button,
    Grid,
    H2,
    Text,
    Container,
    Icon,
    LoadingSpinner,
    Collapse,
    H4,
    CollapseHeader,
    CollapsePanel,
} from '@sourcegraph/wildcard'

import { CodyProRoutes } from '../../../codyProRoutes'
import { CodyAlert } from '../../../components/CodyAlert'
import {
    useCreateTeam,
    usePreviewUpdateCurrentSubscription,
    useUpdateCurrentSubscription,
} from '../../api/react-query/subscriptions'
import type { Subscription } from '../../api/types'
import { BillingAddressPreview } from '../BillingAddressPreview'
import { PaymentMethodPreview } from '../PaymentMethodPreview'
import { StripeAddressElement } from '../StripeAddressElement'
import { StripeCardDetails } from '../StripeCardDetails'

import styles from './NewCodyProSubscriptionPage.module.scss'

// Hard limits
const MIN_SEAT_COUNT = 1
const MAX_SEAT_COUNT = 50

// Monthly seat price in USD
const SEAT_PRICE = 9

interface CodyProCheckoutFormProps {
    subscription?: Subscription
    customerEmail: string | undefined
}

async function createStripeToken(
    stripe: Stripe,
    cardNumberElement: StripeCardNumberElement,
    suppliedAddress: StripeAddressElementChangeEvent['value']['address']
): Promise<string> {
    let response
    try {
        // Note that Stripe may have returned an error response.
        response = await stripe.createToken(cardNumberElement, {
            // We send the address data along with the card info to let Stripe do more validation such as
            // confirming the zip code matches the card's. Later, we'll also store this as the Customer's address.
            address_line1: suppliedAddress.line1,
            address_line2: suppliedAddress.line2 || '',
            address_city: suppliedAddress.city,
            address_state: suppliedAddress.state,
            address_zip: suppliedAddress.postal_code,
            address_country: suppliedAddress.country,
            currency: 'usd',
        })
    } catch (error) {
        throw new Error(`We couldn't create the team. This is what happened: ${error}`)
    }
    if (response.error) {
        throw new Error(response.error.message ?? 'We got an unknown error from Stripe.')
    }
    const tokenId = response.token?.id
    if (!tokenId) {
        throw new Error('Stripe token not found.')
    }
    return tokenId
}

interface TeamSizeChange {
    seatCountDiff: number
    priceDueNow: number
    monthlyPriceDiff: number
    newMonthlyPrice: number
    dueDate?: string
}

export const CodyProCheckoutForm: React.FunctionComponent<CodyProCheckoutFormProps> = ({
    subscription,
    customerEmail,
}) => {
    const stripe = useStripe()
    const elements = useElements()
    const navigate = useNavigate()

    const [urlSearchParams] = useSearchParams()
    const addSeats = !!urlSearchParams.get('addSeats')
    const initialSeatCount = addSeats && subscription ? subscription.maxSeats : 0
    const maxNewSeatCount = MAX_SEAT_COUNT - initialSeatCount
    const initialNewSeats = Math.max(
        Math.min(maxNewSeatCount, parseInt(urlSearchParams.get('seats') || '', 10) || 1),
        MIN_SEAT_COUNT
    )
    const isTeam = addSeats || initialNewSeats > 1

    const [errorMessage, setErrorMessage] = React.useState<string | null>(null)
    // In the case of new subscriptions we have 0 initial seats, so "addedSeatCount" is actually just "seatCount".
    const [submitting, setSubmitting] = React.useState(false)
    const [isCardAndAddressSectionExpanded, setIsCardAndAddressSectionExpanded] = React.useState(false)

    const createTeamMutation = useCreateTeam()
    const previewUpdateCurrentSubscriptionMutation = usePreviewUpdateCurrentSubscription()
    const updateCurrentSubscriptionMutation = useUpdateCurrentSubscription()

    const [planChange, setPlanChange] = React.useState<TeamSizeChange>({
        seatCountDiff: initialNewSeats,
        monthlyPriceDiff: initialNewSeats * SEAT_PRICE,
        newMonthlyPrice: (initialSeatCount + initialNewSeats) * SEAT_PRICE,
        priceDueNow: initialNewSeats * SEAT_PRICE,
        dueDate: undefined,
    })

    const onSeatCountDiffChange = useCallback(
        (newSeatCountDiff: number): void => {
            // In the case of a new subscription, we can recalculate prices locally. Otherwise, use the back end.
            if (!addSeats) {
                setPlanChange({
                    seatCountDiff: newSeatCountDiff,
                    monthlyPriceDiff: newSeatCountDiff * SEAT_PRICE,
                    newMonthlyPrice: (initialSeatCount + newSeatCountDiff) * SEAT_PRICE,
                    priceDueNow: newSeatCountDiff * SEAT_PRICE,
                    dueDate: undefined,
                })
            } else {
                // The `.call` call is a workaround because `previewUpdateCurrentSubscriptionMutation` is not referentially stable,
                // and adding `previewUpdateCurrentSubscriptionMutation` to the list of dependencies causes an infinite loop.
                // `previewUpdateCurrentSubscriptionMutation.mutate` IS referentially stable, and it doesn't internally reference `this`,
                // so calling `.call` is safe. See https://github.com/TanStack/query/issues/1858#issuecomment-1255678830
                previewUpdateCurrentSubscriptionMutation.mutate.call(
                    undefined,
                    { newSeatCount: initialSeatCount + newSeatCountDiff },
                    {
                        onSuccess: result => {
                            if (result) {
                                setPlanChange({
                                    seatCountDiff: newSeatCountDiff,
                                    monthlyPriceDiff: result.newPrice / 100 - initialSeatCount * SEAT_PRICE,
                                    newMonthlyPrice: result.newPrice / 100,
                                    priceDueNow: result.dueNow / 100,
                                    dueDate: result.dueDate,
                                })
                            }
                        },
                    }
                )
            }
        },
        [addSeats, initialSeatCount, previewUpdateCurrentSubscriptionMutation.mutate]
    )

    // Load the initial prices if needed.
    useEffect(() => {
        if (addSeats) {
            onSeatCountDiffChange(initialNewSeats)
        }
    }, [addSeats, initialNewSeats, onSeatCountDiffChange])

    const handleSubscribeSubmit = useCallback(
        async (event: React.FormEvent<HTMLFormElement>): Promise<void> => {
            event.preventDefault()

            if (!stripe || !elements) {
                setErrorMessage('Stripe or Stripe Elements libraries not available.')
                return
            }
            const cardNumberElement = elements.getElement(CardNumberElement)
            if (!cardNumberElement) {
                setErrorMessage('CardNumberElement not found.')
                return
            }
            const addressElement = elements.getElement(AddressElement)
            if (!addressElement) {
                setErrorMessage('AddressElement not found.')
                return
            }
            const addressElementValue = await addressElement.getValue()
            if (!addressElementValue.complete) {
                setErrorMessage('Please fill out your billing address.')
                return
            }

            const suppliedAddress = addressElementValue.value.address

            setSubmitting(true)

            let token
            try {
                token = await createStripeToken(stripe, cardNumberElement, suppliedAddress)
            } catch (error) {
                setErrorMessage(error)
                setSubmitting(false)
                return
            }

            // This is where we send the token to the backend to create a subscription.
            try {
                // Even though .mutate is recommended (https://tkdodo.eu/blog/mastering-mutations-in-react-query#mutate-or-mutateasync),
                // this use makes it very convenient to just have a linear flow with error handling and a redirect at the end.
                // And `.call` is a workaround to https://github.com/TanStack/query/issues/1858#issuecomment-1255678830
                await createTeamMutation.mutateAsync.call(undefined, {
                    name: '(no name yet)',
                    slug: '(no slug yet)',
                    seats: planChange.seatCountDiff,
                    address: {
                        line1: suppliedAddress.line1,
                        line2: suppliedAddress.line2 || '',
                        city: suppliedAddress.city,
                        state: suppliedAddress.state,
                        postalCode: suppliedAddress.postal_code,
                        country: suppliedAddress.country,
                    },
                    billingInterval: 'monthly',
                    couponCode: '',
                    creditCardToken: token,
                })

                navigate(`${CodyProRoutes.Manage}?welcome=1`)

                setSubmitting(false)
            } catch (error) {
                setErrorMessage(`We couldn't create the Stripe token. This is what happened: ${error}`)
                setSubmitting(false)
            }
        },
        [stripe, elements, createTeamMutation.mutateAsync, planChange, navigate]
    )

    const handlePlanChangeSubmit = useCallback(
        async (event: React.FormEvent<HTMLFormElement>): Promise<void> => {
            event.preventDefault()

            if (!stripe || !elements) {
                setErrorMessage('Stripe or Stripe Elements libraries not available.')
                return
            }

            setSubmitting(true)

            try {
                await updateCurrentSubscriptionMutation.mutateAsync.call(undefined, {
                    subscriptionUpdate: {
                        newSeatCount: initialSeatCount + planChange.seatCountDiff,
                    },
                })
            } catch (error) {
                setErrorMessage(`We couldn't update your subscription. This is what happened: ${error}`)
                setSubmitting(false)
                return
            }

            navigate(CodyProRoutes.ManageTeam)

            setSubmitting(false)
        },
        [
            elements,
            initialSeatCount,
            navigate,
            planChange.seatCountDiff,
            stripe,
            updateCurrentSubscriptionMutation.mutateAsync,
        ]
    )

    return (
        <>
            {initialSeatCount + planChange.seatCountDiff >= 30 && (
                <CodyAlert title="Explore an enterprise plan" variant="purple">
                    <Text>
                        Team plans are limited to 50 users.{' '}
                        <Link to="https://sourcegraph.com/contact/sales/">Contact sales</Link> to learn more.
                    </Text>
                </CodyAlert>
            )}
            <Container>
                <Grid columnCount={2} spacing={4} className="mb-0">
                    <div>
                        <SeatCountSelector
                            header={isTeam ? 'Add seats' : 'Select number of seats'}
                            current={planChange.seatCountDiff}
                            min={MIN_SEAT_COUNT}
                            max={maxNewSeatCount}
                            setCount={onSeatCountDiffChange}
                        />
                        <Summary
                            addSeats={addSeats}
                            isLoading={previewUpdateCurrentSubscriptionMutation.isPending}
                            initialSeatCount={initialSeatCount}
                            change={planChange}
                        />
                    </div>
                    <div>
                        <H2 className="font-medium">
                            Purchase {planChange.seatCountDiff} {pluralize('seat', planChange.seatCountDiff)}
                        </H2>
                        {addSeats ? (
                            <Form onSubmit={handlePlanChangeSubmit}>
                                {!!subscription && (
                                    <Collapse
                                        isOpen={isCardAndAddressSectionExpanded}
                                        onOpenChange={setIsCardAndAddressSectionExpanded}
                                        openByDefault={false}
                                    >
                                        <CollapseHeader
                                            as={Button}
                                            variant="secondary"
                                            outline={true}
                                            className="p-0 m-0 mt-2 mb-2 border-0 w-100 font-weight-normal d-flex justify-content-between align-items-center"
                                        >
                                            <H4 className="m-0">Show credit card and billing info</H4>
                                            <Icon
                                                aria-hidden={true}
                                                svgPath={
                                                    isCardAndAddressSectionExpanded ? mdiChevronUp : mdiChevronDown
                                                }
                                                className="mr-1"
                                                size="md"
                                            />
                                        </CollapseHeader>
                                        <CollapsePanel>
                                            <PaymentMethodPreview
                                                paymentMethod={subscription.paymentMethod}
                                                isEditable={false}
                                                className="mb-4"
                                            />
                                            <BillingAddressPreview subscription={subscription} isEditable={false} />
                                        </CollapsePanel>
                                    </Collapse>
                                )}
                                {errorMessage && (
                                    <div className={classNames(styles.paymentDataErrorMessage)}>{errorMessage}</div>
                                )}

                                <Button
                                    variant="primary"
                                    disabled={submitting}
                                    className={classNames('d-block w-100 mb-4', styles.payButton)}
                                    type="submit"
                                >
                                    {submitting ? (
                                        <LoadingSpinner className={styles.lineHeightLoadingSpinner} />
                                    ) : (
                                        'Confirm plan changes'
                                    )}
                                </Button>

                                <div>
                                    <Text size="small" className={styles.disclaimer}>
                                        By clicking the button, you agree to the{' '}
                                        <Link to="/terms/cloud">Terms of Service</Link> and acknowledge that the{' '}
                                        <Link to="/terms/privacy">Privacy Statement</Link> applies. Your subscription
                                        will renew automatically by charging your payment method on file until you{' '}
                                        <Link to="/docs/cody/usage-and-pricing#downgrading-from-pro-to-free">
                                            cancel
                                        </Link>
                                        . You may cancel at any time prior to the next billing cycle.
                                    </Text>
                                </div>
                            </Form>
                        ) : (
                            <Form onSubmit={handleSubscribeSubmit}>
                                <StripeCardDetails className="mb-4" onFocus={() => setErrorMessage('')} />

                                <Text className="mb-2 font-medium text-sm">Email</Text>
                                <Text className="ml-3 mb-4 font-medium text-sm">{customerEmail || ''} </Text>

                                <StripeAddressElement onFocus={() => setErrorMessage('')} />

                                {errorMessage && (
                                    <div className={classNames(styles.paymentDataErrorMessage)}>{errorMessage}</div>
                                )}

                                <Button
                                    variant="primary"
                                    disabled={submitting}
                                    className={classNames('d-block w-100 mb-4', styles.payButton)}
                                    type="submit"
                                >
                                    {submitting ? (
                                        <LoadingSpinner className={styles.lineHeightLoadingSpinner} />
                                    ) : (
                                        'Subscribe'
                                    )}
                                </Button>

                                <div>
                                    <Text size="small" className={styles.disclaimer}>
                                        By clicking the button, you agree to the{' '}
                                        <Link to="/terms/cloud">Terms of Service</Link> and acknowledge that the{' '}
                                        <Link to="/terms/privacy">Privacy Statement</Link> applies. Your subscription
                                        will renew automatically by charging your payment method on file until you{' '}
                                        <Link to="/docs/cody/usage-and-pricing#downgrading-from-pro-to-free">
                                            cancel
                                        </Link>
                                        . You may cancel at any time prior to the next billing cycle.
                                    </Text>
                                </div>
                            </Form>
                        )}
                    </div>
                </Grid>
            </Container>
        </>
    )
}

const SeatCountSelector: React.FunctionComponent<{
    header: string
    current: number
    min: number
    max: number
    setCount: (count: number) => void
}> = ({ header, current, min, max, setCount }) => (
    <>
        <H2 className="font-medium mb-3c">{header}</H2>
        <div className="d-flex flex-row align-items-center pb-3c mb-3c border-bottom">
            <div className="flex-1">${SEAT_PRICE} per seat / month</div>
            <Button
                disabled={current === min}
                onClick={() => setCount(current > min ? current - 1 : current)}
                className="px-3c py-2 border-0"
            >
                <Icon aria-hidden={true} svgPath={mdiMinusThick} className={styles.plusMinusButton} />
            </Button>
            <div className={styles.seatCountSelectorValue}>{current}</div>
            <Button
                disabled={current === max}
                onClick={() => setCount(current < max ? current + 1 : current)}
                className="px-3c py-2 border-0"
            >
                <Icon aria-hidden={true} svgPath={mdiPlusThick} className={styles.plusMinusButton} />
            </Button>
        </div>
    </>
)

const Summary: React.FunctionComponent<{
    addSeats: boolean
    isLoading: boolean
    initialSeatCount: number
    change: TeamSizeChange
}> = ({ addSeats, isLoading, initialSeatCount, change }) => (
    <>
        <H2 className="font-medium mb-3c">Summary</H2>
        {addSeats && (
            <PriceOrSpinner price={change.priceDueNow} isLoading={isLoading}>
                Pro-rated cost for this month
            </PriceOrSpinner>
        )}
        <PriceOrSpinner price={change.monthlyPriceDiff} isLoading={isLoading}>
            {addSeats ? 'Adding ' : ''} {change.seatCountDiff} {pluralize('seat', change.seatCountDiff)}
        </PriceOrSpinner>
        {addSeats && (
            <PriceOrSpinner price={change.newMonthlyPrice} isLoading={isLoading}>
                New total for {initialSeatCount + change.seatCountDiff}{' '}
                {pluralize('seat', initialSeatCount + change.seatCountDiff)}
            </PriceOrSpinner>
        )}
        {addSeats && (
            <Text size="small" className={styles.disclaimer}>
                New seats are pro-rated this month, and will be charged at the full rate{' '}
                {change.dueDate ? `on ${new Date(change.dueDate).toLocaleDateString()}` : 'next month'}.
            </Text>
        )}
    </>
)

interface PriceOrSpinnerProps {
    price: number
    isLoading: boolean
    children: React.ReactNode
}

const PriceOrSpinner: React.FunctionComponent<PriceOrSpinnerProps> = ({ price, isLoading, children }) => (
    <div className="d-flex flex-row align-items-center mb-4">
        <div className="flex-1">{children}</div>
        <div className={styles.price}>
            {isLoading ? (
                <LoadingSpinner className={styles.lineHeightLoadingSpinner} />
            ) : (
                <strong>${price} / month</strong>
            )}
        </div>
    </div>
)
