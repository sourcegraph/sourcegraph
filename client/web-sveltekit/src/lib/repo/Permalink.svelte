<!--
    @component
    Renders a permalink to the current page with the given Git commit ID.
-->
<script lang="ts">
    import { goto } from '$app/navigation'
    import { page } from '$app/stores'
    import { createHotkey } from '$lib/Hotkey'
    import Icon from '$lib/Icon.svelte'
    import { replaceRevisionInURL } from '$lib/shared'
    import Tooltip from '$lib/Tooltip.svelte'
    import { parseBrowserRepoURL } from '$lib/web'

    export let revID: string

    const hotkey = createHotkey({
        keys: { key: 'y' },
        ignoreInputFields: true,
        handler: () => {
            const { revision } = parseBrowserRepoURL($page.url.pathname)
            // Only navigate if necessary. We don't want to add unnecessary history entries.
            if (revision !== revID) {
                goto(href, { noScroll: true, keepFocus: true }).catch(() => {
                    // TODO: log error with Sentry
                })
            }
        },
    })

    $: href = revID ? replaceRevisionInURL($page.url.toString(), revID) : ''
    $: console.log('href', href)
    $: if (href) {
        hotkey.enable()
    } else {
        hotkey.disable()
    }
</script>

{#if href}
    <Tooltip tooltip="Permalink (with full git commit SHA)">
        <a {href}><Icon icon={ILucideLink} inline aria-hidden /> <span data-action-label>Permalink</span></a>
    </Tooltip>
{/if}

<style lang="scss">
    a {
        display: flex;
        align-items: center;
        justify-content: center;
        gap: 0.5rem;
        color: var(--text-body);
        text-decoration: none;
        white-space: nowrap;

        &:hover {
            color: var(--text-title);
        }
    }
</style>
