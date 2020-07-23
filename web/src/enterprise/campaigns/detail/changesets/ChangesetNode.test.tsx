import * as H from 'history'
import React from 'react'
import { ChangesetNode } from './ChangesetNode'
import { Subject } from 'rxjs'
import { Changeset } from '../../../../../../shared/src/graphql/schema'
import { shallow } from 'enzyme'

describe('ChangesetNode', () => {
    const history = H.createMemoryHistory({ keyLength: 0 })
    const location = H.createLocation('/campaigns')
    test('renders ExternalChangeset', () => {
        expect(
            shallow(
                <ChangesetNode
                    isLightTheme={true}
                    history={history}
                    location={location}
                    viewerCanAdminister={false}
                    campaignUpdates={new Subject<void>()}
                    node={{ __typename: 'ExternalChangeset' } as Changeset}
                />
            )
        ).toMatchSnapshot()
    })
    test('renders HiddenExternalChangeset', () => {
        expect(
            shallow(
                <ChangesetNode
                    isLightTheme={true}
                    history={history}
                    location={location}
                    viewerCanAdminister={false}
                    campaignUpdates={new Subject<void>()}
                    node={{ __typename: 'HiddenExternalChangeset' } as Changeset}
                />
            )
        ).toMatchSnapshot()
    })
})
