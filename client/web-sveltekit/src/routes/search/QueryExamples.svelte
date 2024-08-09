<script lang="ts">
    import type { Readable } from 'svelte/store'

    import { isDefined } from '$lib/common'
    import Icon from '$lib/Icon.svelte'
    import SyntaxHighlightedQuery from '$lib/search/SyntaxHighlightedQuery.svelte'
    import { buildSearchURLQuery } from '$lib/shared'
    import { settings } from '$lib/stores'
    import TabPanel from '$lib/TabPanel.svelte'
    import Tabs from '$lib/Tabs.svelte'
    import { defaultPatternTypeFromSettings } from '$lib/web'
    import ProductStatusBadge from '$lib/wildcard/ProductStatusBadge.svelte'
    import { SearchPatternType } from '$lib/graphql-operations'

    import { basicSyntaxColumns, exampleQueryColumns, type QueryExample } from './queryExamples'

    export let showQueryPage = false
    export let queryExample: Readable<QueryExample | null>

    const defaultPatternType = defaultPatternTypeFromSettings({ final: $settings, subjects: [] })
    const showQueryExamplesForKeywordSearch =
        defaultPatternType === SearchPatternType.keyword || defaultPatternType === SearchPatternType.regexp
    const patternTypeForQueryLinks = showQueryExamplesForKeywordSearch
        ? SearchPatternType.keyword
        : SearchPatternType.standard

    $: tabs = [
        $queryExample
            ? {
                  name: 'How to search',
                  columns: basicSyntaxColumns(
                      $queryExample.fileName,
                      $queryExample.repoName,
                      $queryExample.orgName,
                      showQueryExamplesForKeywordSearch
                  ),
              }
            : null,
        showQueryPage
            ? {
                  name: 'Popular queries',
                  columns: exampleQueryColumns,
              }
            : null,
    ].filter(isDefined)
</script>

<div class="container">
    <Tabs>
        {#each tabs as { name, columns }}
            <TabPanel title={name}>
                <div class="columns">
                    {#each columns as sections, i}
                        <div class="column">
                            {#each sections as section}
                                <h2>{section.title}</h2>
                                <ul>
                                    {#each section.queryExamples as example}
                                        {#if example.query.length > 0}
                                            <li>
                                                <a
                                                    class="chip"
                                                    href="/search?{buildSearchURLQuery(
                                                        example.query,
                                                        patternTypeForQueryLinks,
                                                        false
                                                    )}"
                                                >
                                                    <SyntaxHighlightedQuery query={example.query} />
                                                </a>
                                                {#if example.helperText}
                                                    <small>{example.helperText}</small>
                                                {/if}
                                                {#if example.productStatus}
                                                    <ProductStatusBadge status={example.productStatus} />
                                                {/if}
                                            </li>
                                        {/if}
                                    {/each}
                                </ul>
                            {/each}
                            {#if columns.length === i + 1}
                                <a href="/help/code_search/reference/queries" target="blank">
                                    Complete query reference
                                    <Icon icon={ILucideExternalLink} aria-label="Open in new tab" inline />
                                </a>
                            {/if}
                        </div>
                    {/each}
                </div>
            </TabPanel>
        {/each}
    </Tabs>
</div>

<style lang="scss">
    .container {
        flex: 1;
        --tabs-header-align: center;

        :global([data-tab-header]) {
            width: 100%;
        }
    }

    .columns {
        display: flex;
        flex-wrap: wrap;
        gap: 4rem;
        margin-top: 1.5rem;

        @media (--sm-breakpoint-down) {
            gap: 1rem;
        }
    }

    h2 {
        margin-bottom: 0.75rem;
        color: var(--text-muted);
        font-size: var(--font-size-small);
        font-weight: 400;
    }

    ul {
        margin-bottom: 1.5rem;
        padding: 0;
        list-style: none;
    }

    li {
        display: flex;
        align-items: center;
        margin-bottom: 0.25rem;
        gap: 0.5rem;
    }

    a.chip {
        background-color: var(--code-bg);
        box-shadow: var(--search-input-shadow);
        border-radius: var(--border-radius);
        padding: 0.125rem 0.375rem;
        font-size: 0.75rem;
        max-width: 21rem;
        user-select: none;
        border: 1px solid transparent;

        &:hover {
            border-color: var(--border-color);
            text-decoration: none;
        }

        &:active {
            border-color: var(--border-color);
            box-shadow: none;
        }
    }

    a {
        --icon-color: currentColor;
        font-size: var(--font-size-small);
    }

    small {
        color: var(--text-muted);
    }
</style>
