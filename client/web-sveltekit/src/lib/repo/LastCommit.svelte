<script lang="ts">
    import type { Avatar_User } from '$lib/Avatar.gql'
    import Avatar from '$lib/Avatar.svelte'
    import {
        truncateIfNeeded,
        extractPRNumber,
        getFirstNameAndLastInitial,
        convertToElapsedTime,
    } from '$lib/repo/utils'

    export let latestCommit: LastCommitProps

    interface LastCommitProps {
        id: string
        abbreviatedOID: string
        subject: string
        canonicalURL: string
        author: {
            date: string
            person: {
                avatarURL: string | null
                displayName: string
                name: string
            }
        }
    }

    let avatar: Avatar_User = {
        __typename: 'User',
        avatarURL: latestCommit.author.person.avatarURL,
        displayName: latestCommit.author.person.displayName,
        username: latestCommit.author.person.name,
    }

    $: commitMessageNoPR = truncateIfNeeded(latestCommit.subject)
    $: PRNumber = extractPRNumber(latestCommit.subject)
</script>

<div class="latest-commit">
    <div class="user-info">
        <Avatar {avatar} />
        <div class="display-name">
            <small>{getFirstNameAndLastInitial(latestCommit.author.person.displayName)}</small>
        </div>
    </div>

    <div class="commit-message">
        {#if PRNumber}
            <small>{commitMessageNoPR}</small>
            (<a href={latestCommit.canonicalURL}><small>{PRNumber}</small></a>)
        {:else}
            <a href={latestCommit.canonicalURL}><small>{commitMessageNoPR}</small></a>
        {/if}
    </div>

    <div class="commit-date">
        <small>{convertToElapsedTime(latestCommit.author.date)}</small>
    </div>
</div>

<style lang="scss">
    .latest-commit {
        display: flex;
        flex-flow: row nowrap;
        padding-right: 0.5rem;
        min-width: 300px;
        align-items: center;
        justify-content: space-between;
    }

    .user-info {
        display: flex;
        flex-flow: row nowrap;
        align-items: center;
        margin-right: 0.5rem;
    }

    .display-name {
        margin-left: 0.4rem;
    }

    .commit-message,
    .commit-date {
        color: var(--text-muted);
        margin-right: 0.5rem;
    }
</style>
