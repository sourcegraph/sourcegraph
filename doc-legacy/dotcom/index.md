# Sourcegraph.com

[Sourcegraph.com](https://sourcegraph.com/search) lets you search 2 million open source repositories.

To use Sourcegraph on your own (private) code, [use Sourcegraph Cloud](../cloud/index.md) or [deploy self-hosted Sourcegraph](../admin/deploy/index.md).

- [Indexing open source code in Sourcegraph.com](indexing_open_source_code.md)

> Note: Sourcegraph.com is a special instance of Sourcegraph with some different behavior compared to that on Sourcegraph Cloud and self-hosted instances. If you're curious about the differences, search our codebase for [`envvar.SourcegraphDotComMode()`](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+envvar.SourcegraphDotComMode%28%29&patternType=standard&sm=1).
>
> For example, global searches do not search unindexed code by default on sourcegraph.com, whereas on a cloud or self-hosted instance this isn't the case.
> 
> To learn more about where dotcom handles things differently checkout use of the [SourcegraphDotcomMode](https://sourcegraph.com/search?q=context:global+repo:%5Egithub%5C.com/sourcegraph/sourcegraph%24+if+envvar.SourcegraphDotComMode%28%29&patternType=standard&sm=1) env var in our codebase!
