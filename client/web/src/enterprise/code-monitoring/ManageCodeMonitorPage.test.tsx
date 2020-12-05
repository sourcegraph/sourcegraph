import * as React from 'react'
import * as H from 'history'
import { AuthenticatedUser } from '../../auth'
import sinon from 'sinon'
import { mount } from 'enzyme'
import { ManageCodeMonitorPage } from './ManageCodeMonitorPage'
import { mockCodeMonitor, mockCodeMonitorNodes } from './testing/util'
import { of } from 'rxjs'

describe('ManageCodeMonitorPage', () => {
    const mockUser = {
        id: 'userID',
        username: 'username',
        email: 'user@me.com',
        siteAdmin: true,
    } as AuthenticatedUser

    const history = H.createMemoryHistory()
    const props = {
        history,
        location: history.location,
        authenticatedUser: mockUser,
        breadcrumbs: [{ depth: 0, breadcrumb: null }],
        setBreadcrumb: sinon.spy(),
        useBreadcrumb: sinon.spy(),
        fetchUserCodeMonitors: sinon.spy(),
        updateCodeMonitor: sinon.spy(),
        fetchCodeMonitor: sinon.spy((id: string) => of(mockCodeMonitor)),
        match: {
            params: { id: 'test-id' },
            isExact: true,
            path: history.location.pathname,
            url: 'https://sourcegraph.com',
        },
    }
    test('Form is pre-loaded with data', () => {
        const component = mount(<ManageCodeMonitorPage {...props} />)
        const nameInput = component.find('.test-name-input')
        expect(nameInput.length).toBe(1)
        const nameValue = nameInput.getDOMNode().getAttribute('value')
        expect(nameValue).toBe('Test code monitor')
        const currentQueryValue = component.find('.test-existing-query')
        const currentActionEmailValue = component.find('.test-existing-action-email')
        expect(currentQueryValue.getDOMNode().innerHTML).toBe('test')
        expect(currentActionEmailValue.getDOMNode().innerHTML).toBe('user@me.com')
        component.unmount()
    })

    test('Updating the form calls the update request', () => {
        const updateSpy = sinon.spy()
        const component = mount(<ManageCodeMonitorPage {...props} updateCodeMonitor={updateSpy} />)
        const nameInput = component.find('.test-name-input')
        const nameValue = nameInput.getDOMNode().getAttribute('value')
        expect(nameValue).toBe('Test code monitor')
        nameInput.simulate('change', { target: { value: 'Test updated' } })
        const submitButton = component.find('.test-submit-monitor')
        submitButton.simulate('click')
        expect(updateSpy.calledOnce)
        component.unmount()
    })
})
