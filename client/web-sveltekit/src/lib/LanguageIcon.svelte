<script lang="ts">
    import { mdiFileCodeOutline } from '@mdi/js'

    import { getFileIconInfo } from '$lib/wildcard'

    import Icon from './Icon.svelte'

    export let language: string

    $: icon = getFileIconInfo('', language)?.svg
    $: svgPath = icon?.path ?? mdiFileCodeOutline
    $: svgColor = icon?.color
</script>

<Icon {svgPath} --color={svgColor} {...$$restProps} />
