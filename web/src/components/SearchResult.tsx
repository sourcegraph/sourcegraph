import { decode } from 'he'
import FileIcon from 'mdi-react/FileIcon'
import React from 'react'
import { ResultContainer } from '../../../shared/src/components/ResultContainer'
import * as GQL from '../../../shared/src/graphql/schema'
import { renderMarkdown } from '../../../shared/src/util/markdown'
import { SearchResultMatch } from './SearchResultMatch'
import { ThemeProps } from '../../../shared/src/theme'
import * as H from 'history'
import { Markdown } from '../../../shared/src/components/Markdown'

/**
 * This is a subset of GQL.GenericSearchResultInterface and GQL.IGenericSearchResultInterface that we use to render a SearchResult.
 */
interface GenericSearchResult {
    /**
     * URL to an icon that is displayed with every search result.
     */
    icon: string

    /**
     * A markdown string that is rendered prominently.
     */
    label: GQL.IMarkdown

    /**
     * The URL of the result.
     */
    url: string

    /**
     * A markdown string that is rendered less prominently.
     */
    detail: GQL.IMarkdown

    /**
     * A list of matches in this search result.
     */
    matches: GQL.ISearchResultMatch[]
}

interface Props extends ThemeProps {
    result: GenericSearchResult
    history: H.History
}

export class SearchResult extends React.Component<Props> {
    private renderTitle = (): JSX.Element => (
        <div className="search-result__title">
            <Markdown
                className="test-search-result-label"
                dangerousInnerHTML={
                    this.props.result.label.html
                        ? decode(this.props.result.label.html)
                        : renderMarkdown(this.props.result.label.text)
                }
                history={this.props.history}
            />
            {this.props.result.detail && (
                <>
                    <span className="search-result__spacer" />
                    <Markdown
                        dangerousInnerHTML={
                            this.props.result.detail.html
                                ? decode(this.props.result.detail.html)
                                : renderMarkdown(this.props.result.detail.text)
                        }
                        history={this.props.history}
                    />
                </>
            )}
        </div>
    )

    private renderBody = (): JSX.Element => (
        <>
            {this.props.result.matches.map(match => (
                <SearchResultMatch
                    key={match.url}
                    item={match}
                    highlightRanges={match.highlights}
                    isLightTheme={this.props.isLightTheme}
                    history={this.props.history}
                />
            ))}
        </>
    )

    public render(): JSX.Element {
        return (
            <ResultContainer
                stringIcon={this.props.result.icon}
                icon={FileIcon}
                collapsible={this.props.result && this.props.result.matches.length > 0}
                defaultExpanded={true}
                title={this.renderTitle()}
                expandedChildren={this.renderBody()}
            />
        )
    }
}
