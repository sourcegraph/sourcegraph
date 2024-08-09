<script lang="ts" context="module">
    import type { Community } from '$lib/search/communityPages'

    // Unique number identifier for telemetry
    const specTypes: Record<string, number> = {
        backstage: 0,
        chakraui: 1,
        cncf: 2,
        temporal: 3,
        o3de: 4,
        stackstorm: 5,
        kubernetes: 6,
        stanford: 7,
        julia: 8,
    } satisfies Record<Community, number>
</script>

<script lang="ts">
    // @sg EnableRollout Dotcom

    import { onMount } from 'svelte'

    import { buildSearchURLQuery } from '@sourcegraph/shared/src/util/url'

    import LoadingSpinner from '$lib/LoadingSpinner.svelte'
    import CodeHostIcon from '$lib/search/CodeHostIcon.svelte'
    import SearchInput from '$lib/search/input/SearchInput.svelte'
    import { queryStateStore } from '$lib/search/state'
    import SyntaxHighlightedQuery from '$lib/search/SyntaxHighlightedQuery.svelte'
    import { displayRepoName } from '$lib/shared'
    import { settings } from '$lib/stores'
    import { TELEMETRY_RECORDER } from '$lib/telemetry'
    import { Alert, Button, Markdown } from '$lib/wildcard'

    import type { PageData } from './$types'

    export let data: PageData

    $: ({ title, description, homepageIcon, spec, lowProfile, examples } = data)

    $: context = `context:${spec}`
    $: queryState = queryStateStore({ query: context + ' ' }, $settings)

    onMount(() => {
        TELEMETRY_RECORDER.recordEvent('communitySearchContext', 'view', {
            metadata: { spec: specTypes[data.community] },
        })
    })

    function handleSearchExampleClick() {
        TELEMETRY_RECORDER.recordEvent('communitySearchContext.suggestion', 'click')
    }
    function handleRepoLinkClick() {
        TELEMETRY_RECORDER.recordEvent('communitySearchContext.repoLink', 'click')
    }
    function handleExternalRepoLinkClick() {
        TELEMETRY_RECORDER.recordEvent('communitySearchContext.repoLink.external', 'click')
    }
</script>

<svelte:head>
    <title>{title} - Sourcegraph</title>
</svelte:head>

<section>
    <hgroup>
        <h2><img src={homepageIcon} alt="" aria-hidden /><span>{title}</span></h2>
        <!-- We provide our own p element so that we can target it with a CSS selector without :global -->
        <p>
            <Markdown content={description} inline />
        </p>
    </hgroup>

    <SearchInput {queryState} autoFocus />

    {#if !lowProfile}
        <div class="main">
            {#if examples.length > 0}
                <div class="column examples">
                    <h3>Search examples</h3>
                    <ul data-testid="page.community.examples">
                        {#each examples as example}
                            {@const query = `${context} ${example.query}`}
                            <li>
                                <h4><Markdown content={example.title} inline /></h4>
                                <SyntaxHighlightedQuery wrap {query} patternType={example.patternType} />
                                <Button variant="secondary">
                                    <a
                                        slot="custom"
                                        let:buttonClass
                                        class={buttonClass}
                                        href="/search?{buildSearchURLQuery(query, example.patternType, false)}"
                                        on:click={handleSearchExampleClick}>Search</a
                                    >
                                </Button>
                                {#if example.description}
                                    <div class="description">
                                        <Markdown content={example.description} />
                                    </div>
                                {/if}
                            </li>
                        {/each}
                    </ul>
                </div>
            {/if}
            <aside class="column repositories">
                <h3>Repositories</h3>
                <div class="content">
                    <p>
                        Using the context <SyntaxHighlightedQuery query={context} /> in a query will search these repositories:
                    </p>

                    {#await data.repositories}
                        <LoadingSpinner center />
                    {:then repositories}
                        <ul class="repositories" data-testid="page.community.repositories">
                            {#each repositories as { repository: repo } (repo.name)}
                                <li>
                                    {#if repo.externalURLs.length > 0}
                                        <a
                                            href={repo.externalURLs[0].url}
                                            target="_blank"
                                            rel="noopener noreferrer"
                                            on:click={handleExternalRepoLinkClick}
                                        >
                                            <CodeHostIcon repository={repo.name} />
                                        </a>
                                    {/if}
                                    <a href="/{repo.name}" on:click={handleRepoLinkClick}>
                                        {displayRepoName(repo.name)}
                                    </a>
                                </li>
                            {/each}
                        </ul>
                    {:catch error}
                        <Alert variant="danger">{error.message}</Alert>
                    {/await}
                </div>
            </aside>
        </div>
    {/if}
</section>

<style lang="scss">
    section {
        width: 100%;
        max-width: var(--viewport-lg);
        margin-left: auto;
        margin-right: auto;

        display: flex;
        flex-direction: column;
        align-items: center;
    }

    h2 {
        img {
            width: 3rem;
            height: 3rem;
            margin: 0.5rem;
        }

        > * {
            vertical-align: middle;
        }
    }

    hgroup {
        margin-top: 3rem;

        @media (--sm-breakpoint-down) {
            margin-top: 1rem;
        }

        > * {
            text-align: center;
        }
    }

    .main {
        margin-top: 2rem;
        display: flex;
        gap: 1rem;
        align-items: flex-start;
        padding: 1rem;
        box-sizing: border-box;
        max-width: 100%;

        --card-border-radius: var(--border-radius);

        @media (--sm-breakpoint-down) {
            flex-direction: column;
            align-items: stretch;
        }

        @media (--xs-breakpoint-down) {
            --card-border-radius: 0;
            padding: 0;

            h3 {
                margin-left: 1rem;
            }
        }
    }

    ul {
        list-style-type: none;
        padding: 0;
        margin: 0;
    }

    .examples {
        flex: 1;
        min-width: 0;

        ul {
            display: grid;
            grid-template-columns: 1fr auto;
            grid-row-gap: 1rem;

            li {
                display: grid;
                grid-column: span 2;
                grid-template-columns: subgrid;
                grid-template-rows: auto auto auto;
                align-items: center;
                row-gap: 0.5rem;

                border: 1px solid var(--border-color);
                padding: 1rem;
                border-radius: var(--card-border-radius);
                background-color: var(--color-bg-1);

                > h4 {
                    grid-column: span 2;
                }
                .description {
                    padding-top: 0.5rem;
                    border-top: 1px solid var(--border-color);
                    grid-column: span 2;
                }
            }
        }
    }

    .repositories {
        flex: 1;
        // Forces column to grow so that examples and repositories are
        // the same width
        min-width: 0;

        .content {
            border: 1px solid var(--border-color);
            padding: 1rem;
            border-radius: var(--card-border-radius);
            background-color: var(--color-bg-2);
        }

        ul {
            columns: 2;

            @media (--md-breakpoint-down) {
                columns: 1;
            }

            li {
                white-space: nowrap;
                overflow: hidden;
                text-overflow: ellipsis;
                padding: 0.125rem 0;
            }
        }
    }
</style>
