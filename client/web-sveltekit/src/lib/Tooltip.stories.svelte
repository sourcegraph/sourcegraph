<script lang="ts" context="module">
    import { Story } from '@storybook/addon-svelte-csf'
    import Tooltip, { type Placement } from '$lib/Tooltip.svelte'

    export const meta = {
        component: Tooltip,
    }
</script>

<script lang="ts">
    const placements: Placement[] = ['top', 'left', 'bottom', 'right']
    let count = 0
</script>

<Story name="Default">
    <h2>Static Tooltip</h2>
    <p>
        <Tooltip tooltip="Some static tooltip text">
            <span>Hover over me</span>
        </Tooltip>
    </p>

    <h2>Dynamic Tooltip</h2>
    <p>
        Move the mouse over
        <Tooltip tooltip="This text was moused over {count} times" placement="top">
            <strong on:mousemove={() => count++}>this text</strong>
        </Tooltip>
        to update the counter in the tooltip.
    </p>

    <h2>Tooltip placement</h2>
    <table>
        {#each placements as placement}
            <tr>
                <th>{placement}</th>
                <td><Tooltip tooltip="Tooltip" {placement} alwaysVisible><span>Trigger</span></Tooltip></td>
            </tr>
        {/each}
    </table>
</Story>

<style lang="scss">
    td,
    th {
        padding: 2rem;
    }
</style>
