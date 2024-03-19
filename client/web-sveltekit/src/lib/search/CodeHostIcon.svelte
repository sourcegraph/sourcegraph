<script lang="ts" context="module">
    import { mdiBitbucket, mdiGithub, mdiGitlab } from '@mdi/js'

    const iconMap: { [key: string]: string } = {
        'github.com': mdiGithub,
        'gitlab.com': mdiGitlab,
        'bitbucket.org': mdiBitbucket,
    }
</script>

<script lang="ts">
    import Icon from '$lib/Icon.svelte'
    import Tooltip from '$lib/Tooltip.svelte'

    export let repository: string
    export let disableTooltip: boolean = false

    $: hostName = repository.split('/')[0]
    $: svgPath = iconMap[hostName]
</script>

{#if svgPath}
    <Tooltip tooltip={disableTooltip ? '' : hostName}>
        <Icon aria-label={hostName} {svgPath} inline />&nbsp;
    </Tooltip>
{/if}
