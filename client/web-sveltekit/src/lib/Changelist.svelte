<svelte:options immutable />

<script lang="ts">
    import Avatar from '$lib/Avatar.svelte'
    import Icon from '$lib/Icon.svelte'
    import Timestamp from '$lib/Timestamp.svelte'
    import Tooltip from '$lib/Tooltip.svelte'

    import type { Changelist } from './Changelist.gql'
    import { isViewportMobile } from './stores'
    import Button from './wildcard/Button.svelte'

    export let changelist: Changelist
    export let alwaysExpanded: boolean = false

    $: expanded = alwaysExpanded

    $: author = changelist.author
    $: commitDate = new Date(author.date)
    $: authorAvatarTooltip = author.person.name + (author ? ' (author)' : '')
</script>

<div class="root">
    <div class="avatar">
        <Tooltip tooltip={authorAvatarTooltip}>
            <Avatar avatar={author.person} />
        </Tooltip>
    </div>
    <div class="title">
        <!-- TODO need subject-->
        <a class="subject" href={changelist.perforceChangelist?.canonicalURL}>{changelist.subject}</a>
        {#if !alwaysExpanded && changelist.body && !$isViewportMobile}
            <Button
                variant="secondary"
                size="sm"
                on:click={() => (expanded = !expanded)}
                aria-label="{expanded ? 'Hide' : 'Show'} changelist message"
            >
                <Icon icon={ILucideEllipsis} inline aria-hidden />
            </Button>
        {/if}
    </div>
    <div class="author">
        submitted by <strong>{author.person.name}</strong>
        <Timestamp date={commitDate} />
    </div>
    {#if changelist.body}
        <div class="message" class:expanded>
            {#if $isViewportMobile}
                {#if expanded}
                    <Button variant="secondary" size="lg" display="block" on:click={() => (expanded = false)}>
                        Close
                    </Button>
                {:else}
                    <Button variant="secondary" size="sm" display="block" on:click={() => (expanded = true)}>
                        Show changelist message
                    </Button>
                {/if}
            {/if}

            <pre>{changelist.body}</pre>
        </div>
    {/if}
</div>

<style lang="scss">
    .root {
        display: grid;
        overflow: hidden;
        grid-template-columns: auto 1fr;
        grid-template-areas: 'avatar title' 'avatar author' '. message';
        column-gap: 1rem;

        @media (--mobile) {
            grid-template-columns: auto 1fr;
            grid-template-areas: 'avatar title' 'author author' 'message message';
            row-gap: 0.5rem;
        }
    }

    .avatar {
        grid-area: avatar;
        display: flex;
        gap: 0.25rem;
        align-self: center;
    }

    .title {
        grid-area: title;
        align-self: center;

        display: flex;
        gap: 0.5rem;
        align-items: center;
        overflow: hidden;

        .subject {
            font-weight: 600;
            flex: 0 1 auto;
            color: var(--body-color);
            min-width: 0;

            @media (--sm-breakpoint-up) {
                overflow: hidden;
                white-space: nowrap;
                text-overflow: ellipsis;
            }
        }
    }

    .author {
        grid-area: author;
        color: var(--text-muted);
    }

    .message {
        grid-area: message;
        overflow: hidden;

        @media (--mobile) {
            &.expanded {
                position: fixed;
                top: 0;
                left: 0;
                right: 0;
                bottom: 0;
                margin: 0;

                display: flex;
                flex-direction: column;

                background-color: var(--color-bg-1);
            }
        }
    }

    pre {
        display: none;
        margin-top: 0.5rem;
        margin-bottom: 1.5rem;

        font-size: 0.75rem;
        max-width: 100%;
        word-wrap: break-word;
        white-space: pre-wrap;

        .expanded & {
            display: block;
        }

        @media (--mobile) {
            padding: 0.5rem;
            overflow: auto;
            margin: 0;
        }
    }
</style>
