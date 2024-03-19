<script lang="ts" context="module">
    import { AlertType } from '$lib/graphql-types'
    import { formatDistanceStrict, isAfter } from 'date-fns'

    type PossibleAlertVariation = 'info' | 'warning' | 'danger'

    function getAlertVariantForType(type: AlertType): PossibleAlertVariation {
        switch (type) {
            case AlertType.INFO: {
                return 'info'
            }
            case AlertType.WARNING: {
                return 'warning'
            }
            case AlertType.ERROR: {
                return 'danger'
            }
            default: {
                return 'warning'
            }
        }
    }

    function isProductLicenseExpired(expiresAt: string | number | Date): boolean {
        return !isAfter(typeof expiresAt === 'string' ? parseISO(expiresAt) : expiresAt, Date.now())
    }

    function formatRelativeExpirationDate(date: string | number | Date): string {
        return `${formatDistanceStrict(typeof date === 'string' ? parseISO(date) : date, Date.now())} ${
            isProductLicenseExpired(date) ? 'ago' : 'remaining'
        }`
    }
</script>

<script lang="ts">
    import { settings } from '$lib/stores'

    import { Markdown } from '$lib/wildcard'
    import DismissibleAlert from './DismissibleAlert.svelte'

    import type { GlobalNotifications } from './GlobalNotifications.gql'
    import { differenceInDays, parseISO } from 'date-fns'

    export let globalAlerts: GlobalNotifications

    $: settingsMotd = $settings?.motd
    $: notices = $settings?.notices?.filter(notice => notice.location === 'top')

    $: noLicenseWarningUserCount = globalAlerts.productSubscription.noLicenseWarningUserCount
    $: expiresAt = parseISO(globalAlerts.productSubscription.license.expiresAt)
    $: daysLeft = Math.floor(differenceInDays(expiresAt, Date.now()))
</script>

<div class="root" aria-label="Global site notifications">
    {#if globalAlerts.needsRepositoryConfiguration}
        <DismissibleAlert variant="success" partialStorageKey="needsRepositoryConfiguration">
            <a href="/setup/remote-repositories"> Go to setup wizard </a>
            &nbsp;to add remote repositories from GitHub, GitLab, etc.
        </DismissibleAlert>
    {/if}

    {#if globalAlerts.freeUsersExceeded}
        <DismissibleAlert variant="info" partialStorageKey={null}>
            This Sourcegraph instance has reached&nbsp;
            {noLicenseWarningUserCount === null ? 'the limit for' : noLicenseWarningUserCount} free users, and an admin must&nbsp;
            <a href="https://sourcegraph.com/contact/sales">
                contact Sourcegraph to start a free trial or purchase a license
            </a>&nbsp; to add more
        </DismissibleAlert>
    {/if}

    {#each globalAlerts.alerts as alert (alert.message)}
        <DismissibleAlert variant={getAlertVariantForType(alert.type)} partialStorageKey={alert.isDismissibleWithKey}>
            <Markdown content={alert.message} />
        </DismissibleAlert>
    {/each}

    {#if settingsMotd && Array.isArray(settingsMotd)}
        {#each settingsMotd as motd}
            <DismissibleAlert variant="info" partialStorageKey={`motd.${motd}`}>
                <Markdown content={motd} />
            </DismissibleAlert>
        {/each}
    {/if}

    {#if notices && Array.isArray(notices)}
        {#each notices as notice (notice.message)}
            <DismissibleAlert variant="info" partialStorageKey={`notices.${notice.message}`}>
                <Markdown content={notice.message} />
            </DismissibleAlert>
        {/each}
    {/if}

    {#if globalAlerts.productSubscription.license && daysLeft <= 7}
        <DismissibleAlert variant="warning" partialStorageKey={`licenseExpiring.${daysLeft}`}>
            Your Sourcegraph license&nbsp;
            {isProductLicenseExpired(expiresAt)
                ? 'expired ' + formatRelativeExpirationDate(expiresAt) // 'Expired two months ago'
                : 'will expire in ' + formatDistanceStrict(expiresAt, Date.now())}.&nbsp;
            <a href="/site-admin/license">Renew now</a>
            &nbsp;or&nbsp;
            <a href="https://sourcegraph.com/contact">contact Sourcegraph</a>
        </DismissibleAlert>
    {/if}

    {#if process.env.SOURCEGRAPH_API_URL}
        <DismissibleAlert variant="danger" partialStorageKey="dev-web-server-alert">
            <strong>Warning!</strong>&nbsp;This build uses data from the proxied API:
            <a class="proxy-link" target="__blank" href={process.env.SOURCEGRAPH_API_URL}>
                {process.env.SOURCEGRAPH_API_URL}
            </a>
        </DismissibleAlert>
    {/if}
</div>

<style lang="scss">
    .root {
        width: 100%;
        border-top: 1px solid var(--border-color-2);

        &:empty {
            display: none;
        }
    }

    .proxy-link {
        color: var(--body-color);
    }
</style>
