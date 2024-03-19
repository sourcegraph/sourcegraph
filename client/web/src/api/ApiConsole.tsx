import * as React from 'react'

// type only
import type * as _graphiqlModule from 'graphiql'
import type * as H from 'history'
import { useNavigate, useLocation, type NavigateFunction } from 'react-router-dom'
import { from as fromPromise, Subject, Subscription } from 'rxjs'
import { catchError, debounceTime } from 'rxjs/operators'

import { asError, type ErrorLike, isErrorLike, logger } from '@sourcegraph/common'
import type { TelemetryV2Props } from '@sourcegraph/shared/src/telemetry'
import { LoadingSpinner, ErrorAlert } from '@sourcegraph/wildcard'

import { PageTitle } from '../components/PageTitle'
import { eventLogger } from '../tracking/eventLogger'

import { ApiConsoleToolbar } from './ApiConsoleToolbar'

import styles from './ApiConsole.module.scss'

const defaultQuery = `# Type queries here, with completion, validation, and hovers.
#
# Here's an example query to get you started:

query {
  currentUser {
    username
  }
  repositories(first: 1) {
    nodes {
      name
    }
  }
}
`

interface Props extends TelemetryV2Props {}

interface InnerProps extends TelemetryV2Props {
    location: H.Location
    navigate: NavigateFunction
}

interface State {
    /** The dynamically imported graphiql module, undefined while loading. */
    graphiqlOrError?: typeof _graphiqlModule | ErrorLike

    /** The current URL parameters. Only used to update the shareable URL link */
    parameters: Parameters
}

/** Represents URL parameters stored in the location.hash */
interface Parameters {
    /** The GraphQL query string. */
    query?: string

    /** The GraphQL variables as a JSON encoded string. */
    variables?: string

    /** The GraphQL operation name. */
    operationName?: string
}

export const ApiConsole: React.FC<Props> = ({ telemetryRecorder }) => {
    const navigate = useNavigate()
    const location = useLocation()

    return <ApiConsoleInner location={location} navigate={navigate} telemetryRecorder={telemetryRecorder} />
}

/**
 * Component to show the GraphQL API console.
 */
class ApiConsoleInner extends React.PureComponent<InnerProps, State> {
    public state: State = { parameters: {} }

    private updates = new Subject<Parameters>()
    private subscriptions = new Subscription()
    /** The initial URL parameters decoded from the location hash. */
    /** This is used to programmatically set the initial editor state. */
    private initialParameters: Parameters

    constructor(props: InnerProps) {
        super(props)

        // Parse the location.hash JSON to get URL parameters.
        //
        // Precaution: Use URL fragment (not querystring) to avoid leaking sensitive querystrings in
        // HTTP referer headers.
        const parameters = JSON.parse(decodeURIComponent(props.location.hash.slice('#'.length)) || '{}') as Parameters

        // If variables were provided, try to format them.
        if (parameters.variables) {
            try {
                parameters.variables = JSON.stringify(JSON.parse(parameters.variables), null, 2)
            } catch {
                // The parse error can be safely ignored because the string
                // will still be forwarded to the UI where the user will see
                // invalid JSON errors in the GraphiQL editor.
            }
        }
        this.initialParameters = parameters
        this.state = { parameters }
    }

    public componentDidMount(): void {
        eventLogger.logViewEvent('ApiConsole')
        this.props.telemetryRecorder.recordEvent('api-console', 'view')

        // Update the browser URL bar when query/variables/operation name are
        // changed so that the page can be easily shared.
        this.subscriptions.add(
            this.updates
                .pipe(debounceTime(500))
                .subscribe(data => this.props.navigate({ ...location, hash: encodeURIComponent(JSON.stringify(data)) }))
        )

        this.subscriptions.add(
            fromPromise(import('graphiql'))
                .pipe(
                    catchError(error => {
                        logger.error(error)
                        return [asError(error)]
                    })
                )
                .subscribe(graphiqlOrError => {
                    this.setState({ graphiqlOrError })
                })
        )

        // Ensure that the Doc Explorer page opens by default the first time a
        // user opens the API console.
        window.localStorage.setItem(
            'graphiql:docExplorerOpen',
            window.localStorage.getItem('graphiql:docExplorerOpen') || 'true'
        )
    }

    public componentWillUnmount(): void {
        this.subscriptions.unsubscribe()
    }

    public render(): JSX.Element | null {
        return (
            <div className={styles.apiConsole}>
                <PageTitle title="API console" />
                {this.state.graphiqlOrError === undefined ? (
                    <span className={styles.loader}>
                        <LoadingSpinner /> Loading…
                    </span>
                ) : isErrorLike(this.state.graphiqlOrError) ? (
                    <ErrorAlert prefix="Error loading API console" error={this.state.graphiqlOrError} />
                ) : (
                    this.renderGraphiQL()
                )}
            </div>
        )
    }

    /**
     * Renders the API console once GraphiQL has loaded. This method should
     * only be invoked once this.state.graphiqlOrError is loaded successfully.
     */
    private renderGraphiQL = (): JSX.Element => {
        if (!this.state.graphiqlOrError || isErrorLike(this.state.graphiqlOrError)) {
            throw new Error('renderGraphiQL called illegally')
        }
        const GraphiQL = this.state.graphiqlOrError.default
        return (
            <>
                <GraphiQL
                    query={this.initialParameters.query}
                    variables={this.initialParameters.variables}
                    operationName={this.initialParameters.operationName}
                    onEditQuery={this.onEditQuery}
                    onEditVariables={this.onEditVariables}
                    onEditOperationName={this.onEditOperationName}
                    fetcher={this.fetcher}
                    defaultQuery={defaultQuery}
                    editorTheme="sourcegraph"
                >
                    <GraphiQL.Logo>GraphQL API console</GraphiQL.Logo>
                    <ApiConsoleToolbar />
                </GraphiQL>
            </>
        )
    }

    // Update state.parameters when query/variables/operation name are changed
    // so that we can update the browser URL.

    private onEditQuery = (newQuery?: string): void =>
        this.updateStateParameters(parameters => ({ ...parameters, query: newQuery }))

    private onEditVariables = (newVariables: string): void =>
        this.updateStateParameters(parameters => ({ ...parameters, variables: newVariables }))

    private onEditOperationName = (newOperationName: string): void =>
        this.updateStateParameters(parameters => ({ ...parameters, operationName: newOperationName }))

    private updateStateParameters(update: (parameters: Parameters) => Parameters): void {
        this.setState(
            state => ({ ...state, parameters: update(state.parameters) }),
            () => this.updates.next(this.state.parameters)
        )
    }

    private fetcher: _graphiqlModule.Fetcher = async graphQLParameters => {
        const headers = new Headers({
            'x-requested-with': 'Sourcegraph GraphQL Explorer',
        })
        const searchParameters = new URLSearchParams(this.props.location.search)
        if (searchParameters.get('trace') === '1') {
            headers.set('x-sourcegraph-should-trace', 'true')
        }
        for (const feature of searchParameters.getAll('feat')) {
            headers.append('x-sourcegraph-override-feature', feature)
        }
        const response = await fetch('/.api/graphql', {
            method: 'POST',
            body: JSON.stringify(graphQLParameters),
            credentials: 'include',
            headers,
        })
        const responseBody = await response.text()
        try {
            return JSON.parse(responseBody)
        } catch {
            return responseBody
        }
    }
}
