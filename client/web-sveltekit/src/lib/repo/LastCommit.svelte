<script lang="ts">
    import { formatDistanceToNow } from 'date-fns'

    import Avatar from '$lib/Avatar.svelte'
    import type { Avatar_User } from '$testing/graphql-type-mocks'

    export let commitURL: string
    export let avatarURL: string | null
    export let displayName: string
    export let commitMessage: string
    export let commitDate: string

    let avatar: Avatar_User = {
        __typename: 'User',
        avatarURL: avatarURL,
        displayName: displayName,
        username: displayName,
    }

    function getFirstNameAndLastInitial(name: string): string {
        const names = name.split(' ')
        return `${names[0]} ${names[names.length - 1].charAt(0).toUpperCase()}.`
    }

    function extractCommitMessage(commitMessage: string): string {
        let split = commitMessage.split(' ')
        let msg = split.slice(0, split.length - 1)
        return msg.join(' ')
    }

    function extractSHA(cm: string): string {
        let cmWords = cm.split(' ')
        let sha = cmWords[cmWords.length - 1]
        return sha.slice(1, sha.length - 1)
    }

    function convertToElapsedTime(commitDateString: string): string {
        const commitDate = new Date(commitDateString)
        return formatDistanceToNow(commitDate, { addSuffix: true })
    }

    function truncateIfNeeded(cm: string): string {
        cm = extractCommitMessage(commitMessage)
        return cm.length > 30 ? cm.substring(0, 30) + '...' : cm
    }

    $: commitMessageNoSHA = truncateIfNeeded(commitMessage)
    $: commitSHA = extractSHA(commitMessage)
</script>

<div class="latest-commit">
    <div class="commit-info">
        <div class="user-info">
            <Avatar {avatar} />
            <div class="display-name">
                <small>{getFirstNameAndLastInitial(displayName)}</small>
            </div>
        </div>

        <div class="commit-message">
            <small>
                {commitMessageNoSHA}
                (<a href={commitURL}>{commitSHA}</a>)
            </small>
        </div>

        <div class="commit-date">
            <small>{convertToElapsedTime(commitDate)}</small>
        </div>
    </div>
</div>

<style lang="scss">
    .latest-commit {
        display: flex;
        flex-flow: row nowrap;
        padding-right: 0.5rem;
    }

    .user-info {
        display: flex;
        flex-flow: row nowrap;
        align-items: center;
    }

    .display-name {
        margin-left: 0.3rem;
    }

    .commit-info {
        display: flex;
        flex-flow: row nowrap;
        min-width: 290px;
        align-items: center;
        justify-content: space-between;
    }

    .commit-message,
    .commit-date {
        color: var(--text-muted);
    }
</style>
