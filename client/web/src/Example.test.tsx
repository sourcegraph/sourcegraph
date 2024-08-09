import { render, fireEvent } from '@testing-library/react'
import { mount, shallow } from 'enzyme'
import React from 'react'
import renderer from 'react-test-renderer'

interface ExampleObject {
    exampleKey: string
}

interface ExampleComponentProps {
    exampleBool: boolean
    exampleString: string
    exampleFunction: () => void
    exampleObject: ExampleObject
}

/**
 * Used to test snapshots driven through components
 */
const OtherComponent: React.FunctionComponent<ExampleComponentProps> = props => (
    <span>Example Key: {props.exampleObject.exampleKey}</span>
)

/**
 * Used to test prop handling and data access
 */
const ExampleComponent: React.FunctionComponent<ExampleComponentProps> = props => (
    <button onClick={props.exampleFunction} disabled={props.exampleBool} aria-label={props.exampleString}>
        <OtherComponent {...props} />
    </button>
)

// eslint-disable-next-line ban/ban
const mockFunction = jest.fn()

const mockProps: ExampleComponentProps = {
    exampleBool: true,
    exampleString: 'Label for button',
    // eslint-disable-next-line ban/ban
    exampleFunction: mockFunction,
    exampleObject: {
        exampleKey: 'Hello world',
    },
} as const

describe('Comparing React testing libraries', () => {
    beforeEach(() => {
        mockFunction.mockReset()
    })

    describe('snapshotting', () => {
        test('React test renderer', () => {
            const component = renderer.create(<ExampleComponent {...mockProps} />)
            expect(component.toJSON()).toMatchInlineSnapshot(`
                <button
                  aria-label="Label for button"
                  disabled={true}
                  onClick={[MockFunction]}
                >
                  <span>
                    Example Key: 
                    Hello world
                  </span>
                </button>
            `)
        })

        test('enzyme', () => {
            const component = mount(<ExampleComponent {...mockProps} />)
            expect(component).toMatchInlineSnapshot(`
                <ExampleComponent
                  exampleBool={true}
                  exampleFunction={[MockFunction]}
                  exampleObject={
                    Object {
                      "exampleKey": "Hello world",
                    }
                  }
                  exampleString="Label for button"
                >
                  <button
                    aria-label="Label for button"
                    disabled={true}
                    onClick={[MockFunction]}
                  >
                    <OtherComponent
                      exampleBool={true}
                      exampleFunction={[MockFunction]}
                      exampleObject={
                        Object {
                          "exampleKey": "Hello world",
                        }
                      }
                      exampleString="Label for button"
                    >
                      <span>
                        Example Key: 
                        Hello world
                      </span>
                    </OtherComponent>
                  </button>
                </ExampleComponent>
            `)

            const component2 = shallow(<ExampleComponent {...mockProps} />)
            expect(component2).toMatchInlineSnapshot(`
                <button
                  aria-label="Label for button"
                  disabled={true}
                  onClick={[MockFunction]}
                >
                  <OtherComponent
                    exampleBool={true}
                    exampleFunction={[MockFunction]}
                    exampleObject={
                      Object {
                        "exampleKey": "Hello world",
                      }
                    }
                    exampleString="Label for button"
                  />
                </button>
            `)
        })

        test('React testing library', () => {
            const { container } = render(<ExampleComponent {...mockProps} />)
            expect(container).toMatchInlineSnapshot(`
                <div>
                  <button
                    aria-label="Label for button"
                    disabled=""
                  >
                    <span>
                      Example Key: 
                      Hello world
                    </span>
                  </button>
                </div>
            `)
        })
    })

    describe('getting and clicking an element', () => {
        test('react test renderer', () => {
            const component = renderer.create(<ExampleComponent {...mockProps} />).root
            const element = component.findByType('button')
            element.props.onClick()
            expect(mockFunction).toHaveBeenCalledTimes(1)
        })

        test('enzyme', () => {
            const component = mount(<ExampleComponent {...mockProps} />)
            const element = component.find('button')

            // eslint-disable-next-line @typescript-eslint/ban-ts-comment
            // @ts-ignore
            element.props().onClick({})
            // Note: element.simulate('click') actually is better here, but is actually being deprecated 🤯 https://github.com/enzymejs/enzyme/issues/2173#issuecomment-511677655

            expect(mockFunction).toHaveBeenCalledTimes(1)
        })

        test('react testing library', () => {
            const { getByLabelText } = render(<ExampleComponent {...mockProps} />)
            const element = getByLabelText('Label for button')

            fireEvent.click(element)
            // Fails because button is disabled
            expect(mockFunction).toHaveBeenCalledTimes(1)
        })
    })
})
