import React, { useEffect, useMemo, useState } from 'react'

import { QueryClientProvider } from '@tanstack/react-query'
import { useSearchParams } from 'react-router-dom'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import {
    Alert,
    Form,
    Container,
    PageHeader,
    useForm,
    Select,
    ErrorAlert,
    getDefaultInputProps,
    useField,
    createRequiredValidator,
    composeValidators,
    Input,
    Text,
    Label,
    type ValidationResult,
} from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../../../auth'
import { LoaderButton } from '../../../../components/LoaderButton'
import { PageTitle } from '../../../../components/PageTitle'

import { type EnterprisePortalEnvironment, useCreateEnterpriseSubscription, queryClient } from './enterpriseportal'
import { getDefaultEnterprisePortalEnv, EnterprisePortalEnvSelector } from './EnterprisePortalEnvSelector'
import { EnterpriseSubscriptionInstanceType } from './enterpriseportalgen/subscriptions_pb'

interface Props extends TelemetryV2Props {
    authenticatedUser: AuthenticatedUser
}

/**
 * Creates a product subscription for an account based on information provided in the displayed form.
 *
 * For use on Sourcegraph.com by Sourcegraph teammates only.
 */
export const SiteAdminCreateProductSubscriptionPage: React.FunctionComponent<
    React.PropsWithChildren<Props>
> = props => (
    <QueryClientProvider client={queryClient}>
        <Page {...props} />
    </QueryClientProvider>
)

interface FormData {
    displayName: string
    salesforceSubscriptionID: string
    instanceDomain: string
    instanceType: EnterpriseSubscriptionInstanceType
    message: string
}

const QUERY_PARAM_ENV = 'env'

const DISPLAY_NAME_VALIDATOR = createRequiredValidator(
    'A unique display name about this subscription is required. This can be changed later.'
)

const MESSAGE_VALIDATOR = createRequiredValidator('A message about the creation of this subscription is required.')

const SALESFORCE_SUBSCRIPTION_ID_VALIDATOR: (value: string | undefined) => ValidationResult = value => {
    if (!value) {
        return // not required
    }
    if (!value.startsWith('a1a')) {
        return 'Salesforce subscription ID must start with "a1a"'
    }
    if (value.length < 17) {
        return 'Salesforce subscription ID must be 17 characters long'
    }
    return
}

const Page: React.FunctionComponent<React.PropsWithChildren<Props>> = props => {
    useEffect(() => props.telemetryRecorder.recordEvent('admin.productSubscriptions.create', 'view'))

    const [searchParams, setSearchParams] = useSearchParams()
    const [env, setEnv] = useState<EnterprisePortalEnvironment>(
        (searchParams.get(QUERY_PARAM_ENV) as EnterprisePortalEnvironment) || getDefaultEnterprisePortalEnv()
    )
    useEffect(() => {
        searchParams.set(QUERY_PARAM_ENV, env)
        setSearchParams(searchParams)
    }, [env, setSearchParams, searchParams])

    const { mutateAsync: createSubscription, error } = useCreateEnterpriseSubscription(env)

    const {
        formAPI,
        ref: formRef,
        handleSubmit,
    } = useForm<FormData>({
        initialValues: {
            displayName: '',
            message: '',
            salesforceSubscriptionID: '',
            instanceDomain: '',
            instanceType: EnterpriseSubscriptionInstanceType.PRIMARY,
        },
        onSubmit: async ({
            message,
            displayName,
            instanceDomain,
            salesforceSubscriptionID,
            instanceType,
        }: FormData) => {
            props.telemetryRecorder.recordEvent('admin.productSubscriptions', 'create')
            const { subscription } = await createSubscription({
                message,
                subscription: {
                    displayName,
                    instanceDomain,
                    instanceType,
                    salesforce: salesforceSubscriptionID
                        ? {
                              subscriptionId: salesforceSubscriptionID,
                          }
                        : undefined,
                },
            })
            // Redirect to the newly created subscription
            if (subscription) {
                window.location.replace(`/site-admin/dotcom/product/subscriptions/${subscription.id}?env=${env}`)
            }
        },
    })

    const displayName = useField({
        name: 'displayName',
        formApi: formAPI,
        validators: {
            sync: DISPLAY_NAME_VALIDATOR,
        },
    })

    const message = useField({
        name: 'message',
        formApi: formAPI,
        validators: {
            sync: MESSAGE_VALIDATOR,
        },
    })

    const instanceType = useField({
        name: 'instanceType',
        formApi: formAPI,
    })

    const salesforceSubscriptionID = useField({
        name: 'salesforceSubscriptionID',
        formApi: formAPI,
        validators: {
            sync: useMemo(
                () =>
                    composeValidators<string, unknown>([
                        value => {
                            if (instanceType.input.value !== EnterpriseSubscriptionInstanceType.INTERNAL && !value) {
                                return 'Salesforce subscription ID is required for non-internal instances.'
                            }
                            return
                        },
                        SALESFORCE_SUBSCRIPTION_ID_VALIDATOR,
                    ]),
                [instanceType.input.value]
            ),
        },
    })

    const instanceDomain = useField({
        name: 'instanceDomain',
        formApi: formAPI,
    })

    return (
        <div className="site-admin-create-product-subscription-page">
            <PageTitle title="Create Enterprise subscription" />
            <PageHeader
                headingElement="h2"
                path={[
                    { text: 'Enterprise subscriptions', to: '/site-admin/dotcom/product/subscriptions' },
                    { text: 'Create Enterprise subscription' },
                ]}
                className="mb-2"
                actions={<EnterprisePortalEnvSelector env={env} setEnv={setEnv} />}
            />
            <Container className="mb-3">
                {error && <ErrorAlert className="mt-2" error={error} />}
                <Alert variant="info">
                    <Text>
                        <strong>You are creating an Enterprise subscription for a SINGLE Sourcegraph instance</strong>.
                        Customers with multiple Sourcegraph instances should have a separate subscription for each.{' '}
                        <strong>Each subscription should only have licenses for a SINGLE Sourcegraph instance.</strong>
                    </Text>
                    <Text>
                        The Salesforce subscription ID can be set to link multiple Enterprise subscriptions
                        corresponding to a single customer.
                    </Text>
                </Alert>
                <Form ref={formRef} onSubmit={handleSubmit}>
                    <Label className="w-100 mt-2">
                        Display name
                        <Input
                            autoFocus={true}
                            message="Short, human-friendly name for this Enterprise subscription."
                            placeholder="Example: 'Acme Corp. (testing instance)'"
                            disabled={formAPI.submitted}
                            {...getDefaultInputProps(displayName)}
                        />
                    </Label>
                    <Select
                        id="instance-type"
                        label="Instance type"
                        message="Select the type of instance this subscription is used for. A production instance might be a PRIMARY instance, while a dev or staging instance would be a SECONDARY instance. INTERNAL instances are used internally at Sourcegraph."
                        value={instanceType.input.value}
                        disabled={formAPI.submitted}
                        onChange={event => {
                            instanceType.input.onChange(
                                parseInt(event.target.value, 10) as EnterpriseSubscriptionInstanceType
                            )
                        }}
                    >
                        {[
                            EnterpriseSubscriptionInstanceType.PRIMARY,
                            EnterpriseSubscriptionInstanceType.SECONDARY,
                            EnterpriseSubscriptionInstanceType.INTERNAL,
                        ].map(type => (
                            <option key={type} value={type}>
                                {EnterpriseSubscriptionInstanceType[type].toString()}
                            </option>
                        ))}
                    </Select>
                    <Label className="w-100 mt-2">
                        Salesforce subscription ID
                        <Input
                            message="This is VERY important to provide for all subscriptions used by customers. Only leave blank if this subscription is for internal usage."
                            placeholder="Example: 'a1a...'"
                            disabled={formAPI.submitted}
                            {...getDefaultInputProps(salesforceSubscriptionID)}
                        />
                    </Label>
                    <Label className="w-100 mt-2">
                        Instance domain
                        <Input
                            message="External domain of the Sourcegraph instance that will be used by this subscription. Required for Cody Analytics. Must be set manually."
                            placeholder="Example: 'acmecorp.com'"
                            disabled={formAPI.submitted}
                            {...getDefaultInputProps(instanceDomain)}
                        />
                    </Label>
                    <Label className="w-100 mt-2">
                        Message
                        <Input
                            message="Note to associate with the creation of this Enterprise subscription."
                            placeholder="Example: 'Set up test instance subscription for Acme Corp.'"
                            disabled={formAPI.submitted}
                            {...getDefaultInputProps(message)}
                        />
                    </Label>
                    <LoaderButton
                        type="submit"
                        className="mt-2"
                        disabled={formAPI.submitted || !formAPI.valid || formAPI.validating}
                        variant="primary"
                        loading={formAPI.submitted}
                        alwaysShowLabel={true}
                        label="Create subscription"
                    />
                </Form>
            </Container>
        </div>
    )
}
