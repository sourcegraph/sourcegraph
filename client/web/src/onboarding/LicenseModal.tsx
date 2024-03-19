import { type FC, useCallback, useEffect, useState, type FormEvent } from 'react'

import type { ApolloQueryResult } from '@apollo/client'
import classnames from 'classnames'
import { format, formatDistanceToNow } from 'date-fns'
import * as jsonc from 'jsonc-parser'
import { useNavigate } from 'react-router-dom'
import { useDebouncedCallback } from 'use-debounce'

import { useIsLightTheme } from '@sourcegraph/shared/src/theme'
import { Alert, Button, Form, H3, H2, Input, Label, LoadingSpinner, Modal, Text } from '@sourcegraph/wildcard'

import { BrandLogo } from '../components/branding/BrandLogo'
import type { SiteConfigResult } from '../graphql-operations'

import { DEBOUNCE_DELAY_MS, DEFAULT_FORMAT_OPTIONS, DEFAULT_LICENSE_KEY_INFO, MIN_INPUT_LENGTH } from './constants'
import { useUpdateLicenseKeyMutation } from './graphql-hooks'
import type { LicenseInfo, LicenseKeyInfo } from './types'

import styles from './LicenseModal.module.scss'

interface LicenseKeyModalProps {
    id: number
    config: string
    licenseKey: LicenseInfo
    refetch?: () => Promise<ApolloQueryResult<SiteConfigResult>>
    onHandleLicenseCheck: (
        newValue: boolean | ((previousValue: boolean | undefined) => boolean | undefined) | undefined
    ) => void
}

export const LicenseKeyModal: FC<LicenseKeyModalProps> = ({
    licenseKey,
    onHandleLicenseCheck,
    config,
    id,
    refetch,
}) => {
    const navigate = useNavigate()

    const [isValid, setIsValid] = useState(false)
    const [licenseKeyInput, setLicenseKeyInput] = useState(licenseKey.key || '')
    const [initialUpdateLicenseKey, setInitialUpdateLicenseKey] = useState(false)

    const onDismiss = useCallback(() => onHandleLicenseCheck(true), [onHandleLicenseCheck])

    const [updateLicenseKey, { error, loading }] = useUpdateLicenseKeyMutation()

    const updateLicense = useCallback(
        async (value: string): Promise<void> => {
            const editFns = [
                (contents: string) => jsonc.modify(contents, ['licenseKey'], value, DEFAULT_FORMAT_OPTIONS),
            ]
            let input = config
            for (const editFunc of editFns) {
                input = jsonc.applyEdits(config, editFunc(config))
            }
            setInitialUpdateLicenseKey(true)
            await updateLicenseKey({ variables: { lastID: id, input } })
            setIsValid(true)
            if (refetch) {
                await refetch()
            }
        },
        [config, id, refetch, updateLicenseKey]
    )
    const onDebouncedChange = useDebouncedCallback(async (value: string) => {
        setLicenseKeyInput(value)
        if (value.length > MIN_INPUT_LENGTH) {
            await updateLicense(value)
        }
    }, DEBOUNCE_DELAY_MS)

    useEffect(() => {
        if (licenseKeyInput.length > MIN_INPUT_LENGTH && !initialUpdateLicenseKey) {
            updateLicense(licenseKeyInput).catch(() => setIsValid(false))
        }
    }, [updateLicense, licenseKeyInput, initialUpdateLicenseKey])

    const onSubmit = useCallback(
        async (event?: FormEvent<HTMLFormElement>) => {
            event?.preventDefault()
            onHandleLicenseCheck(true)
            navigate('site-admin/configuration')
            if (refetch) {
                await refetch()
            }
        },
        [navigate, refetch, onHandleLicenseCheck]
    )

    return (
        <Modal
            className={styles.modal}
            containerClassName={styles.modalContainer}
            onDismiss={() => onHandleLicenseCheck(true)}
            aria-labelledby="license-key"
        >
            <H3 className="m-0 pb-4">Upgrade your license</H3>
            <Text className="m-0 pb-3">Enter your license key to start your enterprise set up:</Text>
            {error && <Alert variant="danger">License key not recognized. Please try again.</Alert>}
            {}
            <Form onSubmit={onSubmit}>
                <Label htmlFor="license-key">License key</Label>
                <Input
                    type="text"
                    name="license-key"
                    value={licenseKeyInput}
                    className="pb-4"
                    disabled={isValid}
                    onChange={({ target: { value } }) => onDebouncedChange(value)}
                />
                <LicenseKey isValid={isValid} licenseInfo={licenseKey} />
                <div className="d-flex justify-content-end">
                    <Button
                        data-testid="license-dismiss-button"
                        className="mr-2"
                        onClick={onDismiss}
                        outline={true}
                        variant="secondary"
                        disabled={isValid}
                    >
                        Skip for now
                    </Button>
                    <Button className={styles.submit} type="submit" disabled={!isValid} variant="primary">
                        Upgrade and start set up
                        {loading && (
                            <div data-testid="action-item-spinner">
                                <LoadingSpinner inline={false} />
                            </div>
                        )}
                    </Button>
                </div>
            </Form>
        </Modal>
    )
}

interface LicenseKeyProps {
    isValid: boolean
    licenseInfo: LicenseInfo
}

const LicenseKey: FC<LicenseKeyProps> = ({ isValid, licenseInfo }): JSX.Element => {
    const isLightTheme = useIsLightTheme()
    const { title, type, description, logo } = getLicenseKeyInfo(isValid, licenseInfo, isLightTheme)

    return (
        <div className={classnames(styles.keyWrapper, 'p-4 mb-4')}>
            <div className={styles.keyContainer}>
                <div className={styles.logoContainer}>{logo}</div>
                <div>
                    <Text className="mb-1">{title}</Text>
                    <H2 className="mb-1">{type}</H2>
                    <Text className="mb-0">{description}</Text>
                </div>
            </div>
        </div>
    )
}

function getLicenseKeyInfo(isValid: boolean, licenseInfo: LicenseInfo, isLightTheme: boolean): LicenseKeyInfo {
    if (!isValid) {
        return DEFAULT_LICENSE_KEY_INFO
    }

    const { tags = [], userCount = 10, expiresAt = '' } = licenseInfo

    const expiration = new Date(expiresAt)
    const validDate = format(expiration, 'y-LL-dd')
    const remaining = formatDistanceToNow(expiration, { addSuffix: false })
    const type = tags.includes('dev') ? 'Sourcegraph Enterprise (dev-only)' : 'Sourcegraph Enterprise'
    const title = 'License'
    const numUsers = userCount > 0 ? userCount : 1
    const logo = <BrandLogo isLightTheme={isLightTheme} variant="symbol" />

    const description = `${numUsers}-user license, valid until ${validDate} (${remaining} remaining)`
    return { title, type, description, logo }
}
