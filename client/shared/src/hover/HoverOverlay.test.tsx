import { render } from '@testing-library/react'
import * as H from 'history'
import { NEVER } from 'rxjs'

import { subtypeOf } from '@sourcegraph/common'
import { MarkupKind } from '@sourcegraph/extension-api-classes'

import { NOOP_TELEMETRY_SERVICE } from '../telemetry/telemetryService'

import { HoverOverlay, HoverOverlayProps } from './HoverOverlay'

describe('HoverOverlay', () => {
    const NOOP_PLATFORM_CONTEXT = { settings: NEVER }
    const history = H.createMemoryHistory({ keyLength: 0 })
    const commonProps = subtypeOf<HoverOverlayProps>()({
        location: history.location,
        telemetryService: NOOP_TELEMETRY_SERVICE,
        platformContext: NOOP_PLATFORM_CONTEXT,
        hoveredToken: { repoName: 'r', commitID: 'c', revision: 'v', filePath: 'f', line: 1, character: 2 },
        overlayPosition: { left: 0, top: 0 },
        isLightTheme: false,
    })

    test('actions and hover undefined', () => {
        expect(render(<HoverOverlay {...commonProps} />).asFragment()).toMatchSnapshot()
    })

    test('actions loading', () => {
        expect(render(<HoverOverlay {...commonProps} actionsOrError="loading" />).asFragment()).toMatchSnapshot()
    })

    test('hover loading', () => {
        expect(render(<HoverOverlay {...commonProps} hoverOrError="loading" />).asFragment()).toMatchSnapshot()
    })

    test('actions and hover loading', () => {
        expect(
            render(<HoverOverlay {...commonProps} actionsOrError="loading" hoverOrError="loading" />).asFragment()
        ).toMatchSnapshot()
    })

    test('actions empty', () => {
        const component = render(<HoverOverlay {...commonProps} actionsOrError={[]} />)
        expect(component.asFragment()).toMatchSnapshot()
    })

    test('hover empty', () => {
        expect(render(<HoverOverlay {...commonProps} hoverOrError={null} />).asFragment()).toMatchSnapshot()
    })

    test('actions and hover empty', () => {
        expect(
            render(<HoverOverlay {...commonProps} actionsOrError={[]} hoverOrError={null} />).asFragment()
        ).toMatchSnapshot()
    })

    test('actions present', () => {
        expect(
            render(
                <HoverOverlay
                    {...commonProps}
                    actionsOrError={[{ action: { id: 'a', command: 'c', title: 'Some title' }, active: true }]}
                />
            ).asFragment()
        ).toMatchSnapshot()
    })

    test('hover present', () => {
        expect(
            render(
                <HoverOverlay
                    {...commonProps}
                    hoverOrError={{ contents: [{ kind: MarkupKind.Markdown, value: 'v' }] }}
                />
            ).asFragment()
        ).toMatchSnapshot()
    })

    test('multiple hovers present', () => {
        expect(
            render(
                <HoverOverlay
                    {...commonProps}
                    hoverOrError={{
                        contents: [
                            { kind: MarkupKind.Markdown, value: 'v' },
                            { kind: MarkupKind.Markdown, value: 'v2' },
                        ],
                    }}
                />
            ).asFragment()
        ).toMatchSnapshot()
    })

    test('actions and hover present', () => {
        expect(
            render(
                <HoverOverlay
                    {...commonProps}
                    actionsOrError={[{ action: { id: 'a', command: 'c' }, active: true }]}
                    hoverOrError={{ contents: [{ kind: MarkupKind.Markdown, value: 'v' }] }}
                />
            ).asFragment()
        ).toMatchSnapshot()
    })

    test('actions present, hover loading', () => {
        expect(
            render(
                <HoverOverlay
                    {...commonProps}
                    actionsOrError={[{ action: { id: 'a', command: 'c' }, active: true }]}
                    hoverOrError="loading"
                />
            ).asFragment()
        ).toMatchSnapshot()
    })

    test('hover present, actions loading', () => {
        expect(
            render(
                <HoverOverlay
                    {...commonProps}
                    actionsOrError="loading"
                    hoverOrError={{ contents: [{ kind: MarkupKind.Markdown, value: 'v' }] }}
                />
            ).asFragment()
        ).toMatchSnapshot()
    })

    test('actions error', () => {
        expect(
            render(<HoverOverlay {...commonProps} actionsOrError={{ message: 'm', name: 'c' }} />).asFragment()
        ).toMatchSnapshot()
    })

    test('hover error', () => {
        expect(
            render(<HoverOverlay {...commonProps} hoverOrError={{ message: 'm', name: 'c' }} />).asFragment()
        ).toMatchSnapshot()
    })

    test('actions and hover error', () => {
        expect(
            render(
                <HoverOverlay
                    {...commonProps}
                    actionsOrError={{ message: 'm1', name: 'c1' }}
                    hoverOrError={{ message: 'm2', name: 'c2' }}
                />
            ).asFragment()
        ).toMatchSnapshot()
    })

    test('actions error, hover present', () => {
        expect(
            render(
                <HoverOverlay
                    {...commonProps}
                    actionsOrError={{ message: 'm', name: 'c' }}
                    hoverOrError={{ contents: [{ kind: MarkupKind.Markdown, value: 'v' }] }}
                />
            ).asFragment()
        ).toMatchSnapshot()
    })

    test('hover error, actions present', () => {
        expect(
            render(
                <HoverOverlay
                    {...commonProps}
                    actionsOrError={[{ action: { id: 'a', command: 'c' }, active: true }]}
                    hoverOrError={{ message: 'm', name: 'c' }}
                />
            ).asFragment()
        ).toMatchSnapshot()
    })
})
