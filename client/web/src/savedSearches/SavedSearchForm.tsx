import * as React from 'react'
import { RouteComponentProps } from 'react-router'
import { Omit } from 'utility-types'
import { Form } from '../../../branded/src/components/Form'
import { NamespaceProps } from '../namespaces'
import { ErrorAlert } from '../components/alerts'
import { AuthenticatedUser } from '../auth'
import { Scalars } from '../../../shared/src/graphql-operations'

export interface SavedQueryFields {
    id: Scalars['ID']
    description: string
    query: string
    notify: boolean
    notifySlack: boolean
    slackWebhookURL: string | null
}

interface Props extends RouteComponentProps<{}>, NamespaceProps {
    authenticatedUser: AuthenticatedUser | null
    defaultValues?: Partial<SavedQueryFields>
    title?: string
    submitLabel: string
    onSubmit: (fields: Omit<SavedQueryFields, 'id'>) => void
    loading: boolean
    error?: any
}

interface State {
    values: Omit<SavedQueryFields, 'id'>
}

export class SavedSearchForm extends React.Component<Props, State> {
    constructor(props: Props) {
        super(props)

        const { description = '', query = '', notify = false, notifySlack = false, slackWebhookURL = '' } =
            props.defaultValues || {}

        this.state = {
            values: {
                description,
                query,
                notify,
                notifySlack,
                slackWebhookURL,
            },
        }
    }

    /**
     * Returns an input change handler that updates the SavedQueryFields in the component's state
     *
     * @param key The key of saved query fields that a change of this input should update
     */
    private createInputChangeHandler(key: keyof SavedQueryFields): React.FormEventHandler<HTMLInputElement> {
        return event => {
            const { value, checked, type } = event.currentTarget
            this.setState(state => ({
                values: {
                    ...state.values,
                    [key]: type === 'checkbox' ? checked : value,
                },
            }))
        }
    }

    private handleSubmit = (event: React.FormEvent<HTMLFormElement>): void => {
        event.preventDefault()
        this.props.onSubmit(this.state.values)
    }

    public render(): JSX.Element | null {
        const {
            values: { query, description, notify, notifySlack, slackWebhookURL },
        } = this.state

        return (
            <div className="saved-search-form">
                <div className="saved-search-form__header">
                    <h2>{this.props.title}</h2>
                    <div>Get notifications when there are new results for specific search queries</div>
                </div>
                <Form onSubmit={this.handleSubmit}>
                    <div className="saved-search-form__input">
                        <label className="saved-search-form__label">Description:</label>
                        <input
                            type="text"
                            name="description"
                            className="form-control test-saved-search-form-input-description"
                            placeholder="Description"
                            required={true}
                            value={description}
                            onChange={this.createInputChangeHandler('description')}
                        />
                    </div>
                    <div className="saved-search-form__input">
                        <label className="saved-search-form__label">Query:</label>
                        <input
                            type="text"
                            name="query"
                            className="form-control test-saved-search-form-input-query"
                            placeholder="Query"
                            required={true}
                            value={query}
                            onChange={this.createInputChangeHandler('query')}
                        />
                    </div>
                    <div className="saved-search-form__input">
                        <label className="saved-search-form__label">Email notifications:</label>
                        <div>
                            <label>
                                <input
                                    type="checkbox"
                                    name="Notify owner"
                                    className="saved-search-form__checkbox"
                                    defaultChecked={notify}
                                    onChange={this.createInputChangeHandler('notify')}
                                />{' '}
                                <span>
                                    {this.props.namespace.__typename === 'Org'
                                        ? 'Send email notifications to all members of this organization'
                                        : this.props.namespace.__typename === 'User'
                                        ? 'Send email notifications to my email'
                                        : 'Email notifications'}
                                </span>
                            </label>
                        </div>
                    </div>
                    {notifySlack && slackWebhookURL && (
                        <div className="saved-search-form__input">
                            <label className="saved-search-form__label">Slack notifications:</label>
                            <label>
                                <input
                                    type="text"
                                    name="Slack webhook URL"
                                    className="form-control"
                                    value={slackWebhookURL}
                                    disabled={true}
                                    onChange={this.createInputChangeHandler('slackWebhookURL')}
                                />
                            </label>
                            <label className="small">
                                Slack webhooks are deprecated and will be removed in a future Sourcegraph version.
                            </label>
                        </div>
                    )}
                    {this.isUnsupportedNotifyQuery(this.state.values) && (
                        <div className="alert alert-warning mb-3">
                            <strong>Warning:</strong> non-commit searches do not currently support notifications.
                            Consider adding <code>type:diff</code> or <code>type:commit</code> to your query.
                        </div>
                    )}
                    {notify && !window.context.emailEnabled && !this.isUnsupportedNotifyQuery(this.state.values) && (
                        <div className="alert alert-warning mb-3">
                            <strong>Warning:</strong> Sending emails is not currently configured on this Sourcegraph
                            server.{' '}
                            {this.props.authenticatedUser?.siteAdmin
                                ? 'Use the email.smtp site configuration setting to enable sending emails.'
                                : 'Contact your server admin for more information.'}
                        </div>
                    )}
                    <button
                        type="submit"
                        disabled={this.props.loading}
                        className="btn btn-primary saved-search-form__submit-button test-saved-search-form-submit-button"
                    >
                        {this.props.submitLabel}
                    </button>
                    {this.props.error && !this.props.loading && (
                        <ErrorAlert className="mb-3" error={this.props.error} />
                    )}
                </Form>
            </div>
        )
    }
    /**
     * Tells if the query is unsupported for sending notifications.
     */
    private isUnsupportedNotifyQuery(savedQuery: Omit<SavedQueryFields, 'id'>): boolean {
        const notifying = savedQuery.notify || savedQuery.notifySlack
        return notifying && !savedQuery.query.includes('type:diff') && !savedQuery.query.includes('type:commit')
    }
}
