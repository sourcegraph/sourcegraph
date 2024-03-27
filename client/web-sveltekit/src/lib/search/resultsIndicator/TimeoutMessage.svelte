<script lang="ts">
    const CENTER_DOT = '\u00B7' // interpunct
</script>

<div class="suggested-action">
    <div class="info-badge"><small>Taking too long?</small></div>
    <div class="separator">{CENTER_DOT}</div>
    <div class="action-badge">
        <small>
            Use
            <a href="/help/code-search/types/search-jobs" target="_blank" rel="noopener noreferrer"> Search Job </a>
            for background search
        </small>
    </div>
</div>

<style lang="scss">
    .info-badge {
        background-color: var(--warning-2);
        border-radius: 3px;
        color: var(--text-body);
        padding-left: 0.2rem;
        padding-right: 0.2rem;
    }

    .separator {
        padding: 0rem 0.4rem;
    }

    .suggested-action {
        display: flex;
        flex-flow: row nowrap;
    }
</style>
