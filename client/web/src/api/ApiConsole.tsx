import { LoadingSpinner } from '@sourcegraph/react-loading-spinner'
import * as _graphiqlModule from 'graphiql' // type only
import * as H from 'history'
import * as React from 'react'
import { Link } from 'react-router-dom'
import { from as fromPromise, Subject, Subscription } from 'rxjs'
import { catchError, debounceTime } from 'rxjs/operators'
import { asError, ErrorLike, isErrorLike } from '../../../shared/src/util/errors'
import { eventLogger } from '../tracking/eventLogger'
import { ErrorAlert } from '../components/alerts'
import { PageTitle } from '../components/PageTitle'

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

interface Props {
    location: H.Location
    history: H.History
}

interface State {
    /** The dynamically imported graphiql module, undefined while loading. */
    graphiqlOrError?: typeof _graphiqlModule | ErrorLike

    /** The URL parameters decoded from the location hash. */
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

/**
 * Component to show the GraphQL API console.
 */
export class ApiConsole extends React.PureComponent<Props, State> {
    public state: State = { parameters: {} }

    private updates = new Subject<Parameters>()
    private subscriptions = new Subscription()
    private graphiQLRef: _graphiqlModule.default | null = null

    constructor(props: Props) {
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
        this.state = { parameters }
    }

    public componentDidMount(): void {
        eventLogger.logViewEvent('ApiConsole')

        // Update the browser URL bar when query/variables/operation name are
        // changed so that the page can be easily shared.
        this.subscriptions.add(
            this.updates
                .pipe(debounceTime(500))
                .subscribe(data => this.props.history.replace({ hash: encodeURIComponent(JSON.stringify(data)) }))
        )

        this.subscriptions.add(
            fromPromise(import('graphiql'))
                .pipe(
                    catchError(error => {
                        console.error(error)
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
            <div className="api-console">
                <PageTitle title="API console" />
                {this.state.graphiqlOrError === undefined ? (
                    <span className="api-console__loader">
                        <LoadingSpinner className="icon-inline" /> Loading…
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
                    query={this.state.parameters.query}
                    variables={this.state.parameters.variables}
                    operationName={this.state.parameters.operationName}
                    onEditQuery={this.onEditQuery}
                    onEditVariables={this.onEditVariables}
                    onEditOperationName={this.onEditOperationName}
                    fetcher={fetcher}
                    defaultQuery={defaultQuery}
                    editorTheme="sourcegraph"
                    ref={this.setGraphiQLRef}
                >
                    <GraphiQL.Logo>GraphQL API console</GraphiQL.Logo>
                    <GraphiQL.Toolbar>
                        <div className="d-flex align-items-center">
                            <GraphiQL.Button
                                onClick={this.handlePrettifyQuery}
                                title="Prettify Query (Shift-Ctrl-P)"
                                label="Prettify"
                            />
                            <GraphiQL.Button onClick={this.handleToggleHistory} title="Show History" label="History" />
                            <Link className="btn btn-link" to="/help/api/graphql">
                                Docs
                            </Link>
                            <div className="alert alert-warning py-1 px-3 mb-0 ml-2 text-nowrap">
                                <small>
                                    The API console uses <strong>real production data.</strong>
                                </small>
                            </div>
                        </div>
                    </GraphiQL.Toolbar>
                </GraphiQL>
            </>
        )
    }

    // Update state.parameters when query/variables/operation name are changed
    // so that we can update the browser URL.

    private onEditQuery = (newQuery: string): void =>
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

    // Forward GraphiQL prettify/history buttons directly to their original
    // implementation. We have to do this because it is impossible to inject
    // children into the GraphiQL toolbar unless you completely specify your
    // own.

    private setGraphiQLRef = (reference: _graphiqlModule.default | null): void => {
        this.graphiQLRef = reference
    }
    private handlePrettifyQuery = (): void => {
        if (!this.graphiQLRef) {
            return
        }
        this.graphiQLRef.handlePrettifyQuery()
    }
    private handleToggleHistory = (): void => {
        if (!this.graphiQLRef) {
            return
        }
        this.graphiQLRef.handleToggleHistory()
    }
}

async function fetcher(graphQLParameters: _graphiqlModule.GraphQLParams): Promise<string> {
    const response = await fetch('/.api/graphql', {
        method: 'POST',
        body: JSON.stringify(graphQLParameters),
        credentials: 'include',
        headers: new Headers({ 'x-requested-with': 'Sourcegraph GraphQL Explorer' }),
    })
    const responseBody = await response.text()
    try {
        return JSON.parse(responseBody)
    } catch {
        return responseBody
    }
}
