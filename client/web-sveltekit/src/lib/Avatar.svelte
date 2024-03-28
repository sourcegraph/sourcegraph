<script lang="ts">
    import type { Avatar_User, Avatar_Team, Avatar_Person } from './Avatar.gql'

    type Avatar = Avatar_User | Avatar_Team | Avatar_Person

    export let avatar: Avatar

    function getInitials(name: string): string {
        const names = name.split(' ')
        const initials = names.map(name => name.charAt(0).toLowerCase())
        if (initials.length > 1) {
            return `${initials[0]}${initials[initials.length - 1].toUpperCase()}`
        }
        return initials[0]
    }

    function getName(avatar: Avatar): string {
        switch (avatar.__typename) {
            case 'User':
                return avatar.displayName || avatar.username || ''
            case 'Person':
                return avatar.displayName || avatar.name || ''
            case 'Team':
                return avatar.displayName || ''
            default:
                return ''
        }
    }

    $: name = getName(avatar)
    $: avatarURL = avatar.avatarURL
</script>

{#if avatarURL}
    <img src={avatarURL} role="presentation" aria-hidden="true" alt="Avatar of {name}" />
{:else}
    <div>
        <span>{getInitials(name)}</span>
    </div>
{/if}

<style lang="scss">
    img,
    div {
        isolation: isolate;
        display: inline-flex;
        border-radius: 50%;
        text-transform: capitalize;
        color: var(--color-bg-1);
        align-items: center;
        justify-content: center;
        min-width: 1rem;
        min-height: 1rem;
        position: relative;
        background: linear-gradient(to bottom, var(--logo-purple), var(--logo-orange));
        width: var(--avatar-size, var(--icon-inline-size));
        height: var(--avatar-size, var(--icon-inline-size));
    }

    div::after {
        content: '';
        position: absolute;
        top: 0;
        right: 0;
        bottom: 0;
        left: 0;
        border-radius: 50%;
        background: linear-gradient(to right, var(--logo-purple), var(--logo-blue));
        mask-image: linear-gradient(to bottom, #000000, transparent);
    }

    span {
        z-index: 1;
        color: var(--white);
    }
</style>
