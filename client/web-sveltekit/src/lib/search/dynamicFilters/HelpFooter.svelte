<script lang="ts">
    import Icon from '$lib/Icon.svelte'
</script>

<footer>
    <a href="/help/code_search/reference/queries" class="link">
        <div class="icon"><Icon icon={ILucideBookOpen} aria-hidden /></div>
        <div>
            <h4>Need more advanced filters?</h4>
            <small>Explore the query syntax docs</small>
        </div>
        <div class="icon"><Icon icon={ILucideExternalLink} aria-hidden inline /></div>
    </a>
</footer>

<style lang="scss">
    footer {
        line-height: 1;

        small {
            font-size: var(--font-size-xs);
        }

        &::before {
            content: '';
            height: 1px;
            background: var(--border-color);
            display: block;
        }

        &:hover {
            h4 {
                color: var(--primary);
            }

            &::before {
                background: linear-gradient(91deg, var(--logo-blue) 3.11%, #c66fff 44.21%, #ff8578 83.64%);
            }

            .link {
                background: linear-gradient(
                    90deg,
                    rgba(0, 203, 236, 0.08) 3.11%,
                    rgba(198, 111, 255, 0.08) 44.21%,
                    rgba(255, 133, 120, 0.08) 83.64%
                );
            }

            --icon-color: var(--primary);
        }

        h4 {
            margin-bottom: 0.25rem;
            color: var(--text-muted);
        }

        a {
            display: flex;
            align-items: center;
            gap: 1rem;
            :last-child {
                margin-left: auto;
            }
            .icon {
                flex-shrink: 0;
            }

            padding: 1rem;
            &:not(:hover) {
                text-decoration: none;
                color: var(--text-muted);
            }
        }
    }
</style>
