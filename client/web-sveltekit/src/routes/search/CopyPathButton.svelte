<script lang="ts">
    import { mdiContentCopy } from '@mdi/js'
    import copy from 'copy-to-clipboard'

    import Icon from '$lib/Icon.svelte'
    import Tooltip from '$lib/Tooltip.svelte'
    import { Button } from '$lib/wildcard'

    export let path: string

    let recentlyCopied = false
    function handleCopyPath(): void {
        copy(path)
        recentlyCopied = true
        setTimeout(() => {
            recentlyCopied = false
        }, 1000)
    }

    $: tooltip = recentlyCopied ? 'Copied!' : 'Copy path to clipboard'
</script>

<span data-visible-on-focus class="copy-path-button">
    <Tooltip {tooltip} placement="bottom">
        <Button on:click={() => handleCopyPath()} variant="icon" size="sm" aria-label="Copy path to clipboard">
            <Icon inline svgPath={mdiContentCopy} aria-hidden />
        </Button>
    </Tooltip>
</span>

<style lang="scss">
    .copy-path-button {
        --color: var(--icon-color);
        &:hover {
            --color: var(--body-color);
        }
    }
</style>
