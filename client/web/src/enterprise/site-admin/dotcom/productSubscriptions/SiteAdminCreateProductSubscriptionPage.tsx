import React, { useEffect, useState } from 'react'

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
    Input,
} from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../../../../auth'
import { LoaderButton } from '../../../../components/LoaderButton'
import { PageTitle } from '../../../../components/PageTitle'

import { type EnterprisePortalEnvironment, useCreateEnterpriseSubscription } from './enterpriseportal'

interface Props extends TelemetryV2Props {
    authenticatedUser: AuthenticatedUser
}

interface FormData {
    displayName: string
    salesforceSubscriptionID?: string
    instanceDomain?: string
    message: string
}

const QUERY_PARAM_ENV = 'env'

/**
 * Creates a product subscription for an account based on information provided in the displayed form.
 *
 * For use on Sourcegraph.com by Sourcegraph teammates only.
 */
export const SiteAdminCreateProductSubscriptionPage: React.FunctionComponent<
    React.PropsWithChildren<Props>
> = props => {
    useEffect(() => props.telemetryRecorder.recordEvent('admin.productSubscriptions.create', 'view'))

    const [searchParams, setSearchParams] = useSearchParams()
    const [env, setEnv] = useState<EnterprisePortalEnvironment>(
        searchParams.get(QUERY_PARAM_ENV) || window.context.deployType === 'dev' ? 'dev' : 'prod'
    )
    useEffect(() => {
        searchParams.set(QUERY_PARAM_ENV, env)
        setSearchParams(searchParams)
    }, [env, setSearchParams, searchParams])

    const { mutate: createSubscription, isPending, error } = useCreateEnterpriseSubscription(env)

    const {
        formAPI,
        ref: formRef,
        handleSubmit,
    } = useForm<FormData>({
        initialValues: {
            displayName: '',
            message: '',
        },
        onSubmit: ({ message, displayName, instanceDomain, salesforceSubscriptionID }: FormData) => {
            props.telemetryRecorder.recordEvent('admin.productSubscriptions', 'create')
            createSubscription(
                {
                    message,
                    subscription: {
                        displayName,
                        instanceDomain,
                        salesforce: salesforceSubscriptionID
                            ? {
                                  subscriptionId: salesforceSubscriptionID,
                              }
                            : undefined,
                    },
                },
                {
                    onSuccess: ({ subscription }) => {
                        // Redirect to the newly created subscription
                        if (subscription) {
                            window.location.replace(
                                `/site-admin/dotcom/product/subscriptions/${subscription.id}&env=${env}`
                            )
                        }
                    },
                }
            )
        },
    })

    const displayName = useField({
        name: 'displayName',
        formApi: formAPI,
        validators: {
            sync: createRequiredValidator(
                'A unique display name about this subscription is required. This can be changed later.'
            ),
        },
    })

    const message = useField({
        name: 'message',
        formApi: formAPI,
        validators: {
            sync: createRequiredValidator('A message about the creation of this subscription is required.'),
        },
    })

    const salesforceSubscriptionID = useField({
        name: 'salesforceSubscriptionID',
        formApi: formAPI,
        validators: {
            sync: value => {
                if (!value?.startsWith('a1a')) {
                    return 'Salesforce subscription ID must start with "a1a"'
                }
                if (value?.length < 17) {
                    return 'Salesforce subscription ID must be 17 characters long'
                }
                return
            },
        },
    })

    const instanceDomain = useField({
        name: 'instanceDomain',
        formApi: formAPI,
    })

    return (
        <div className="site-admin-create-product-subscription-page">
            <PageTitle title="Create product subscription" />
            <PageHeader
                headingElement="h2"
                path={[{ text: 'Create Enterprise subscription' }]}
                className="mb-2"
                actions={
                    <Select
                        id=""
                        name="env"
                        onChange={event => {
                            setEnv(event.target.value as EnterprisePortalEnvironment)
                        }}
                        value={env ?? undefined}
                        className="mb-0"
                        isCustomStyle={true}
                        label="Environment"
                    >
                        {[
                            { label: 'Production', value: 'prod' },
                            { label: 'Development', value: 'dev' },
                        ]
                            .concat(window.context.deployType === 'dev' ? [{ label: 'Local', value: 'local' }] : [])
                            .map(opt => (
                                <option key={opt.value} value={opt.value} label={opt.label} />
                            ))}
                    </Select>
                }
            />
            <Container className="mb-3">
                {error && <ErrorAlert className="mt-2" error={error} />}
                <Alert variant="info">
                    You are creating an Enterprise subscription for a SINGLE Sourcegraph instance. Customers with
                    multiple Sourcegraph instances should have a separate subscription for each. Each subscription
                    should only have licenses for a SINGLE Sourcegraph instance.
                    <br />
                    The Salesforce subscription ID can be set to link subscriptions corresponding to a single customer.
                </Alert>
                <Form ref={formRef} onSubmit={handleSubmit}>
                    <Input
                        autoFocus={true}
                        required={true}
                        message="Subscription display name"
                        about="Human-friendly name for this Enterprise instance subscription. Can be changed later."
                        placeholder="Example: 'Acme Corp. (testing instance)'"
                        {...getDefaultInputProps(displayName)}
                    />
                    <Input
                        message="Salesforce subscription ID"
                        about="This is VERY important to provide for all subscriptions used by customers. Only leave blank if this subscription is for development purposes. Can be changed later."
                        {...getDefaultInputProps(salesforceSubscriptionID)}
                    />
                    <Input
                        message="Instance domain"
                        about="External domain of the Sourcegraph instance that will be used by this subscription. Required for Cody Analytics. Can be changed later."
                        placeholder="Example: 'acmecorp.com'"
                        {...getDefaultInputProps(instanceDomain)}
                    />
                    <Input
                        required={true}
                        message="Message"
                        about="Permanent note to associate with the creation of this Enterprise instance subscription."
                        placeholder="Example: 'Set up test instance subscription for Acme Corp.'"
                        {...getDefaultInputProps(message)}
                    />
                    <LoaderButton
                        type="submit"
                        disabled={isPending || formAPI.submitting || !formAPI.valid}
                        variant="primary"
                        loading={isPending || formAPI.submitting}
                        alwaysShowLabel={true}
                        label="Generate key"
                    />
                </Form>
            </Container>
        </div>
    )
}
