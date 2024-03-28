<script lang="ts">
    import { formatDistanceToNow } from 'date-fns'

    import type { Avatar_User } from '$lib/Avatar.gql'
    import Avatar from '$lib/Avatar.svelte'

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

    function getFirstNameAndLastInitial(name: string): string {
        const names = name.split(' ')
        if (names.length < 2) {
            return `${names[0]}`
        }
        return `${names[0]} ${names[names.length - 1].charAt(0).toUpperCase()}.`
    }

    function extractCommitMessage(commitMessage: string): string {
        let split = commitMessage.split(' ')
        let msg = split.slice(0, split.length - 1)
        return msg.join(' ')
    }

    function extractPRNumber(cm: string): string {
        let cmWords = cm.split(' ')
        let sha = cmWords[cmWords.length - 1]
        return sha.slice(1, sha.length - 1)
    }

    function convertToElapsedTime(commitDateString: string): string {
        const commitDate = new Date(commitDateString)
        return formatDistanceToNow(commitDate, { addSuffix: true })
    }

    function truncateIfNeeded(cm: string): string {
        cm = extractCommitMessage(cm)
        return cm.length > 23 ? cm.substring(0, 23) + '...' : cm
    }

    $: commitMessageNoSHA = truncateIfNeeded(latestCommit.subject)
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
        <small>{commitMessageNoSHA}</small>
        (<a href={latestCommit.canonicalURL}><small>{PRNumber}</small></a>)
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
