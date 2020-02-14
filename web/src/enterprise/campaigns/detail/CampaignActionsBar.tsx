import React from 'react'
import * as GQL from '../../../../../shared/src/graphql/schema'
import { CampaignsIcon } from '../icons'
import classNames from 'classnames'
import { Link } from '../../../../../shared/src/components/Link'
import { CampaignTitleField } from './form/CampaignTitleField'
import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import { CloseDeleteCampaignPrompt } from './form/CloseDeleteCampaignPrompt'

interface Props {
    mode: string
    previewingCampaignPlan: boolean

    campaign?: Pick<GQL.ICampaign, '__typename' | 'name' | 'closedAt' | 'viewerCanAdminister' | 'publishedAt'> & {
        changesets: Pick<GQL.ICampaign['changesets'], 'totalCount'>
        status: Pick<GQL.ICampaign['status'], 'state'>
    }

    onClose: (closeChangesets: boolean) => Promise<void>
    onDelete: (closeChangesets: boolean) => Promise<void>
    onEdit: React.MouseEventHandler
    onNameChange: React.Dispatch<React.SetStateAction<string>>
}

export const CampaignActionsBar: React.FunctionComponent<Props> = ({
    campaign,
    previewingCampaignPlan,
    mode,
    onClose,
    onDelete,
    onEdit,
    onNameChange,
}) => {
    const showActionButtons = campaign && !previewingCampaignPlan && campaign.viewerCanAdminister
    const showSpinner = mode === 'saving' || mode === 'deleting' || mode === 'closing'
    const editingCampaign = mode === 'editing' || mode === 'saving'

    const campaignProcessing = campaign ? campaign.status.state === GQL.BackgroundProcessState.PROCESSING : false
    const actionsDisabled = mode === 'deleting' || mode === 'closing' || campaignProcessing

    return (
        <div className="d-flex mb-2">
            <h2 className="m-0">
                <CampaignsIcon
                    className={classNames(
                        'icon-inline mr-2',
                        !campaign ? 'text-muted' : campaign.closedAt ? 'text-danger' : 'text-success'
                    )}
                />
                <span>
                    <Link to="/campaigns">Campaigns</Link>
                </span>
                <span className="text-muted d-inline-block mx-2">/</span>
                {editingCampaign ? (
                    <CampaignTitleField
                        className="w-auto d-inline-block e2e-campaign-title"
                        value={campaign?.name}
                        disabled={mode === 'saving'}
                    />
                ) : (
                    <span>{campaign?.name}</span>
                )}
            </h2>
            <span className="flex-grow-1 d-flex justify-content-end align-items-center">
                {showSpinner && <LoadingSpinner className="mr-2" />}
                {campaign &&
                    showActionButtons &&
                    (editingCampaign ? (
                        <>
                            <button type="submit" className="btn btn-primary mr-1" disabled={mode === 'saving'}>
                                Save
                            </button>
                            <button type="reset" className="btn btn-secondary" disabled={mode === 'saving'}>
                                Cancel
                            </button>
                        </>
                    ) : (
                        <>
                            <button
                                type="button"
                                id="e2e-campaign-edit"
                                className="btn btn-secondary mr-1"
                                onClick={onEdit}
                                disabled={actionsDisabled}
                            >
                                Edit
                            </button>
                            {!campaign.closedAt && (
                                <CloseDeleteCampaignPrompt
                                    disabled={actionsDisabled}
                                    disabledTooltip="Cannot close while campaign is being created"
                                    message={
                                        <p>
                                            Close campaign <strong>{campaign.name}</strong>?
                                        </p>
                                    }
                                    changesetsCount={campaign.changesets.totalCount}
                                    buttonText="Close"
                                    onButtonClick={onClose}
                                    buttonClassName="btn-secondary mr-1"
                                />
                            )}
                            <CloseDeleteCampaignPrompt
                                disabled={actionsDisabled}
                                disabledTooltip="Cannot delete while campaign is being created"
                                message={
                                    <p>
                                        Delete campaign <strong>{campaign.name}</strong>?
                                    </p>
                                }
                                changesetsCount={campaign.changesets.totalCount}
                                buttonText="Delete"
                                onButtonClick={onDelete}
                                buttonClassName="btn-danger"
                            />
                        </>
                    ))}
            </span>
        </div>
    )
}
