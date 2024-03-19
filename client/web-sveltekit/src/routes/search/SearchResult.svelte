<article data-testid="search-result">
    <div class="header">
        <div class="icon">
            <slot name="icon" />
        </div>
        <div class="title">
            <slot name="title" />
        </div>
        <div class="info">
            <slot name="info" />
        </div>
    </div>
    {#if $$slots.default || $$slots.body}
        <slot name="body">
            <div class="body">
                <slot />
            </div>
        </slot>
    {/if}
</article>

<style lang="scss">
    .header {
        display: flex;
        align-items: center;
        padding: 0.5rem;
        position: sticky;
        top: 0;
        background-color: var(--body-bg);
        border-bottom: 1px solid var(--border-color);
    }

    .icon {
        display: flex;
        align-items: center;
        flex-shrink: 0;
        --color: var(--text-muted);
    }

    .title {
        flex: 1 1 auto;
        overflow: hidden;
        display: flex;
        flex-wrap: wrap;

        // .title-inner
        overflow-wrap: anywhere;
    }

    .info {
        margin-left: auto;
        display: flex;
        flex-wrap: wrap;
        color: var(--text-muted);
        flex-shrink: 0;
        gap: 0.5rem;
        align-items: center;
    }

    .body:not(:empty) {
        background-color: var(--code-bg);
        border-bottom: 1px solid var(--border-color);
    }
</style>
