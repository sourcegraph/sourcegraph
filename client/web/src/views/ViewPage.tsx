import React, { useMemo } from 'react'
import { PageTitle } from '../components/PageTitle'
import * as H from 'history'
import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import { useObservable } from '../../../shared/src/util/useObservable'
import { ViewContentProps, ViewContent } from './ViewContent'
import { Observable } from 'rxjs'
import { View } from 'sourcegraph'

interface Props extends Omit<ViewContentProps, 'viewContent' | 'containerClassName'> {
    viewID: string
    extraPath: string
    getViewForID: (id: string, queryParameters: Record<string, string>) => Observable<View | null | undefined>

    location: H.Location
    history: H.History
}

/**
 * A page that displays a single view (contributed by an extension) as a standalone page.
 */
export const ViewPage: React.FunctionComponent<Props> = ({ viewID, extraPath, location, getViewForID, ...props }) => {
    const queryParameters = useMemo<Record<string, string>>(
        () => ({ ...Object.fromEntries(new URLSearchParams(location.search).entries()), extraPath }),
        [extraPath, location.search]
    )

    const view = useObservable(useMemo(() => getViewForID(viewID, queryParameters), [queryParameters, viewID]))

    if (view === undefined) {
        return <LoadingSpinner className="icon-inline" />
    }

    if (view === null) {
        return (
            <div className="alert alert-danger">
                View not found: <code>{viewID}</code>
            </div>
        )
    }

    return (
        <div>
            <PageTitle title={view.title || 'View'} />
            {view.title && <h1>{view.title}</h1>}
            <ViewContent viewID={viewID} viewContent={view.content} location={location} {...props} />
        </div>
    )
}
