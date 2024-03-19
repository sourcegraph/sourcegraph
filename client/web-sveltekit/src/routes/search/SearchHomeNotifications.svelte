<script lang="ts">
    import {settings} from '$lib/stores'
    import {Markdown} from '$lib/wildcard'

    $: notices = $settings?.notices?.filter(notice => notice.location === 'home')
</script>

{#if notices}
    <ul class="root">
        {#each notices as notice (notice.message)}
            <li class="item">
                <Markdown content={notice.message}/>
            </li>
        {/each}
    </ul>
{/if}

<style lang="scss">
    .root {
        display: flex;
        flex-direction: column;
        gap: 0.25rem;
        margin: 0;
        padding: 0;
    }

    .item {
        display: flex;
        justify-content: center;
        align-items: center;
        border: 1px solid var(--border-color);
        padding: 0.5rem;
    }
</style>
