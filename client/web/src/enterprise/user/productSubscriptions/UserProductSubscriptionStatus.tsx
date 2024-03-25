import React, { useState, useCallback } from 'react'

import { mdiKey, mdiInformation } from '@mdi/js'

import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Button, CardFooter, Link, Icon, Code, H3 } from '@sourcegraph/wildcard'

import { CopyableText } from '../../../components/CopyableText'
import { formatUserCount, mailtoSales } from '../../../productSubscription/helpers'
import { ExpirationDate } from '../../productSubscription/ExpirationDate'
import { LicenseGenerationKeyWarning } from '../../productSubscription/LicenseGenerationKeyWarning'
import { ProductCertificate } from '../../productSubscription/ProductCertificate'

interface Props extends TelemetryV2Props {
    subscriptionName: string
    productNameWithBrand: string
    userCount: number
    expiresAt: Date | number
    licenseKey: string | null
    className?: string
}

/**
 * Displays a certificate with information about and status for a user's product subscription.
 */
export const UserProductSubscriptionStatus: React.FunctionComponent<React.PropsWithChildren<Props>> = ({
    subscriptionName,
    productNameWithBrand,
    userCount,
    expiresAt,
    licenseKey,
    className,
    telemetryRecorder,
}) => {
    const [showLicenseKey, setShowLicenseKey] = useState(false)

    const toggleShowLicenseKey = useCallback((): void => {
        telemetryRecorder.recordEvent('settings.userSubscription.licenseKey', showLicenseKey ? 'hide' : 'reveal')
        setShowLicenseKey(!showLicenseKey)
    }, [telemetryRecorder, showLicenseKey])

    return (
        <ProductCertificate
            title={productNameWithBrand}
            className={className}
            subtitle={
                <>
                    {formatUserCount(userCount, true)} license,{' '}
                    <ExpirationDate date={expiresAt} showRelative={true} showPrefix={true} lowercase={true} />
                </>
            }
            footer={
                <>
                    <CardFooter className="d-flex align-items-center justify-content-between flex-wrap">
                        <Button className="mr-4 my-1" onClick={toggleShowLicenseKey} variant="primary">
                            <Icon aria-hidden={true} svgPath={mdiKey} /> {showLicenseKey ? 'Hide' : 'Reveal'} license
                            key
                        </Button>
                        <div className="flex-fill" />
                        <div className="my-1" />
                    </CardFooter>
                    {showLicenseKey && (
                        <CardFooter>
                            <H3>License key</H3>
                            {licenseKey ? (
                                <>
                                    <CopyableText text={licenseKey} className="d-block" />
                                    <small className="mt-2 d-flex align-items-center">
                                        <Icon aria-hidden={true} className="mr-1" svgPath={mdiInformation} />{' '}
                                        <span>
                                            Use this license key as the <Code weight="bold">licenseKey</Code> property
                                            value in Sourcegraph site configuration.
                                        </span>
                                    </small>
                                    <LicenseGenerationKeyWarning className="mb-0 mt-1" />
                                </>
                            ) : (
                                <div className="text-muted">
                                    No license key found.{' '}
                                    <Link
                                        to={mailtoSales({
                                            subject: `No license key for subscription ${subscriptionName}`,
                                        })}
                                    >
                                        Contact sales
                                    </Link>{' '}
                                    for help.
                                </div>
                            )}
                        </CardFooter>
                    )}
                </>
            }
        />
    )
}
