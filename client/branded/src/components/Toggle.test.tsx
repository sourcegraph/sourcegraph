import React from 'react'
import { Toggle } from './Toggle'
import { mount } from 'enzyme'
import sinon from 'sinon'

describe('Toggle', () => {
    test('value is false', () => {
        expect(mount(<Toggle value={false} />)).toMatchSnapshot()
    })

    test('value is true', () => {
        expect(mount(<Toggle value={true} />)).toMatchSnapshot()
    })

    test('disabled', () => {
        const onToggle = sinon.spy(() => undefined)
        const component = mount(<Toggle onToggle={onToggle} disabled={true} />)

        component.find('.toggle').simulate('click')
        sinon.assert.notCalled(onToggle)
        expect(component).toMatchSnapshot()
    })

    test('className', () => expect(mount(<Toggle className="c" />)).toMatchSnapshot())

    test('aria', () =>
        expect(
            mount(<Toggle aria-describedby="test-id-1" aria-labelledby="test-id-2" aria-label="test toggle" />)
        ).toMatchSnapshot())
})
