import * as H from 'history'
import React, { useMemo, useState } from 'react'
import { useObservable } from '../../../../../shared/src/util/useObservable'
import { PageTitle } from '../../../components/PageTitle'
import {
    fetchCampaignSpecById as _fetchCampaignSpecById,
    queryChangesetSpecs,
    queryChangesetSpecFileDiffs,
} from './backend'
import { ErrorAlert } from '../../../components/alerts'
import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import { CampaignHeader } from '../detail/CampaignHeader'
import { ChangesetSpecList } from './ChangesetSpecList'
import { ThemeProps } from '../../../../../shared/src/theme'
import { Link } from '../../../../../shared/src/components/Link'
import { Timestamp } from '../../../components/time/Timestamp'
import { Markdown } from '../../../../../shared/src/components/Markdown'
import { renderMarkdown } from '../../../../../shared/src/util/markdown'
import { CreateUpdateCampaignAlert } from './CreateUpdateCampaignAlert'
import AlertCircleIcon from 'mdi-react/AlertCircleIcon'
import { HeroPage } from '../../../components/HeroPage'

export interface CampaignApplyPageProps extends ThemeProps {
    specID: string
    history: H.History
    location: H.Location

    /** Used for testing. */
    fetchCampaignSpecById?: typeof _fetchCampaignSpecById
    /** Used for testing. */
    queryChangesetSpecs?: typeof queryChangesetSpecs
    /** Used for testing. */
    queryChangesetSpecFileDiffs?: typeof queryChangesetSpecFileDiffs
}

export const CampaignApplyPage: React.FunctionComponent<CampaignApplyPageProps> = ({
    specID,
    history,
    location,
    isLightTheme,
    fetchCampaignSpecById = _fetchCampaignSpecById,
    queryChangesetSpecs,
    queryChangesetSpecFileDiffs,
}) => {
    const [isLoading, setIsLoading] = useState<boolean | Error>(false)
    const spec = useObservable(useMemo(() => fetchCampaignSpecById(specID), [specID, fetchCampaignSpecById]))
    if (spec === undefined) {
        return <LoadingSpinner />
    }
    if (spec === null) {
        return <ErrorAlert history={history} error={new Error('Campaign spec not found')} />
    }

    if (spec === undefined) {
        return (
            <div className="text-center">
                <LoadingSpinner className="icon-inline mx-auto my-4" />
            </div>
        )
    }
    if (spec === null) {
        return <HeroPage icon={AlertCircleIcon} title="Campaign spec not found" />
    }

    return (
        <>
            <PageTitle title="Apply campaign spec" />
            <div className="mb-3">
                <CampaignHeader name={spec.description.name} namespace={spec.namespace} className="d-inline-block" />
                <span className="text-muted ml-3">
                    Uploaded <Timestamp date={spec.createdAt} /> by{' '}
                    {spec.creator && <Link to={spec.creator.url}>{spec.creator.username}</Link>}
                    {!spec.creator && <strong>deleted user</strong>}
                </span>
            </div>
            <CreateUpdateCampaignAlert
                history={history}
                specID={spec.id}
                campaign={spec.appliesToCampaign}
                isLoading={isLoading}
                setIsLoading={setIsLoading}
                viewerCanAdminister={spec.viewerCanAdminister}
            />
            <Markdown
                dangerousInnerHTML={renderMarkdown(spec.description.description || '_No description_')}
                history={history}
                className="mb-3"
            />
            <ChangesetSpecList
                campaignSpecID={specID}
                history={history}
                location={location}
                isLightTheme={isLightTheme}
                queryChangesetSpecs={queryChangesetSpecs}
                queryChangesetSpecFileDiffs={queryChangesetSpecFileDiffs}
            />
            <CreateUpdateCampaignAlert
                history={history}
                specID={spec.id}
                campaign={spec.appliesToCampaign}
                isLoading={isLoading}
                setIsLoading={setIsLoading}
                viewerCanAdminister={spec.viewerCanAdminister}
            />
        </>
    )
}
