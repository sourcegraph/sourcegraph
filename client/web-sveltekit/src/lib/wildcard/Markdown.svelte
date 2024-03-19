<script lang="ts">
    import { renderMarkdown } from '$lib/common'
    import styles from './Markdown.module.scss'

    export let content: string
</script>

{#if content}
    <div class={styles.markdown}>
        {@html renderMarkdown(content)}
    </div>
{/if}
