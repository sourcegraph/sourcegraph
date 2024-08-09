<script lang="ts">
    import { renderMarkdown, type RenderMarkdownOptions } from '$lib/common'

    import styles from './Markdown.module.scss'

    export let content: string
    export let options: RenderMarkdownOptions | undefined = undefined
    export let inline = false

    $: finalOptions = options ? { ...options, inline } : { inline }
</script>

{#if content}
    <svelte:element this={inline ? 'span' : 'div'} class={styles.markdown}>
        {@html renderMarkdown(content, finalOptions)}
    </svelte:element    >
{/if}
