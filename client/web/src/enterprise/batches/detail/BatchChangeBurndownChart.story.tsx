import { storiesOf } from '@storybook/react'
import { select } from '@storybook/addon-knobs'
import React from 'react'
import { BatchChangeBurndownChart } from './BatchChangeBurndownChart'
import { of } from 'rxjs'
import { EnterpriseWebStory } from '../../components/EnterpriseWebStory'

const { add } = storiesOf('web/batches/BurndownChart', module).addDecorator(story => (
    <div className="p-3 container web-content">{story()}</div>
))

add('All states', () => (
    <EnterpriseWebStory>
        {props => (
            <BatchChangeBurndownChart
                {...props}
                batchChangeID="123"
                queryChangesetCountsOverTime={() =>
                    of(
                        [
                            {
                                __typename: 'ChangesetCounts',
                                date: '2019-11-13T12:00:00Z',
                                closed: 0,
                                merged: 0,
                                openApproved: 0,
                                openChangesRequested: 0,
                                openPending: 0,
                                total: 10,
                                draft: 10,
                                open: 0,
                            },
                            {
                                __typename: 'ChangesetCounts',
                                date: '2019-11-14T12:00:00Z',
                                closed: 0,
                                merged: 0,
                                openApproved: 0,
                                openChangesRequested: 0,
                                openPending: 2,
                                total: 10,
                                draft: 5,
                                open: 5,
                            },
                            {
                                __typename: 'ChangesetCounts',
                                date: '2019-11-15T12:00:00Z',
                                closed: 0,
                                merged: 1,
                                openApproved: 1,
                                openChangesRequested: 0,
                                openPending: 3,
                                total: 10,
                                draft: 0,
                                open: 8,
                            },
                            {
                                __typename: 'ChangesetCounts',
                                date: '2019-11-16T12:00:00Z',
                                closed: 1,
                                merged: 1,
                                openApproved: 1,
                                openChangesRequested: 0,
                                openPending: 3,
                                total: 10,
                                draft: 0,
                                open: 7,
                            },
                            {
                                __typename: 'ChangesetCounts',
                                date: '2019-11-17T12:00:00Z',
                                closed: 1,
                                merged: 2,
                                openApproved: 1,
                                openChangesRequested: 0,
                                openPending: 5,
                                total: 10,
                                draft: 0,
                                open: 6,
                            },
                            {
                                __typename: 'ChangesetCounts',
                                date: '2019-11-18T12:00:00Z',
                                closed: 2,
                                merged: 4,
                                openApproved: 0,
                                openChangesRequested: 2,
                                openPending: 2,
                                total: 10,
                                draft: 0,
                                open: 4,
                            },
                            {
                                __typename: 'ChangesetCounts',
                                date: '2019-11-19T12:00:00Z',
                                closed: 2,
                                merged: 4,
                                openApproved: 0,
                                openChangesRequested: 2,
                                openPending: 2,
                                total: 10,
                                draft: 0,
                                open: 4,
                            },
                            {
                                __typename: 'ChangesetCounts',
                                date: '2019-11-20T12:00:00Z',
                                closed: 2,
                                merged: 4,
                                openApproved: 0,
                                openChangesRequested: 2,
                                openPending: 2,
                                total: 10,
                                draft: 0,
                                open: 4,
                            },
                            {
                                __typename: 'ChangesetCounts',
                                date: '2019-11-21T12:00:00Z',
                                closed: 2,
                                merged: 5,
                                openApproved: 3,
                                openChangesRequested: 0,
                                openPending: 0,
                                total: 10,
                                draft: 0,
                                open: 3,
                            },
                            {
                                __typename: 'ChangesetCounts',
                                date: '2019-11-22T12:00:00Z',
                                closed: 2,
                                merged: 8,
                                openApproved: 0,
                                openChangesRequested: 0,
                                openPending: 0,
                                total: 10,
                                draft: 0,
                                open: 0,
                            },
                        ].slice(0, select('Days of data', [0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10], 10))
                    )
                }
            />
        )}
    </EnterpriseWebStory>
))

add('No data', () => (
    <EnterpriseWebStory>
        {props => (
            <BatchChangeBurndownChart {...props} batchChangeID="123" queryChangesetCountsOverTime={() => of([])} />
        )}
    </EnterpriseWebStory>
))
