<script lang="ts">
    import { formatDistanceToNow } from 'date-fns'

    export let commitURL: string
    export let avatarURL: string | null
    export let displayName: string
    export let commitMessage: string
    export let commitDate: string
    export let owner: string

    function getFirstNameAndLastInitial(name: string): string {
        const names = name.split(' ')
        const firstName = names[0]
        const lastInitial = names[names.length - 1].charAt(0).toUpperCase()
        return `${firstName} ${lastInitial}.`
    }

    function extractCommitMessage(commitMessage: string): string {
        let split = commitMessage.split(' ')
        let msg = split.slice(0, split.length - 1)
        return msg.join(' ')
    }

    function extractSHA(commitMessage: string): string {
        let split = commitMessage.split(' ')
        let sha = split[split.length - 1]
        let noParens = sha.slice(1, sha.length - 1)
        return noParens
    }

    function convertToElapsedTime(commitDateString: string): string {
        const commitDate = new Date(commitDateString)
        const elapsed = formatDistanceToNow(commitDate, { addSuffix: true })

        return elapsed
    }

    $: commitMessageNoSHA = extractCommitMessage(commitMessage)
    $: commitSHA = extractSHA(commitMessage)
</script>

<div class="latest-commit">
    <div class="commit-info">
        <img class="avatar" src={avatarURL} role="presentation" aria-hidden="true" alt="Avatar of {displayName}" />
        <div class="display-name">
            <small>{getFirstNameAndLastInitial(displayName)}</small>
        </div>

        <!-- TODO: don't hard code this-->
        <div class="commit-message">
            <small>
                {commitMessageNoSHA}
                (<a href={commitURL}>{commitSHA}</a>)
            </small>
        </div>
        <!-- TODO: don't hard code this-->
        <div class="commit-date">
            <small>{convertToElapsedTime(commitDate)}</small>
        </div>
    </div>
    <div class="owner-info">
        <div class="owner"><small>{owner}</small></div>
    </div>
</div>

<style lang="scss">
    .avatar {
        width: 1rem;
        height: 1rem;
        border-radius: 100%;
        margin-right: 0.3rem;
    }

    .latest-commit {
        display: flex;
        flex-flow: row nowrap;
        align-items: center;
    }

    .commit-info {
        display: flex;
        flex-flow: row nowrap;
        align-items: center;
        justify-content: space-evenly;
        border-right: 1px solid var(--text-muted);
        padding-right: 0.2rem;
    }

    .commit-info div {
        padding: 0 0.3rem;
    }

    .owner-info {
        padding: 0rem 0.5rem;
    }

    .commit-message,
    .commit-date,
    .owner {
        color: var(--text-muted);
    }
</style>
