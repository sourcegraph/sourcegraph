import * as React from 'react'

import classNames from 'classnames'
import type { NavigateFunction } from 'react-router-dom'

import type { SearchPatternTypeProps } from '@sourcegraph/shared/src/search'
import { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { Button, Modal, Select, H3, Form } from '@sourcegraph/wildcard'

import type { AuthenticatedUser } from '../auth'

import styles from './SavedSearchModal.module.scss'

interface Props extends SearchPatternTypeProps, TelemetryV2Props {
    authenticatedUser: Pick<AuthenticatedUser, 'organizations' | 'username'> | null
    query?: string
    onDidCancel: () => void
    navigate: NavigateFunction
}

enum UserOrOrg {
    User = 'User',
    Org = 'Org',
}

interface State {
    saveLocation: UserOrOrg
    organization?: string
}

const MODAL_LABEL_ID = 'saved-search-modal-id'

export class SavedSearchModal extends React.Component<Props, State> {
    constructor(props: Props) {
        super(props)
        this.state = {
            saveLocation: UserOrOrg.User,
        }
        props.telemetryRecorder.recordEvent('search.resultsInfoBar.savedQueriesModal', 'view')
    }

    private onLocationChange = (event: React.ChangeEvent<HTMLSelectElement>): void => {
        const locationType = event.target.value
        this.setState({ saveLocation: locationType as UserOrOrg })
    }

    private onOrganizationChange = (event: React.ChangeEvent<HTMLSelectElement>): void => {
        const orgName = event.target.value
        this.setState({ organization: orgName })
    }

    public render(): JSX.Element | null {
        return (
            this.props.authenticatedUser && (
                <Modal
                    aria-labelledby={MODAL_LABEL_ID}
                    className={styles.savedSearchModalForm}
                    onDismiss={this.props.onDidCancel}
                    data-testid="saved-search-modal"
                >
                    <Form onSubmit={this.onSubmit} className="test-saved-search-modal">
                        <H3 id={MODAL_LABEL_ID}>Save search query to: </H3>

                        <Select aria-label="" onChange={this.onLocationChange} selectClassName={styles.select}>
                            <option value={UserOrOrg.User}>User</option>
                            {this.props.authenticatedUser.organizations &&
                                this.props.authenticatedUser.organizations.nodes.length > 0 && (
                                    <option value={UserOrOrg.Org}>Organization</option>
                                )}
                        </Select>
                        {this.props.authenticatedUser.organizations &&
                            this.props.authenticatedUser.organizations.nodes.length > 0 &&
                            this.state.saveLocation === UserOrOrg.Org && (
                                <Select
                                    aria-label=""
                                    onChange={this.onOrganizationChange}
                                    placeholder="Select an organization"
                                    selectClassName={styles.select}
                                >
                                    <option value="" disabled={true} selected={true}>
                                        Select an organization
                                    </option>
                                    {this.props.authenticatedUser.organizations.nodes.map(org => (
                                        <option value={org.name} key={org.name}>
                                            {org.displayName ? org.displayName : org.name}
                                        </option>
                                    ))}
                                </Select>
                            )}

                        <Button
                            type="submit"
                            disabled={this.state.saveLocation === UserOrOrg.Org && !this.state.organization}
                            className={classNames(styles.button, 'test-saved-search-modal-save-button')}
                            variant="primary"
                        >
                            Save query
                        </Button>
                    </Form>
                </Modal>
            )
        )
    }

    private onSubmit = (): void => {
        if (this.props.query && this.props.authenticatedUser) {
            const encodedQuery = encodeURIComponent(this.props.query)
            this.props.navigate(
                this.state.saveLocation.toLowerCase() === 'user'
                    ? `/users/${this.props.authenticatedUser.username}/searches/new?query=${encodedQuery}&patternType=${this.props.patternType}`
                    : `/organizations/${this.state.organization!}/searches/new?query=${encodedQuery}&patternType=${
                          this.props.patternType
                      }`
            )
        }
    }
}
