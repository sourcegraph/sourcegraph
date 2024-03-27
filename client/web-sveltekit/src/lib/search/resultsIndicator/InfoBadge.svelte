<script lang="ts">
    import { getProgressText } from '$lib/branded'
    import type { Progress } from '$lib/shared'

    export let progress: Progress
    export let isError: boolean
</script>

<div class="progress-message" class:error-text={isError}>
    {getProgressText(progress).visibleText}
</div>

<style lang="scss">
    .progress-message {
        font-size: var(--font-size-base);
        &.error-text {
            color: var(--danger);
        }
    }
</style>
