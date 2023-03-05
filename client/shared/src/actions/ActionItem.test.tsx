import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import * as H from 'history'
import { NEVER } from 'rxjs'

import { assertAriaEnabled } from '@sourcegraph/testing'
import { renderWithBrandedContext } from '@sourcegraph/wildcard/src/testing'

import { NOOP_TELEMETRY_SERVICE } from '../telemetry/telemetryService'
import { createBarrier } from '../testing/testHelpers'

import { ActionItem } from './ActionItem'

jest.mock('mdi-react/OpenInNewIcon', () => 'OpenInNewIcon')

describe('ActionItem', () => {
    const NOOP_EXTENSIONS_CONTROLLER = { executeCommand: () => Promise.resolve(undefined) }
    const NOOP_PLATFORM_CONTEXT = { settings: NEVER }
    const history = H.createMemoryHistory()

    test('non-actionItem variant', () => {
        const component = render(
            <ActionItem
                active={true}
                action={{ id: 'c', command: 'c', title: 't', description: 'd', iconURL: 'u' }}
                telemetryService={NOOP_TELEMETRY_SERVICE}
                location={history.location}
                extensionsController={NOOP_EXTENSIONS_CONTROLLER}
                platformContext={NOOP_PLATFORM_CONTEXT}
            />
        )
        expect(component.asFragment()).toMatchSnapshot()
    })

    test('actionItem variant', () => {
        const component = render(
            <ActionItem
                active={true}
                action={{ id: 'c', command: 'c', title: 't', description: 'd', iconURL: 'u' }}
                telemetryService={NOOP_TELEMETRY_SERVICE}
                variant="actionItem"
                location={history.location}
                extensionsController={NOOP_EXTENSIONS_CONTROLLER}
                platformContext={NOOP_PLATFORM_CONTEXT}
            />
        )
        expect(component.asFragment()).toMatchSnapshot()
    })

    test('noop command', () => {
        const component = render(
            <ActionItem
                active={true}
                action={{ id: 'c', title: 't', description: 'd', iconURL: 'u' }}
                telemetryService={NOOP_TELEMETRY_SERVICE}
                location={history.location}
                extensionsController={NOOP_EXTENSIONS_CONTROLLER}
                platformContext={NOOP_PLATFORM_CONTEXT}
            />
        )
        expect(component.asFragment()).toMatchSnapshot()
    })

    test('pressed toggle actionItem', () => {
        const component = render(
            <ActionItem
                active={true}
                action={{ id: 'a', command: 'c', actionItem: { pressed: true, label: 'b' } }}
                telemetryService={NOOP_TELEMETRY_SERVICE}
                variant="actionItem"
                location={history.location}
                extensionsController={NOOP_EXTENSIONS_CONTROLLER}
                platformContext={NOOP_PLATFORM_CONTEXT}
            />
        )
        expect(component.asFragment()).toMatchSnapshot()
    })

    test('non-pressed actionItem', () => {
        const component = render(
            <ActionItem
                active={true}
                action={{ id: 'a', command: 'c', actionItem: { pressed: false, label: 'b' } }}
                telemetryService={NOOP_TELEMETRY_SERVICE}
                variant="actionItem"
                location={history.location}
                extensionsController={NOOP_EXTENSIONS_CONTROLLER}
                platformContext={NOOP_PLATFORM_CONTEXT}
            />
        )
        expect(component.asFragment()).toMatchSnapshot()
    })

    test('title element', () => {
        const component = render(
            <ActionItem
                active={true}
                action={{ id: 'c', command: 'c', title: 't', description: 'd', iconURL: 'u' }}
                telemetryService={NOOP_TELEMETRY_SERVICE}
                variant="actionItem"
                title={<span>t2</span>}
                location={history.location}
                extensionsController={NOOP_EXTENSIONS_CONTROLLER}
                platformContext={NOOP_PLATFORM_CONTEXT}
            />
        )
        expect(component.asFragment()).toMatchSnapshot()
    })

    test('run command', async () => {
        const { wait, done } = createBarrier()

        const { container, asFragment } = render(
            <ActionItem
                active={true}
                action={{ id: 'c', command: 'c', title: 't', description: 'd', iconURL: 'u' }}
                telemetryService={NOOP_TELEMETRY_SERVICE}
                variant="actionItem"
                disabledDuringExecution={true}
                location={history.location}
                extensionsController={{ ...NOOP_EXTENSIONS_CONTROLLER, executeCommand: () => wait }}
                platformContext={NOOP_PLATFORM_CONTEXT}
            />
        )

        // Run command and wait for execution to finish.
        userEvent.click(container)
        expect(asFragment()).toMatchSnapshot()

        // Finish execution. (Use setTimeout to wait for the executeCommand resolution to result in the setState
        // call.)
        done()
        await new Promise<void>(resolve => setTimeout(resolve))
        expect(asFragment()).toMatchSnapshot()
    })

    test('run command with showLoadingSpinnerDuringExecution', async () => {
        const { wait, done } = createBarrier()

        const { asFragment } = render(
            <ActionItem
                active={true}
                action={{ id: 'c', command: 'c', title: 't', description: 'd', iconURL: 'u' }}
                telemetryService={NOOP_TELEMETRY_SERVICE}
                variant="actionItem"
                showLoadingSpinnerDuringExecution={true}
                location={history.location}
                extensionsController={{ ...NOOP_EXTENSIONS_CONTROLLER, executeCommand: () => wait }}
                platformContext={NOOP_PLATFORM_CONTEXT}
            />
        )

        // Run command and wait for execution to finish.
        userEvent.click(screen.getByRole('button'))

        await waitFor(() => {
            expect(screen.getByTestId('action-item-spinner')).toBeInTheDocument()
        })

        expect(asFragment()).toMatchSnapshot()

        // Finish execution. (Use setTimeout to wait for the executeCommand resolution to result in the setState
        // call.)
        done()
        await new Promise<void>(resolve => setTimeout(resolve))
        expect(asFragment()).toMatchSnapshot()
    })

    test('run command with error', async () => {
        const { asFragment } = render(
            <ActionItem
                active={true}
                action={{ id: 'c', command: 'c', title: 't', description: 'd', iconURL: 'u' }}
                telemetryService={NOOP_TELEMETRY_SERVICE}
                variant="actionItem"
                disabledDuringExecution={true}
                location={history.location}
                extensionsController={{
                    ...NOOP_EXTENSIONS_CONTROLLER,
                    executeCommand: () => Promise.reject(new Error('x')),
                }}
                platformContext={NOOP_PLATFORM_CONTEXT}
            />
        )

        // Run command (which will reject with an error). (Use setTimeout to wait for the executeCommand resolution
        // to result in the setState call.)
        userEvent.click(screen.getByRole('button'))

        // we should wait for the button to be enabled again after got errors. Otherwise, it will be flaky
        await waitFor(() => assertAriaEnabled(screen.getByLabelText('d')))

        expect(asFragment()).toMatchSnapshot()
    })

    test('run command with error with showInlineError', async () => {
        const { asFragment } = render(
            <ActionItem
                active={true}
                action={{ id: 'c', command: 'c', title: 't', description: 'd', iconURL: 'u', category: 'g' }}
                telemetryService={NOOP_TELEMETRY_SERVICE}
                variant="actionItem"
                showInlineError={true}
                location={history.location}
                extensionsController={{
                    ...NOOP_EXTENSIONS_CONTROLLER,
                    executeCommand: () => Promise.reject(new Error('x')),
                }}
                platformContext={NOOP_PLATFORM_CONTEXT}
            />
        )

        // Run command (which will reject with an error). (Use setTimeout to wait for the executeCommand resolution
        // to result in the setState call.)
        userEvent.click(screen.getByRole('button'))

        await waitFor(() => expect(screen.getByLabelText('Error: x')).toBeInTheDocument())

        expect(asFragment()).toMatchSnapshot()
    })

    describe('"open" command', () => {
        it('renders as link', () => {
            jsdom.reconfigure({ url: 'https://example.com/foo' })

            const { asFragment } = renderWithBrandedContext(
                <ActionItem
                    active={true}
                    action={{ id: 'c', command: 'open', commandArguments: ['https://example.com/bar'], title: 't' }}
                    telemetryService={NOOP_TELEMETRY_SERVICE}
                    location={history.location}
                    extensionsController={NOOP_EXTENSIONS_CONTROLLER}
                    platformContext={NOOP_PLATFORM_CONTEXT}
                />
            )
            expect(asFragment()).toMatchSnapshot()
        })

        it('renders as link with icon and opens a new tab for a different origin', () => {
            jsdom.reconfigure({ url: 'https://example.com/foo' })

            const { asFragment } = renderWithBrandedContext(
                <ActionItem
                    active={true}
                    action={{ id: 'c', command: 'open', commandArguments: ['https://other.com/foo'], title: 't' }}
                    telemetryService={NOOP_TELEMETRY_SERVICE}
                    location={history.location}
                    extensionsController={NOOP_EXTENSIONS_CONTROLLER}
                    platformContext={NOOP_PLATFORM_CONTEXT}
                />
            )
            expect(asFragment()).toMatchSnapshot()
        })

        it('renders as link that opens in a new tab, but without icon for a different origin as the alt action and a primary action defined', () => {
            jsdom.reconfigure({ url: 'https://example.com/foo' })

            const { asFragment } = renderWithBrandedContext(
                <ActionItem
                    active={true}
                    action={{ id: 'c1', command: 'whatever', title: 'primary' }}
                    altAction={{ id: 'c2', command: 'open', commandArguments: ['https://other.com/foo'], title: 'alt' }}
                    telemetryService={NOOP_TELEMETRY_SERVICE}
                    location={history.location}
                    extensionsController={NOOP_EXTENSIONS_CONTROLLER}
                    platformContext={NOOP_PLATFORM_CONTEXT}
                />
            )
            expect(asFragment()).toMatchSnapshot()
        })
    })
})
