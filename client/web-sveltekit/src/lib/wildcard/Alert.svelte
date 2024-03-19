<script lang="ts">
    export let size: 'normal' | 'slim' = 'normal'
    export let variant: 'info' | 'warning' | 'danger' | 'success'
</script>

<div
    role="alert"
    class:info={variant === 'info'}
    class:warning={variant === 'warning'}
    class:danger={variant === 'danger'}
    class:success={variant === 'success'}
    class:slim={size === 'slim'}
>
    <slot />
</div>

<style lang="scss">
    div {
        --alert-icon-display: block;
        --alert-icon-block-width: 2.5rem;
        --alert-content-padding: 0.5rem;
        --alert-background-color: var(--color-bg-1);

        display: flex;
        align-items: center;
        overflow: hidden;
        position: relative;
        margin-bottom: 1rem;
        color: var(--body-color);
        border-radius: var(--border-radius);
        border: 1px solid var(--alert-border-color);

        background-color: var(--alert-background-color);
        padding: var(--alert-content-padding) var(--alert-content-padding) var(--alert-content-padding)
            calc(var(--alert-icon-block-width) + var(--alert-content-padding));

        &::before,
        &::after {
            display: var(--alert-icon-display);
            content: '';
            position: absolute;
            top: 0;
            left: 0;
            width: var(--alert-icon-block-width);
            height: 100%;
        }

        /* Alert icon background. */
        &::before {
            border: 2px solid var(--color-bg-1);
            border-top-left-radius: var(--border-radius);
            border-bottom-left-radius: var(--border-radius);
            background-color: var(--alert-icon-background-color);
        }

        &::after {
            mask-repeat: no-repeat;
            mask-size: 1rem;
            mask-position: 50% 50%;

            /* Applied as a fill color for SVG icon because of the mask-image. */
            background-color: var(--alert-icon-color);
        }
    }

    .danger {
        --alert-border-color: var(--danger);
        --alert-icon-background-color: var(--danger-4);

        :global(.theme-light) & {
            --alert-icon-color: var(--danger-3);
        }

        :global(.theme-dark) & {
            --alert-icon-color: var(--danger);
        }

        &::after {
            /* Icon: mdi/AlertCircle */
            mask-image: url("data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24'><path d='M13 13h-2V7h2m0 10h-2v-2h2M12 2A10 10 0 002 12a10 10 0 0010 10 10 10 0 0010-10A10 10 0 0012 2z'/></svg>");
        }
    }

    .info {
        --alert-border-color: var(--primary);
        --alert-icon-background-color: var(--primary-4);

        :global(.theme-light) & {
            --alert-icon-color: var(--primary-3);
        }

        :global(.theme-dark) & {
            --alert-icon-color: var(--primary);
        }

        &::after {
            // Icon: mdi/Information
            mask-image: url("data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24'><path d='M13 9h-2V7h2m0 10h-2v-6h2m-1-9A10 10 0 002 12a10 10 0 0010 10 10 10 0 0010-10A10 10 0 0012 2z'/></svg>");
        }
    }

    .warning {
        --alert-border-color: var(--warning);
        --alert-icon-background-color: var(--warning-4);

        :global(.theme-light) & {
            --alert-icon-color: var(--warning-3);
        }

        :global(.theme-dark) & {
            --alert-icon-color: var(--warning);
        }

        &::after {
            mask-image: url("data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24'><path d='M13 14h-2V9h2m0 9h-2v-2h2M1 21h22L12 2 1 21z'/></svg>");
        }
    }

    .success {
        --alert-border-color: var(--success);
        --alert-icon-background-color: var(--success-4);

        :global(.theme-light) & {
            --alert-icon-color: var(--success-3);
        }

        :global(.theme-dark) & {
            --alert-icon-color: var(--success);
        }

        &::after {
            /* Icon: mdi-react/CheckCircle */
            mask-image: url("data:image/svg+xml,<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 24 24'><path d='M12 2C6.5 2 2 6.5 2 12s4.5 10 10 10 10-4.5 10-10S17.5 2 12 2m-2 15l-5-5 1.41-1.41L10 14.17l7.59-7.59L19 8l-9 9z'/></svg>");
        }
    }

    .slim {
        margin-bottom: 0;
        border-radius: 0;
        border-width: 0 0 1px 0;
        background: var(--alert-icon-background-color);
        padding-left: var(--alert-content-padding);
        border-color: var(--border-color-2);

        &::before,
        &::after {
            display: none;
        }
    }
</style>
