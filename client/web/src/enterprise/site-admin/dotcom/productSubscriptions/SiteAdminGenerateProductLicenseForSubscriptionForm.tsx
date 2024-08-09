import React, { useState, useCallback } from 'react'

import { Timestamp } from '@bufbuild/protobuf'
import { UTCDate } from '@date-fns/utc'
import { mdiChatQuestionOutline } from '@mdi/js'
import classNames from 'classnames'
import { addDays, endOfDay } from 'date-fns'
import { noop } from 'lodash'

import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import {
    Alert,
    Button,
    Link,
    Input,
    ErrorAlert,
    Form,
    Select,
    useDebounce,
    H3,
    Modal,
    Text,
    Checkbox,
    H4,
    Icon,
    Tooltip,
    Label,
} from '@sourcegraph/wildcard'

import { Collapsible } from '../../../../components/Collapsible'
import { LoaderButton } from '../../../../components/LoaderButton'
import { ExpirationDate } from '../../../productSubscription/ExpirationDate'
import { hasUnknownTags, ProductLicenseTags, UnknownTagWarning } from '../../../productSubscription/ProductLicenseTags'

import { type EnterprisePortalEnvironment, useCreateEnterpriseSubscriptionLicense } from './enterpriseportal'
import type { EnterpriseSubscription, EnterpriseSubscriptionLicense } from './enterpriseportalgen/subscriptions_pb'
import {
    ALL_PLANS,
    TAG_AIR_GAPPED,
    TAG_BATCH_CHANGES,
    TAG_CODE_INSIGHTS,
    TAG_DISABLE_TELEMETRY_EXPORT,
    TAG_TRIAL,
    TAG_TRUEUP,
} from './plandata'

import styles from './SiteAdminGenerateProductLicenseForSubscriptionForm.module.scss'

interface Props extends TelemetryV2Props {
    env: EnterprisePortalEnvironment
    subscription: EnterpriseSubscription
    latestLicense: EnterpriseSubscriptionLicense | undefined
    onGenerate: () => void
    onCancel: () => void
}

interface FormData {
    message: string
    /** Comma-separated additional license tags. */
    tags: string
    salesforceOpportunityID: string
    plan: string
    userCount: bigint
    expiresAt: Date
    trueUp: boolean
    trial: boolean
    airGapped: boolean
    disableTelemetry: boolean
    batchChanges: boolean
    codeInsights: boolean
}

const getEmptyFormData = (latestLicense: EnterpriseSubscriptionLicense | undefined): FormData => {
    const licenseData = latestLicense?.license?.value
    const formData: FormData = {
        message: '',
        tags: '',
        salesforceOpportunityID: licenseData?.info?.salesforceOpportunityId ?? '',
        plan: licenseData?.info?.tags.find(tag => tag.startsWith('plan:'))?.slice('plan:'.length) ?? '',
        userCount: licenseData?.info?.userCount ?? BigInt(1),
        expiresAt: endOfDay(new UTCDate(UTCDate.now())),
        trial: licenseData?.info?.tags.includes(TAG_TRIAL.tagValue) ?? false,
        trueUp: licenseData?.info?.tags.includes(TAG_TRUEUP.tagValue) ?? false,
        airGapped: licenseData?.info?.tags.includes(TAG_AIR_GAPPED.tagValue) ?? false,
        batchChanges: licenseData?.info?.tags.includes(TAG_BATCH_CHANGES.tagValue) ?? false,
        codeInsights: licenseData?.info?.tags.includes(TAG_CODE_INSIGHTS.tagValue) ?? false,
        disableTelemetry: licenseData?.info?.tags.includes(TAG_DISABLE_TELEMETRY_EXPORT.tagValue) ?? false,
    }

    if (licenseData?.info) {
        // Based on the tag-less formData created above, generate the list of tags to add.
        // We then only add additional tags for the things that aren't yet expressed,
        // to avoid duplicates and let the specific flags on form data handle addition
        // of their tag values.
        const presentTags = getTagsFromFormData(formData)
        formData.tags =
            licenseData?.info?.tags
                .filter(tag => !tag.startsWith('plan:') && !tag.startsWith('customer:') && !presentTags.includes(tag))
                .join(',') ?? ''
    }

    return formData
}

const DURATION_LINKS = [
    { label: '7 days', days: 7 },
    { label: '14 days', days: 14 },
    { label: '30 days', days: 30 },
    { label: '60 days', days: 60 },
    { label: '1 year', days: 366 }, // 366 not 365 to account for leap year
]

const tagsFromString = (tagString: string): string[] =>
    tagString
        .split(',')
        .map(item => item.trim())
        .filter(tag => tag !== '')

const getTagsFromFormData = (formData: FormData): string[] =>
    Array.from(
        new Set([
            ...(formData.plan ? [`plan:${formData.plan}`] : []),
            ...(formData.trueUp &&
            ALL_PLANS.find(other => other.label === formData.plan)?.additionalTags?.some(
                tag => tag.tagValue === TAG_TRUEUP.tagValue
            )
                ? [TAG_TRUEUP.tagValue]
                : []),
            ...(formData.trial ? [TAG_TRIAL.tagValue] : []),
            ...(formData.airGapped ? [TAG_AIR_GAPPED.tagValue] : []),
            ...(formData.batchChanges ? [TAG_BATCH_CHANGES.tagValue] : []),
            ...(formData.codeInsights ? [TAG_CODE_INSIGHTS.tagValue] : []),
            ...(formData.disableTelemetry ? [TAG_DISABLE_TELEMETRY_EXPORT.tagValue] : []),
            ...tagsFromString(formData.tags),
        ])
    )

const getTagsForTelemetry = (formData: FormData): { [key: string]: number } => ({
    trueUp: formData.trueUp ? 1 : 0,
    trial: formData.trial ? 1 : 0,
    airGapped: formData.airGapped ? 1 : 0,
    batchChanges: formData.batchChanges ? 1 : 0,
    codeInsights: formData.codeInsights ? 1 : 0,
    disableTelemetry: formData.disableTelemetry ? 1 : 0,
})

/**
 * Displays a form to generate a new product license for a product subscription.
 *
 * For use on Sourcegraph.com by Sourcegraph teammates only.
 */
export const SiteAdminGenerateProductLicenseForSubscriptionForm: React.FunctionComponent<
    React.PropsWithChildren<Props>
> = ({ env, latestLicense, subscription, onGenerate, onCancel, telemetryRecorder }) => {
    const labelId = 'generateLicense'

    const [hasAcknowledgedInfo, setHasAcknowledgedInfo] = useState(false)

    const [formData, setFormData] = useState<FormData>(getEmptyFormData(latestLicense))

    const onPlanChange = useCallback<React.ChangeEventHandler<HTMLSelectElement>>(
        event => setFormData(formData => ({ ...formData, plan: event.target.value })),
        []
    )

    const useOnChange = (key: string): React.ChangeEventHandler<HTMLInputElement> =>
        useCallback<React.ChangeEventHandler<HTMLInputElement>>(
            event => setFormData(formData => ({ ...formData, [key]: event.target.value })),
            [key]
        )
    const onMessageChange = useOnChange('message')
    const onSFOpportunityIDChange = useOnChange('salesforceOpportunityID')

    const [sfOpportunityIDError, setSFOpportunityIDError] = useState<string | undefined>(undefined)

    const onTagsChange = useCallback<React.ChangeEventHandler<HTMLInputElement>>(
        event => setFormData(formData => ({ ...formData, tags: event.target.value || '' })),
        []
    )

    const onUserCountChange = useCallback<React.ChangeEventHandler<HTMLInputElement>>(
        event => setFormData(formData => ({ ...formData, userCount: BigInt(event.target.valueAsNumber || 0) })),
        []
    )

    const onTrueUpChange = useCallback<React.ChangeEventHandler<HTMLInputElement>>(
        event => setFormData(formData => ({ ...formData, trueUp: event.target.checked })),
        []
    )

    const onIsTrialChange = useCallback<React.ChangeEventHandler<HTMLInputElement>>(
        event => setFormData(formData => ({ ...formData, trial: event.target.checked })),
        []
    )

    const onAirGappedChange = useCallback<React.ChangeEventHandler<HTMLInputElement>>(
        event => setFormData(formData => ({ ...formData, airGapped: event.target.checked })),
        []
    )

    const onDisableTelemetryChange = useCallback<React.ChangeEventHandler<HTMLInputElement>>(
        event => setFormData(formData => ({ ...formData, disableTelemetry: event.target.checked })),
        []
    )

    const onBatchChangesChange = useCallback<React.ChangeEventHandler<HTMLInputElement>>(
        event => setFormData(formData => ({ ...formData, batchChanges: event.target.checked })),
        []
    )

    const onCodeInsightsChange = useCallback<React.ChangeEventHandler<HTMLInputElement>>(
        event => setFormData(formData => ({ ...formData, codeInsights: event.target.checked })),
        []
    )

    const setValidDays = useCallback((validDays: number): void => {
        setFormData(formData => ({
            ...formData,
            expiresAt: addDaysAndRoundToEndOfDayInUTC(validDays),
        }))
    }, [])
    const onExpiresAtChange = useCallback<React.ChangeEventHandler<HTMLInputElement>>(
        event => {
            // The event.target.valueAsDate property returns a native Javascript Date object.
            // However, we can't use the endOfDay() date-fns utility with it since by default it
            // does all calculations using the browser's local timezone, not UTC.
            //
            // However, as of date-fns@v3, they introduced a new Date wrapper, UTCDate. When using this wrapper,
            // all of the date-fns calculations will do them with respect to UTC (which is the desired behavior with
            // expiration dates).

            // The value field from the date picker input element is in the format
            // yyyy-mm-dd, so we can use it to directly construct a new UTCDate object
            // with the appropriate date.
            //
            // See https://developer.mozilla.org/en-US/docs/Web/HTML/Element/input/date#value
            // for more information.
            const dateRegex = /\d{4}-\d{2}-\d{2}/

            const date: Date = dateRegex.test(event.target.value)
                ? new UTCDate(event.target.value)
                : getEmptyFormData(latestLicense).expiresAt

            const expiresAt = endOfDay(new UTCDate(date))

            setFormData(formData => ({
                ...formData,
                expiresAt,
            }))
        },
        [latestLicense]
    )

    const { mutate: generateLicense, isPending: isLoading, error } = useCreateEnterpriseSubscriptionLicense(env)

    const onSubmit = useCallback<React.FormEventHandler>(
        event => {
            event.preventDefault()
            telemetryRecorder.recordEvent('admin.productSubscription.license', 'generate', {
                metadata: getTagsForTelemetry(formData),
            })
            generateLicense(
                {
                    message: formData.message,
                    license: {
                        subscriptionId: subscription.id,
                        license: {
                            // We only support creating old-school license keys
                            case: 'key',
                            value: {
                                info: {
                                    tags: getTagsFromFormData(formData),
                                    userCount: formData.userCount,
                                    expireTime: Timestamp.fromDate(formData.expiresAt),
                                    salesforceOpportunityId:
                                        formData.salesforceOpportunityID.trim().length > 0
                                            ? formData.salesforceOpportunityID.trim()
                                            : undefined,
                                },
                            },
                        },
                    },
                },
                {
                    onSuccess: onGenerate,
                }
            )
        },
        [formData, telemetryRecorder, generateLicense, subscription, onGenerate]
    )

    const tags = useDebounce<string[]>(tagsFromString(formData.tags), 300)

    const selectedPlan = formData.plan ? ALL_PLANS.find(plan => plan.label === formData.plan) : undefined

    return (
        <Modal
            position="center"
            // We pass a noop to onDismiss so clicks on the backdrop don't close the modal accidentally, loosing all the data.
            onDismiss={noop}
            aria-labelledby={labelId}
            className={styles.modal}
        >
            <H3 className="flex-shrink-0" id={labelId}>
                Generate new Sourcegraph license
                {hasAcknowledgedInfo && (
                    <Button variant="icon" onClick={() => setHasAcknowledgedInfo(false)}>
                        <Tooltip content="Show additional information on the license issuance process">
                            <Icon aria-label="Show handbook page" svgPath={mdiChatQuestionOutline} />
                        </Tooltip>
                    </Button>
                )}
            </H3>

            {error && <ErrorAlert error={error} />}

            {!hasAcknowledgedInfo && (
                <>
                    <Alert variant="danger" className="flex-shrink-0">
                        Each subscription must map to exactly ONE Sourcegraph instance.{' '}
                        <strong>
                            DO NOT create licenses used by multiple Sourcegraph instances within a single subscription
                        </strong>{' '}
                        - instead, create a NEW subscription with the appropriate Salesforce subscription ID and a
                        relevant display name.
                    </Alert>

                    <Alert variant="info" className="flex-shrink-0">
                        Existing licenses can be re-linked to a new subscription by reaching out to{' '}
                        <Link rel="noopener" target="_blank" to="https://sourcegraph.slack.com/archives/C05GJPTSZCZ">
                            #discuss-core-services
                        </Link>
                        .
                    </Alert>

                    <Button variant="secondary" onClick={() => setHasAcknowledgedInfo(true)}>
                        Acknowledge information
                    </Button>
                </>
            )}

            {hasAcknowledgedInfo && (
                <>
                    <div
                        className={classNames(
                            styles.modalContainer,
                            'site-admin-generate-product-license-for-subscription-form'
                        )}
                    >
                        <Form onSubmit={onSubmit}>
                            <Input
                                id="site-admin-create-product-subscription-page__salesforce_op_id_input"
                                label="Message"
                                description="Enter a message about the creation of this license."
                                type="text"
                                required={true}
                                disabled={isLoading}
                                value={formData.message}
                                onChange={onMessageChange}
                            />
                            <Select
                                id="site-admin-create-product-subscription-page__plan_select"
                                label="Plan"
                                disabled={isLoading}
                                value={formData.plan}
                                onChange={onPlanChange}
                                description="Select the plan the license is for."
                                className="mb-2"
                                required={true}
                                message={
                                    <>
                                        {formData.plan !== '' &&
                                            ALL_PLANS.find(plan => plan.label === formData.plan)?.deprecated && (
                                                <span className="text-danger">
                                                    This plan has been deprecated. Only issue a new license for this
                                                    plan if you got approval to do so.
                                                </span>
                                            )}
                                    </>
                                }
                            >
                                <option value="" disabled={true}>
                                    Select a plan
                                </option>
                                {ALL_PLANS.filter(plan => !plan.deprecated && !plan.stopIssuance).map(plan => (
                                    <option key={plan.label} value={plan.label}>
                                        {plan.name}
                                    </option>
                                ))}
                                <option value="" disabled={true}>
                                    Deprecated plans
                                </option>
                                {ALL_PLANS.filter(plan => plan.deprecated && !plan.stopIssuance).map(plan => (
                                    <option key={plan.label} value={plan.label}>
                                        {plan.name}
                                    </option>
                                ))}
                            </Select>

                            {selectedPlan && (
                                <>
                                    <div className="form-group mb-2">
                                        <Checkbox
                                            id="productSubscription__trial"
                                            aria-label="Is trial"
                                            label="This license is for a trial"
                                            disabled={isLoading}
                                            checked={formData.trial}
                                            onChange={onIsTrialChange}
                                        />
                                    </div>

                                    <Input
                                        id="site-admin-create-product-subscription-page__salesforce_op_id_input"
                                        label="Salesforce Opportunity ID"
                                        description="Enter the corresponding Opportunity ID from Salesforce. This is VERY important to provide for all subscriptions used by customers. It cannot be changed after a license has been created."
                                        type="text"
                                        disabled={isLoading}
                                        error={sfOpportunityIDError}
                                        value={formData.salesforceOpportunityID}
                                        onChange={event => {
                                            onSFOpportunityIDChange(event)
                                            const { value } = event.target
                                            if (!value) {
                                                setSFOpportunityIDError(undefined)
                                                return
                                            }

                                            if (!value.startsWith('006')) {
                                                setSFOpportunityIDError(
                                                    'Salesforce opportunity ID must start with "006"'
                                                )
                                                return
                                            }
                                            if (value.length < 17) {
                                                setSFOpportunityIDError(
                                                    'Salesforce opportunity ID must be longer than 17 characters'
                                                )
                                                return
                                            }

                                            // No problems
                                            setSFOpportunityIDError(undefined)
                                        }}
                                    />

                                    <Input
                                        type="number"
                                        label="Users"
                                        min={1}
                                        id="site-admin-create-product-subscription-page__userCount"
                                        disabled={!selectedPlan || isLoading}
                                        value={Number(formData.userCount)}
                                        onChange={onUserCountChange}
                                        description="The maximum number of users permitted on this license."
                                        className="w-100"
                                        message={
                                            <>
                                                {formData.trueUp && (
                                                    <Text className="mb-0">
                                                        With true up enabled, the maximum user count is not enforced and
                                                        additional users can join the instance. Bill for those users
                                                        separately.
                                                    </Text>
                                                )}
                                            </>
                                        }
                                    />

                                    <Label>Additional Options</Label>
                                    {selectedPlan.additionalTags?.find(tag => tag.tagValue === TAG_TRUEUP.tagValue) && (
                                        <div className="form-group mb-2">
                                            <Checkbox
                                                id="productSubscription__trueup"
                                                aria-label="Whether true up is allowed"
                                                label="TrueUp"
                                                checked={formData.trueUp}
                                                onChange={onTrueUpChange}
                                                message={TAG_TRUEUP.description}
                                            />
                                        </div>
                                    )}
                                    {selectedPlan.additionalTags?.find(
                                        tag => tag.tagValue === TAG_AIR_GAPPED.tagValue
                                    ) && (
                                        <div className="form-group mb-2">
                                            <Checkbox
                                                id="productSubscription__airgapped"
                                                aria-label="Whether the instance may be air gapped"
                                                label="Allow air gapped"
                                                checked={formData.airGapped}
                                                onChange={onAirGappedChange}
                                                message={TAG_AIR_GAPPED.description}
                                            />
                                        </div>
                                    )}
                                    {selectedPlan.additionalTags?.find(
                                        tag => tag.tagValue === TAG_BATCH_CHANGES.tagValue
                                    ) && (
                                        <div className="form-group mb-2">
                                            <Checkbox
                                                id="productSubscription__batches"
                                                aria-label="Whether the instance may use Batch Changes unrestrictedly"
                                                label="Allow unrestricted Batch Changes"
                                                checked={formData.batchChanges}
                                                onChange={onBatchChangesChange}
                                                message={TAG_BATCH_CHANGES.description}
                                            />
                                        </div>
                                    )}
                                    {selectedPlan.additionalTags?.find(
                                        tag => tag.tagValue === TAG_CODE_INSIGHTS.tagValue
                                    ) && (
                                        <div className="form-group mb-2">
                                            <Checkbox
                                                id="productSubscription__codeinsights"
                                                aria-label="Whether the instance may use Code Insights"
                                                label="Allow Code Insights"
                                                checked={formData.codeInsights}
                                                onChange={onCodeInsightsChange}
                                                message={TAG_CODE_INSIGHTS.description}
                                            />
                                        </div>
                                    )}
                                    {selectedPlan.additionalTags?.find(
                                        tag => tag.tagValue === TAG_DISABLE_TELEMETRY_EXPORT.tagValue
                                    ) && (
                                        <div className="form-group mb-2">
                                            <Checkbox
                                                id="productSubscription__disableTelemetry"
                                                aria-label={TAG_DISABLE_TELEMETRY_EXPORT.description}
                                                label="Allow disable telemetry export"
                                                checked={formData.disableTelemetry || formData.airGapped}
                                                disabled={formData.airGapped}
                                                onChange={onDisableTelemetryChange}
                                                message={
                                                    formData.airGapped
                                                        ? 'Always possible for air gapped instances'
                                                        : TAG_DISABLE_TELEMETRY_EXPORT.description
                                                }
                                            />
                                        </div>
                                    )}
                                    <Input
                                        type="date"
                                        description="When this license expires. Sourcegraph will disable beyond this date. Usually the end date of the contract."
                                        label="Expires At"
                                        id="site-admin-create-product-subscription-page__expiresAt"
                                        min={formatDateForInput(addDaysAndRoundToEndOfDayInUTC(1))}
                                        max={formatDateForInput(addDaysAndRoundToEndOfDayInUTC(2000))}
                                        value={formatDateForInput(formData.expiresAt)}
                                        onChange={onExpiresAtChange}
                                        required={true}
                                        message={
                                            <>
                                                {formData.expiresAt !== null && (
                                                    <ExpirationDate
                                                        date={formData.expiresAt}
                                                        showTime={true}
                                                        showRelative={true}
                                                        showPrefix={true}
                                                    />
                                                )}
                                                <Text>
                                                    Set to{' '}
                                                    {DURATION_LINKS.map(({ label, days }) => (
                                                        <Button
                                                            key={days}
                                                            className="p-0 mr-2"
                                                            onClick={() => setValidDays(days)}
                                                            variant="link"
                                                            size="sm"
                                                        >
                                                            {label}
                                                        </Button>
                                                    ))}
                                                </Text>
                                            </>
                                        }
                                    />
                                    <Collapsible titleAtStart={true} title="Additional Information">
                                        <Input
                                            type="text"
                                            label="Tags"
                                            id="site-admin-create-product-subscription-page__tags"
                                            disabled={isLoading}
                                            value={formData.tags}
                                            onChange={onTagsChange}
                                            list="known-tags"
                                            description="Comma separated list of tags. Tags restrict a license."
                                            message={
                                                <Text className="text-danger">
                                                    Note that specifying tags manually is no longer required and the
                                                    form should handle all options. For example, the{' '}
                                                    <span className="text-monospace">customer:</span> tag is
                                                    automatically added on license creation.
                                                    <br />
                                                    Only use this if you know what you're doing!
                                                    <br />
                                                    All final tags are displayed at the end of the form as well.
                                                </Text>
                                            }
                                            className="mt-2"
                                        />
                                        <datalist id="known-tags">
                                            {selectedPlan?.additionalTags?.map(tag => (
                                                <option key={tag.tagValue} value={tag.tagValue}>
                                                    {tag.name}
                                                </option>
                                            ))}
                                        </datalist>
                                    </Collapsible>
                                    <hr className="mb-3" />
                                    <H4>Final License Details</H4>
                                    <Text>
                                        Please double check that the license tags and user count are correct before
                                        generating the license. The license cannot be modified once generated.
                                    </Text>
                                    <div>
                                        {hasUnknownTags(tags) && <UnknownTagWarning className="mb-2" />}
                                        <Text>
                                            <ProductLicenseTags tags={getTagsFromFormData(formData)} />
                                        </Text>
                                    </div>
                                </>
                            )}

                            <div className="d-flex justify-content-end">
                                <Button
                                    disabled={isLoading}
                                    className="mr-2"
                                    onClick={onCancel}
                                    outline={true}
                                    variant="secondary"
                                >
                                    Cancel
                                </Button>
                                <LoaderButton
                                    type="submit"
                                    disabled={isLoading || !selectedPlan || !!sfOpportunityIDError}
                                    variant="primary"
                                    loading={isLoading}
                                    alwaysShowLabel={true}
                                    label="Generate key"
                                />
                            </div>
                        </Form>
                    </div>
                </>
            )}
        </Modal>
    )
}

/**
 * Adds 1 day to the current date, then rounds it up to midnight in UTC. This is a
 * generous interpretation of "valid for N days" to avoid confusion over timezones or "will it
 * expire at the beginning of the day or at the end of the day?"
 */
const addDaysAndRoundToEndOfDayInUTC = (amount: number): UTCDate => {
    const now = new UTCDate(UTCDate.now())
    return endOfDay(addDays(now, amount))
}

const formatDateForInput = (date: Date): string => date.toISOString().slice(0, 10)
