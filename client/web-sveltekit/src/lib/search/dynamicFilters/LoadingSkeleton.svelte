<div class="lines">
    <div class="line-title line" />
    <div class="line" />
    <div class="line" />
    <div class="line" />
</div>

<style lang="scss">
    .lines {
        display: flex;
        flex-direction: column;
        gap: 0.5rem;
        padding: 0.5rem 0.75rem;
        margin-bottom: 0.75rem;

        --main-color: #3e475e;
        --blink-color: #4d5875;

        :global(.theme-light) & {
            --main-color: #dddddd;
            --blink-color: #e8e8e8;
        }
    }

    .line {
        height: 0.75rem;
        border-radius: var(--border-radius);
        background-size: 200% 100%;
        animation: shine-lines 1s infinite linear;
        background-image: linear-gradient(100deg, var(--main-color) 0, var(--blink-color) 40px, var(--main-color) 80px);

        &-title {
            max-width: 50%;
            height: 0.5rem;
        }
    }

    @keyframes shine-lines {
        100% {
            background-position-x: -200%;
        }
    }
</style>
