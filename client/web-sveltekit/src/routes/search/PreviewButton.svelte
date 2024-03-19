<script lang="ts">
    import type { ContentMatch, PathMatch, SymbolMatch } from '$lib/shared'
    import { Button } from '$lib/wildcard'

    import { getSearchResultsContext } from './searchResultsContext'

    export let result: ContentMatch | SymbolMatch | PathMatch

    const searchResultContext = getSearchResultsContext()
</script>

<div class="preview-button">
    <Button variant="link" on:click={() => searchResultContext.setPreview(result)}>Preview</Button>
</div>

<style lang="scss">
    // Override the padding of the button so it doesn't grow the header.
    .preview-button :global(button) {
        padding: 0;
    }
</style>
